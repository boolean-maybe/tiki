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

// parsedTiki is the result of parsing a markdown file: the Tiki itself, the
// verbatim decoded frontmatter map (retained for raw-type checks on load),
// and the set of workflow-field keys whose raw value failed coercion. Stale
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

	fields, stale, err := buildFieldsFromFrontmatter(fmMap, path)
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
// the Tiki Fields map. Each key is dispatched through workflow.Field() —
// the workflow catalog is the single source of truth for what is workflow-
// declared and how to coerce it. Workflow-declared keys whose values fail
// coercion are demoted to verbatim with a stale marker. Keys not declared
// in workflow.yaml round-trip verbatim as unknown fields.
func buildFieldsFromFrontmatter(fmMap map[string]interface{}, path string) (map[string]interface{}, map[string]struct{}, error) {
	if len(fmMap) == 0 {
		return nil, nil, nil
	}

	out := map[string]interface{}{}
	var stale map[string]struct{}

	registryChecked := false
	requireRegistry := func() error {
		if registryChecked {
			return nil
		}
		registryChecked = true
		return config.RequireWorkflowFieldsLoaded()
	}

	for key, raw := range fmMap {
		// identity/audit keys live on the Tiki struct, not in Fields
		if key == "id" || key == "title" || tiki.IsIdentityField(key) {
			continue
		}
		if err := requireRegistry(); err != nil {
			return nil, nil, fmt.Errorf("loading frontmatter for %s: %w", path, err)
		}
		fd, registered := workflow.Field(key)
		if registered && !fd.Custom {
			// system field (e.g. filepath) — drop stale value from disk
			slog.Debug("dropping synthetic system field from frontmatter", "field", key, "file", path)
			continue
		}
		if !registered {
			slog.Debug("preserving unknown field in frontmatter", "field", key, "file", path)
			out[key] = raw
			continue
		}
		val, err := coerceCustomValue(fd, raw)
		if err != nil {
			slog.Warn("demoting stale workflow field value to unknown",
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
// frontmatter (without the surrounding --- delimiters). Emission order:
// id first, title second, then workflow-declared fields in workflow.yaml
// declaration order, then remaining (unknown) keys alphabetically. Per-key
// encoding is driven by the workflow field type — no key is special-cased
// by name.
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

	// Workflow-declared fields first, in workflow.yaml declaration order,
	// dispatched through workflow.Field(name).Type.
	emitted := map[string]struct{}{}
	for _, fd := range workflow.WorkflowFields() {
		v, present := t.Fields[fd.Name]
		if !present {
			continue
		}
		encoded, err := encodeFieldByType(fd, v)
		if err != nil {
			return nil, fmt.Errorf("marshaling field %q: %w", fd.Name, err)
		}
		buf.Write(encoded)
		emitted[fd.Name] = struct{}{}
	}

	// Remaining (unknown / unregistered) keys in sorted order.
	// In-memory-only fields never reach disk: createdBy carries runtime audit
	// metadata, comments carries in-memory comment state.
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
		out, err := yaml.Marshal(map[string]interface{}{k: v})
		if err != nil {
			return nil, fmt.Errorf("marshaling field %q: %w", k, err)
		}
		buf.Write(out)
	}

	return []byte(buf.String()), nil
}

// encodeFieldByType emits one "key: value" YAML line for a workflow-declared
// field, dispatching on the field's declared type. Registered list fields
// get dedupe/sort normalization; values whose shape does not satisfy the
// declared type pass through verbatim (mirroring the load-side demotion to
// stale unknown).
func encodeFieldByType(fd workflow.FieldDef, value interface{}) ([]byte, error) {
	key := fd.Name
	switch fd.Type {
	case workflow.TypeListString:
		ss, err := coerceStringList(value)
		if err != nil {
			return yaml.Marshal(map[string]interface{}{key: value})
		}
		v := collectionutil.NormalizeStringSet(ss)
		sort.Strings(v)
		return yaml.Marshal(map[string]interface{}{key: v})

	case workflow.TypeListRef:
		ss, err := coerceStringList(value)
		if err != nil {
			return yaml.Marshal(map[string]interface{}{key: value})
		}
		v := collectionutil.NormalizeRefSet(ss)
		sort.Strings(v)
		return yaml.Marshal(map[string]interface{}{key: v})

	case workflow.TypeDate:
		tv, ok := coerceTimeForYAML(value)
		if !ok {
			return yaml.Marshal(map[string]interface{}{key: value})
		}
		// DueValue.MarshalYAML emits date-only (YYYY-MM-DD), which is the
		// correct shape for TypeDate fields.
		return yaml.Marshal(map[string]interface{}{key: taskpkg.DueValue{Time: tv}})

	case workflow.TypeTimestamp:
		tv, ok := coerceTimeForYAML(value)
		if !ok {
			return yaml.Marshal(map[string]interface{}{key: value})
		}
		// Emit a full RFC3339 timestamp so the time component round-trips.
		// Using DueValue here would silently truncate to YYYY-MM-DD.
		return yaml.Marshal(map[string]interface{}{key: tv.UTC().Format(time.RFC3339)})

	case workflow.TypeInt:
		n, ok := coerceIntForYAML(value)
		if !ok {
			return yaml.Marshal(map[string]interface{}{key: value})
		}
		return yaml.Marshal(map[string]interface{}{key: n})
	}

	// All other types (string, bool, enum, recurrence, ref, id) pass through
	// yaml.Marshal verbatim. Validators upstream guarantee shape correctness.
	return yaml.Marshal(map[string]interface{}{key: value})
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
