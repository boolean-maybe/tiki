package tikistore

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/tiki"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"

	"gopkg.in/yaml.v3"
)

// tiki_bridge.go is Phase 3's central seam. Persistence parses every file
// into a *tiki.Tiki with a generic Fields map (source of truth), then the
// adapters below project to/from *taskpkg.Task for the rest of the store
// and the downstream task-shaped API surface. Phase 5 will flip the store's
// internal map to *tiki.Tiki and retire this bridge; Phase 3 keeps the map
// task-shaped so ruki/service/UI readers of Task.IsWorkflow /
// Task.WorkflowFrontmatter keep compiling unchanged.

// workflowKeys lists the frontmatter keys that carry workflow semantics in
// canonical emission order. Kept adjacent to the Tiki bridge because every
// workflow/field seam in this package reads it.
var workflowKeys = []string{
	"status",
	"type",
	"tags",
	"dependsOn",
	"due",
	"recurrence",
	"assignee",
	"priority",
	"points",
}

// parsedTiki is the result of parsing a markdown file: the Tiki itself plus
// the verbatim decoded frontmatter map (retained only so the task adapter
// can compute IsWorkflow the same way document.IsWorkflowFrontmatter would,
// without re-parsing) plus the set of custom-field keys whose raw value
// failed registry coercion on load. Stale keys land in Fields verbatim so
// round-trips preserve them, but the Task adapter routes them to
// UnknownFields (not CustomFields) so downstream consumers see the same
// split the pre-Phase-3 load produced.
type parsedTiki struct {
	t     *tiki.Tiki
	raw   map[string]interface{}
	stale map[string]struct{}
}

// loadTikiFromBytes parses markdown bytes into a Tiki. The caller owns
// file-level concerns (stat, author lookup, path resolution).
func loadTikiFromBytes(path string, content []byte) (*parsedTiki, error) {
	frontmatter, body, err := store.ParseFrontmatter(string(content))
	if err != nil {
		return nil, newLoadError(LoadReasonParseError, fmt.Errorf("parsing frontmatter: %w", err))
	}

	var fmMap map[string]interface{}
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &fmMap); err != nil {
			return nil, newLoadError(LoadReasonParseError, fmt.Errorf("parsing yaml: %w", err))
		}
	}

	rawID, hasID := document.FrontmatterID(fmMap)
	if !hasID || rawID == "" {
		return nil, newLoadError(LoadReasonMissingID,
			fmt.Errorf("missing frontmatter id in %s: run `tiki repair ids --fix` to backfill", path))
	}
	id := document.NormalizeID(rawID)
	if !document.IsValidID(id) {
		return nil, newLoadError(LoadReasonInvalidID,
			fmt.Errorf("%s: invalid document id %q: expected %d uppercase alphanumeric characters",
				path, id, document.IDLength))
	}

	title, _ := fmMap["title"].(string)

	// Decode schema-known workflow keys through the existing typed wrappers
	// so the on-disk values land in canonical Go types (PriorityValue
	// clamping, tag list normalization, etc.). We do this by re-unmarshaling
	// into taskFrontmatter alongside the raw-map parse above.
	var fm taskFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return nil, newLoadError(LoadReasonParseError, fmt.Errorf("parsing yaml into schema: %w", err))
	}

	fields, stale, err := buildFieldsFromFrontmatter(fmMap, &fm, path)
	if err != nil {
		return nil, err
	}

	t := tiki.New()
	t.ID = id
	t.Title = title
	t.Body = strings.TrimSpace(body)
	t.Fields = fields
	t.Path = path

	return &parsedTiki{t: t, raw: fmMap, stale: stale}, nil
}

// buildFieldsFromFrontmatter copies every non-identity frontmatter key into
// the Tiki Fields map. Schema-known workflow keys use values decoded via
// taskFrontmatter (already clamped/parsed). Registered custom fields run
// through the workflow coercer. Unknown keys round-trip verbatim.
//
// Returns the built Fields map plus the set of keys whose registered custom
// field failed coercion (stale enum values, type mismatches). The Task
// adapter routes stale keys to UnknownFields so the pre-Phase-3 split is
// preserved downstream.
func buildFieldsFromFrontmatter(fmMap map[string]interface{}, fm *taskFrontmatter, path string) (map[string]interface{}, map[string]struct{}, error) {
	if len(fmMap) == 0 {
		return nil, nil, nil
	}

	out := map[string]interface{}{}
	var stale map[string]struct{}

	// Workflow-schema keys: copy decoded typed values, gated on presence in
	// fmMap so we never synthesize keys the file didn't carry.
	for _, k := range workflowKeys {
		if _, present := fmMap[k]; !present {
			continue
		}
		switch k {
		case "status":
			if fm.Status != "" {
				out[k] = string(taskpkg.MapStatus(fm.Status))
			} else {
				out[k] = ""
			}
		case "type":
			out[k] = fm.Type
		case "tags":
			out[k] = fm.Tags.ToStringSlice()
		case "dependsOn":
			out[k] = fm.DependsOn.ToStringSlice()
		case "due":
			out[k] = fm.Due.ToTime()
		case "recurrence":
			out[k] = string(fm.Recurrence.ToRecurrence())
		case "assignee":
			out[k] = fm.Assignee
		case "priority":
			out[k] = int(fm.Priority)
		case "points":
			out[k] = fm.Points
		}
	}

	// Custom and unknown fields: defer registry lookup until we hit a
	// non-builtin key so the load path stays lightweight for pure workflow
	// files.
	registryChecked := false
	requireRegistry := func() error {
		if registryChecked {
			return nil
		}
		registryChecked = true
		return config.RequireWorkflowRegistriesLoaded()
	}

	for key, raw := range fmMap {
		if key == "id" || key == "title" {
			continue
		}
		if builtInFrontmatterKeys[key] {
			continue
		}
		if err := requireRegistry(); err != nil {
			return nil, nil, fmt.Errorf("extractCustomFields for %s: %w", path, err)
		}
		fd, registered := workflow.Field(key)
		if registered && !fd.Custom {
			// synthetic built-in (e.g. filepath) — drop stale value from disk
			slog.Debug("dropping synthetic built-in field from frontmatter", "field", key, "file", path)
			continue
		}
		if !registered {
			slog.Debug("preserving unknown field in frontmatter", "field", key, "file", path)
			out[key] = raw
			continue
		}
		val, err := coerceCustomValue(fd, raw)
		if err != nil {
			slog.Warn("demoting stale custom field value to unknown",
				"field", key, "file", path, "error", err)
			out[key] = raw
			if stale == nil {
				stale = map[string]struct{}{}
			}
			stale[key] = struct{}{}
			continue
		}
		out[key] = val
	}

	if len(out) == 0 {
		return nil, stale, nil
	}
	return out, stale, nil
}

// marshalTikiFrontmatter serializes t's id, title, and Fields into YAML
// frontmatter (without the surrounding --- delimiters). Emission is
// deterministic: id first, title second, then every Fields key in a
// canonical order — workflow-schema keys first in their legacy order, then
// remaining keys alphabetically — so file contents stay stable across
// runs.
//
// Schema-known workflow keys use the typed value wrappers from the task
// package (PriorityValue, DueValue, TagsValue, DependsOnValue,
// RecurrenceValue) so the on-disk format matches the pre-Phase-3 writer
// byte-for-byte. Registered custom fields run through registry-aware list
// coercion. Unknown keys pass through yaml.Marshal verbatim.
func marshalTikiFrontmatter(t *tiki.Tiki) ([]byte, error) {
	if t == nil {
		return yaml.Marshal(map[string]interface{}{})
	}

	var buf strings.Builder

	if t.ID != "" {
		out, err := yaml.Marshal(map[string]interface{}{"id": t.ID})
		if err != nil {
			return nil, err
		}
		buf.Write(out)
	}
	if t.Title != "" {
		out, err := yaml.Marshal(map[string]interface{}{"title": t.Title})
		if err != nil {
			return nil, err
		}
		buf.Write(out)
	}

	// Emit workflow-schema keys first, in the canonical order, so sparse
	// workflow files keep their familiar key ordering.
	emitted := map[string]struct{}{}
	for _, k := range workflowKeys {
		v, present := t.Fields[k]
		if !present {
			continue
		}
		encoded, err := encodeWorkflowField(k, v)
		if err != nil {
			return nil, fmt.Errorf("marshaling field %q: %w", k, err)
		}
		buf.Write(encoded)
		emitted[k] = struct{}{}
	}

	// Then emit remaining (custom + unknown) keys in sorted order.
	// "createdBy" and "comments" are in-memory-only fields that must not
	// be written to disk: they carry runtime audit metadata and in-memory
	// comment state respectively.
	inMemoryOnlyFields := map[string]struct{}{
		"createdBy": {},
		"comments":  {},
	}
	remaining := make([]string, 0, len(t.Fields))
	for k := range t.Fields {
		if _, done := emitted[k]; done {
			continue
		}
		if _, skip := inMemoryOnlyFields[k]; skip {
			continue
		}
		remaining = append(remaining, k)
	}
	sort.Strings(remaining)

	for _, k := range remaining {
		v := t.Fields[k]
		encoded, err := encodeExtraField(k, v)
		if err != nil {
			return nil, fmt.Errorf("marshaling field %q: %w", k, err)
		}
		buf.Write(encoded)
	}

	return []byte(buf.String()), nil
}

// encodeWorkflowField emits a single "key: value" YAML line for a
// schema-known workflow key, routing through the typed value wrappers so
// the disk format matches the legacy writer.
func encodeWorkflowField(key string, value interface{}) ([]byte, error) {
	switch key {
	case "status":
		s, _ := value.(string)
		return yaml.Marshal(map[string]interface{}{key: taskpkg.StatusToString(taskpkg.Status(s))})
	case "type":
		s, _ := value.(string)
		return yaml.Marshal(map[string]interface{}{key: s})
	case "tags":
		tags := coerceToStringSlice(value)
		tags = collectionutil.NormalizeStringSet(tags)
		sort.Strings(tags)
		return yaml.Marshal(map[string]interface{}{key: taskpkg.TagsValue(tags)})
	case "dependsOn":
		deps := coerceToStringSlice(value)
		deps = collectionutil.NormalizeRefSet(deps)
		sort.Strings(deps)
		return yaml.Marshal(map[string]interface{}{key: taskpkg.DependsOnValue(deps)})
	case "due":
		tv, _ := coerceTimeForYAML(value)
		return yaml.Marshal(map[string]interface{}{key: taskpkg.DueValue{Time: tv}})
	case "recurrence":
		s, _ := value.(string)
		return yaml.Marshal(map[string]interface{}{key: taskpkg.RecurrenceValue{Value: taskpkg.Recurrence(s)}})
	case "assignee":
		s, _ := value.(string)
		return yaml.Marshal(map[string]interface{}{key: s})
	case "priority":
		n, _ := coerceIntForYAML(value)
		return yaml.Marshal(map[string]interface{}{key: taskpkg.PriorityValue(n)})
	case "points":
		n, _ := coerceIntForYAML(value)
		return yaml.Marshal(map[string]interface{}{key: n})
	}
	return nil, fmt.Errorf("unknown workflow field %q", key)
}

// encodeExtraField emits a custom or unknown field. Registered custom
// fields get list-type normalization; values whose shape does not satisfy
// the registry (stale entries preserved for repair) pass through verbatim
// — mirroring the load-path demotion in buildFieldsFromFrontmatter, so a
// stale value round-trips unchanged. Unregistered keys also pass through
// verbatim.
func encodeExtraField(key string, value interface{}) ([]byte, error) {
	fd, ok := workflow.Field(key)
	if !ok || !fd.Custom {
		return yaml.Marshal(map[string]interface{}{key: value})
	}
	v := value
	switch fd.Type {
	case workflow.TypeListString:
		ss, err := coerceStringList(v)
		if err != nil {
			// Stale list value for a registered list field. The load path
			// demoted this to UnknownFields; emit verbatim so the round-trip
			// preserves exactly what's on disk for the user to repair.
			return yaml.Marshal(map[string]interface{}{key: value})
		}
		v = collectionutil.NormalizeStringSet(ss)
	case workflow.TypeListRef:
		ss, err := coerceStringList(v)
		if err != nil {
			return yaml.Marshal(map[string]interface{}{key: value})
		}
		v = collectionutil.NormalizeRefSet(ss)
	}
	return yaml.Marshal(map[string]interface{}{key: v})
}

// coerceIntForYAML accepts the numeric shapes yaml.v3 emits, plus plain int.
func coerceIntForYAML(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == float64(int64(n)) {
			return int(n), true
		}
		return 0, false
	default:
		return 0, false
	}
}

// coerceTimeForYAML accepts time.Time directly or a YYYY-MM-DD string.
func coerceTimeForYAML(v interface{}) (time.Time, bool) {
	switch val := v.(type) {
	case time.Time:
		return val, true
	case string:
		if t, err := time.Parse("2006-01-02", val); err == nil {
			return t, true
		}
		return time.Time{}, false
	default:
		return time.Time{}, false
	}
}

// coerceToStringSlice handles []string, []interface{} of strings, or nil.
func coerceToStringSlice(v interface{}) []string {
	switch s := v.(type) {
	case []string:
		out := make([]string, len(s))
		copy(out, s)
		return out
	case []interface{}:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

// tikiToTask projects a Tiki to a Task for the store's internal map and
// downstream task-shaped callers. IsWorkflow is derived from field-map
// presence (any workflow-schema key present → workflow); WorkflowFrontmatter
// snapshots the verbatim workflow subset. CustomFields and UnknownFields
// are split out from Fields by consulting the workflow registry.
//
// Phase 5 will retire this adapter once the rest of the codebase reads from
// Tiki.Fields directly.
func tikiToTask(pt *parsedTiki) *taskpkg.Task {
	if pt == nil || pt.t == nil {
		return nil
	}
	t := pt.t

	task := &taskpkg.Task{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Body,
		LoadedMtime: t.LoadedMtime,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		FilePath:    t.Path,
	}

	// Carry typed workflow fields out of Fields for downstream Task readers.
	if s, ok := t.Fields["status"].(string); ok {
		task.Status = taskpkg.Status(s)
	}
	if s, ok := t.Fields["type"].(string); ok {
		task.Type = taskpkg.Type(s)
	}
	if v, ok := t.Fields["tags"]; ok {
		task.Tags = coerceToStringSlice(v)
	}
	if v, ok := t.Fields["dependsOn"]; ok {
		task.DependsOn = coerceToStringSlice(v)
	}
	if v, ok := t.Fields["due"]; ok {
		if d, ok := v.(time.Time); ok {
			task.Due = d
		}
	}
	if s, ok := t.Fields["recurrence"].(string); ok {
		task.Recurrence = taskpkg.Recurrence(s)
	}
	if s, ok := t.Fields["assignee"].(string); ok {
		task.Assignee = s
	}
	if n, ok := coerceIntForYAML(t.Fields["priority"]); ok {
		task.Priority = n
	}
	if n, ok := coerceIntForYAML(t.Fields["points"]); ok {
		task.Points = n
	}

	// IsWorkflow + WorkflowFrontmatter synthesis.
	//
	// The presence test here intentionally consults the VERBATIM decoded
	// frontmatter (pt.raw) rather than t.Fields. Reason: buildFieldsFromFrontmatter
	// may drop a synthetic registry-owned field, which would flip a file's
	// workflow classification away from what's on disk. The raw map is the
	// same input document.IsWorkflowFrontmatter would see.
	task.IsWorkflow = document.IsWorkflowFrontmatter(pt.raw)
	if task.IsWorkflow {
		task.WorkflowFrontmatter = extractWorkflowSubsetFromFields(pt.raw)
		// Pre-Phase-3 load convention: workflow tasks always have non-nil
		// Tags and DependsOn slices on the Task (empty for absent fields),
		// so downstream reflect.DeepEqual tests and range loops don't have
		// to nil-guard. The Tiki Fields map keeps its absence semantics —
		// this normalization happens only at the Task adapter boundary.
		if task.Tags == nil {
			task.Tags = []string{}
		}
		if task.DependsOn == nil {
			task.DependsOn = []string{}
		}
	}

	task.CustomFields, task.UnknownFields = splitExtraFields(t.Fields, pt.stale)

	return task
}

// taskToTiki projects a Task back to a Tiki for the save path. This is the
// inverse of tikiToTask:
//
//   - loaded workflow tasks (non-nil WorkflowFrontmatter) emit only the
//     preserved presence set — a sparse file stays sparse across edits;
//   - in-memory workflow tasks (nil WorkflowFrontmatter, IsWorkflow=true)
//     emit every schema-known workflow key so a task created via
//     CreateTask(&Task{IsWorkflow:true}) without typed defaults still
//     lands on disk with at least one workflow marker, keeping the file
//     classified as workflow across a reload;
//   - plain tasks emit no workflow keys, regardless of residual typed
//     values (e.g. registry-default Priority that the store may have
//     populated for its own use).
//
// CustomFields and UnknownFields are merged back into Fields verbatim;
// the emitter in encodeExtraField handles validation-tolerant round-trip
// for stale custom list values.
func taskToTiki(task *taskpkg.Task) *tiki.Tiki {
	if task == nil {
		return nil
	}
	out := tiki.New()
	out.ID = task.ID
	out.Title = task.Title
	out.Body = task.Description
	out.Path = task.FilePath
	out.LoadedMtime = task.LoadedMtime
	out.CreatedAt = task.CreatedAt
	out.UpdatedAt = task.UpdatedAt

	if task.IsWorkflow {
		// Determine which workflow keys end up in Fields. The preserved
		// presence set drives sparse emission for loaded docs; newly
		// created workflow tasks (nil WorkflowFrontmatter) fall back to
		// value-derived presence so creation defaults land on disk.
		var emit map[string]struct{}
		if task.WorkflowFrontmatter != nil {
			emit = make(map[string]struct{}, len(task.WorkflowFrontmatter))
			for k := range task.WorkflowFrontmatter {
				emit[k] = struct{}{}
			}
		} else {
			emit = derivedWorkflowPresence(task)
		}
		setWorkflowField := func(k string, v interface{}) {
			if _, ok := emit[k]; !ok {
				return
			}
			out.Set(k, v)
		}
		setWorkflowField("status", taskpkg.StatusToString(task.Status))
		setWorkflowField("type", string(task.Type))
		if len(task.Tags) > 0 || fieldInSet(emit, "tags") {
			tags := append([]string(nil), task.Tags...)
			setWorkflowField("tags", tags)
		}
		if len(task.DependsOn) > 0 || fieldInSet(emit, "dependsOn") {
			deps := append([]string(nil), task.DependsOn...)
			setWorkflowField("dependsOn", deps)
		}
		setWorkflowField("due", task.Due)
		setWorkflowField("recurrence", string(task.Recurrence))
		setWorkflowField("assignee", task.Assignee)
		setWorkflowField("priority", task.Priority)
		setWorkflowField("points", task.Points)
	}

	for k, v := range task.CustomFields {
		out.Set(k, v)
	}
	for k, v := range task.UnknownFields {
		out.Set(k, v)
	}

	return out
}

// derivedWorkflowPresence computes a presence set for an in-memory workflow
// task whose WorkflowFrontmatter is nil (e.g. NewTaskTemplate /
// CreateTask(IsWorkflow=true)). Matches the pre-Phase-3 yaml-struct behavior
// of taskFrontmatter: `type` and `status` are always emitted (no omitempty)
// so at least one workflow marker lands on disk and classification survives
// a round-trip even for an otherwise-empty workflow task; every other key
// is emitted only when its typed value is non-zero (yaml `,omitempty`
// equivalent).
//
// Loaded docs bypass this entirely via the preserved WorkflowFrontmatter
// snapshot, so their on-disk shape is not "filled in" by an unrelated edit.
func derivedWorkflowPresence(task *taskpkg.Task) map[string]struct{} {
	emit := map[string]struct{}{
		// always-present sentinels for classification survival
		"status": {},
		"type":   {},
	}
	if len(task.Tags) > 0 {
		emit["tags"] = struct{}{}
	}
	if len(task.DependsOn) > 0 {
		emit["dependsOn"] = struct{}{}
	}
	if !task.Due.IsZero() {
		emit["due"] = struct{}{}
	}
	if string(task.Recurrence) != "" {
		emit["recurrence"] = struct{}{}
	}
	if task.Assignee != "" {
		emit["assignee"] = struct{}{}
	}
	if task.Priority != 0 {
		emit["priority"] = struct{}{}
	}
	if task.Points != 0 {
		emit["points"] = struct{}{}
	}
	return emit
}

func fieldInSet(s map[string]struct{}, k string) bool {
	_, ok := s[k]
	return ok
}

// extractWorkflowSubsetFromFields returns the verbatim subset of raw that
// covers workflow-schema keys. This matches the semantics of the pre-Phase-3
// workflowSubsetFromFrontmatter helper: absent keys stay absent, so the
// returned map is a presence-preserving snapshot of the source YAML.
func extractWorkflowSubsetFromFields(raw map[string]interface{}) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]interface{}
	for _, k := range workflowKeys {
		if _, present := raw[k]; !present {
			continue
		}
		if out == nil {
			out = make(map[string]interface{})
		}
		out[k] = raw[k]
	}
	return out
}

// splitExtraFields separates Fields into custom (registry-known, Custom=true,
// coerced successfully) and unknown buckets for the Task adapter. Schema-
// known workflow keys are skipped — those come back through the typed Task
// fields. Keys in the stale set are routed to unknown (not custom) so the
// downstream split matches pre-Phase-3 behavior.
func splitExtraFields(fields map[string]interface{}, stale map[string]struct{}) (custom, unknown map[string]interface{}) {
	if len(fields) == 0 {
		return nil, nil
	}
	for k, v := range fields {
		if builtInFrontmatterKeys[k] {
			continue
		}
		if _, isStale := stale[k]; isStale {
			if unknown == nil {
				unknown = make(map[string]interface{})
			}
			unknown[k] = v
			continue
		}
		fd, ok := workflow.Field(k)
		if ok && fd.Custom {
			if custom == nil {
				custom = make(map[string]interface{})
			}
			custom[k] = v
			continue
		}
		if ok && !fd.Custom {
			// synthetic built-in (filepath etc.) — drop (already dropped on load)
			continue
		}
		if unknown == nil {
			unknown = make(map[string]interface{})
		}
		unknown[k] = v
	}
	return custom, unknown
}
