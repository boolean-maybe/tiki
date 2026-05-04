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

// tiki_bridge.go holds the load/save bridge: parsing markdown files into
// *tiki.Tiki and serializing them back. The store's internal map and all
// public APIs operate on *tiki.Tiki directly.

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

// parsedTiki is the result of parsing a markdown file: the Tiki itself, the
// verbatim decoded frontmatter map (retained for raw-type checks on load), and
// the set of custom-field keys whose raw value failed registry coercion. Stale
// keys land in Fields verbatim so round-trips preserve them.
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
