package task

import (
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/document"
)

// Workflow frontmatter keys — the single source of truth shared by
// FromDocument/ToDocument and the presence classifier. Keep in sync with
// document.IsWorkflowFrontmatter: both read the same set.
const (
	fmKeyStatus     = "status"
	fmKeyType       = "type"
	fmKeyTags       = "tags"
	fmKeyDependsOn  = "dependsOn"
	fmKeyDue        = "due"
	fmKeyRecurrence = "recurrence"
	fmKeyAssignee   = "assignee"
	fmKeyPriority   = "priority"
	fmKeyPoints     = "points"
)

// FromDocument projects a Document into a Task using the Phase 4 mapping
// rules. Workflow fields are read from the document's Frontmatter map with
// presence-awareness: missing keys stay zero-valued on the Task rather than
// being filled with registry defaults, so a plain doc never masquerades as
// a workflow-capable Task.
//
// Mapping rules (plan Phase 4):
//   - id                    → Task.ID
//   - title                 → Task.Title
//   - body                  → Task.Description (trimmed whitespace)
//   - status/type/...       → typed Task fields only when present in
//     Frontmatter
//   - CustomFields          → carried through as-is
//   - UnknownFields         → carried through as-is
//   - Path/LoadedMtime/...  → FilePath / LoadedMtime / CreatedAt / UpdatedAt
//
// FromDocument does NOT apply creation defaults. It also does NOT re-run
// workflow-field coercion (enums, tag normalization, etc.); callers that
// need coercion should load through the store boundary, which owns the
// workflow registry dependency.
//
// IsWorkflow is set from document.IsWorkflowFrontmatter so a caller that
// round-trips a plain document back to a Task does not accidentally promote
// it.
func FromDocument(d *document.Document) *Task {
	if d == nil {
		return nil
	}
	t := &Task{
		ID:                  d.ID,
		Title:               d.Title,
		Description:         strings.TrimSpace(d.Body),
		CustomFields:        cloneMap(d.CustomFields),
		UnknownFields:       cloneMap(d.UnknownFields),
		LoadedMtime:         d.LoadedMtime,
		CreatedAt:           d.CreatedAt,
		UpdatedAt:           d.UpdatedAt,
		FilePath:            d.Path,
		IsWorkflow:          document.IsWorkflowFrontmatter(d.Frontmatter),
		WorkflowFrontmatter: extractWorkflowSubset(d.Frontmatter),
	}
	applyWorkflowFrontmatter(t, d.Frontmatter)
	return t
}

// ToDocument projects a Task into a Document. Presence-aware contract:
//
//   - Plain tasks (IsWorkflow=false) emit a Document with nil Frontmatter,
//     regardless of any residual non-zero typed field values the store may
//     have populated for its own use. This closes the hole where a load
//     path that applied registry defaults to Priority/Points would cause a
//     round-trip through ToDocument to promote the doc by synthesizing
//     workflow frontmatter.
//   - Workflow tasks loaded from disk emit the verbatim workflow subset of
//     their source frontmatter (Task.WorkflowFrontmatter), so a key that
//     was absent from the file stays absent in the projected Document. A
//     body-only edit via UpdateDocument therefore cannot acquire synthetic
//     defaults for fields the user never set.
//   - Workflow tasks built in-memory (NewTaskTemplate/CreateTask) have no
//     source frontmatter to preserve; for those ToDocument falls back to
//     value-derived synthesis. That matches the plan's rule that workflow
//     creation paths are the only places defaults apply.
//
// Frontmatter ordering is not preserved: callers that need deterministic
// serialization (e.g. the persistence layer) should build the YAML from a
// struct with explicit yaml tags rather than reading back from Frontmatter.
// This adapter is for in-memory interchange, not on-disk representation.
func ToDocument(t *Task) *document.Document {
	if t == nil {
		return nil
	}
	return &document.Document{
		ID:            t.ID,
		Title:         t.Title,
		Body:          t.Description,
		Path:          t.FilePath,
		Frontmatter:   projectFrontmatter(t),
		CustomFields:  cloneMap(t.CustomFields),
		UnknownFields: cloneMap(t.UnknownFields),
		LoadedMtime:   t.LoadedMtime,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}
}

// projectFrontmatter chooses the right emission strategy for ToDocument.
// See the godoc on ToDocument for the three cases.
func projectFrontmatter(t *Task) map[string]interface{} {
	if t == nil || !t.IsWorkflow {
		return nil
	}
	if len(t.WorkflowFrontmatter) > 0 {
		// Preserved presence set wins — no synthesis, no drift.
		return cloneMap(t.WorkflowFrontmatter)
	}
	// In-memory workflow task (template/create). Emit from typed values so
	// new docs surface their creation defaults when first projected.
	return workflowFrontmatterFromTask(t)
}

// MergeTypedWorkflowDeltas GROWS newTask.WorkflowFrontmatter by the workflow
// keys whose typed value differs between newTask and oldTask AND is non-zero
// on newTask. It intentionally never removes keys from the preserved set,
// because task-shaped compatibility callers routinely build a Task from
// only the fields they care about — a zero on the incoming task means
// "caller did not set this", not "clear this field".
//
// Example: a ruki/UI caller constructs Task{ID, Title, Priority: 1} for a
// workflow doc whose file has only `status: ready`. The merge sees:
//   - status: new="" old="ready" → differ, but new is zero → no-op (grow-only)
//   - priority: new=1 old=3 (defaulted on load) → differ, new is non-zero → add
//
// Result: WorkflowFrontmatter grows to {status, priority} (status was
// inherited from oldTask before this call runs). The file writes status
// unchanged and priority=1, keeping the sparse file sparse-plus-new-edit.
//
// Callers who need to explicitly CLEAR a workflow key use the document-
// first API: remove the key from doc.Frontmatter before UpdateDocument and
// the document adapter propagates the full intended presence set verbatim,
// bypassing this helper entirely.
//
// newTask and oldTask must share the same ID. Nil-safe: oldTask==nil means
// every non-zero typed value on newTask is treated as an edit.
func MergeTypedWorkflowDeltas(newTask, oldTask *Task) {
	if newTask == nil {
		return
	}
	for _, k := range workflowKeys {
		newVal := typedWorkflowValue(newTask, k)
		// Grow-only: a zero on the incoming task is always "caller did not
		// set this". Never shrink the presence set from this path.
		if isZeroWorkflowValue(k, newVal) {
			continue
		}
		if oldTask != nil {
			oldVal := typedWorkflowValue(oldTask, k)
			if workflowValuesEqual(k, newVal, oldVal) {
				continue
			}
		}
		if newTask.WorkflowFrontmatter == nil {
			newTask.WorkflowFrontmatter = make(map[string]interface{})
		}
		newTask.WorkflowFrontmatter[k] = newVal
	}
}

// typedWorkflowValue projects a Task's typed workflow field back into the
// generic shape that lives in WorkflowFrontmatter. Kept in sync with
// workflowFrontmatterFromTask so the two paths do not drift.
func typedWorkflowValue(t *Task, key string) interface{} {
	if t == nil {
		return nil
	}
	switch key {
	case fmKeyStatus:
		return string(t.Status)
	case fmKeyType:
		return string(t.Type)
	case fmKeyTags:
		if len(t.Tags) == 0 {
			return []string(nil)
		}
		return append([]string(nil), t.Tags...)
	case fmKeyDependsOn:
		if len(t.DependsOn) == 0 {
			return []string(nil)
		}
		return append([]string(nil), t.DependsOn...)
	case fmKeyDue:
		return t.Due
	case fmKeyRecurrence:
		return string(t.Recurrence)
	case fmKeyAssignee:
		return t.Assignee
	case fmKeyPriority:
		return t.Priority
	case fmKeyPoints:
		return t.Points
	}
	return nil
}

// workflowValuesEqual compares two projected values by workflow-field key.
// Slice compares are order-sensitive; callers have already normalized the
// task's tags/dependsOn slices through NormalizeCollectionFields.
func workflowValuesEqual(key string, a, b interface{}) bool {
	switch key {
	case fmKeyTags, fmKeyDependsOn:
		sa, _ := a.([]string)
		sb, _ := b.([]string)
		if len(sa) != len(sb) {
			return false
		}
		for i := range sa {
			if sa[i] != sb[i] {
				return false
			}
		}
		return true
	case fmKeyDue:
		ta, _ := a.(time.Time)
		tb, _ := b.(time.Time)
		return ta.Equal(tb)
	default:
		return a == b
	}
}

// isZeroWorkflowValue reports whether a projected value represents the
// absence of the field — used to drop cleared fields from the presence set.
func isZeroWorkflowValue(key string, v interface{}) bool {
	switch key {
	case fmKeyTags, fmKeyDependsOn:
		s, _ := v.([]string)
		return len(s) == 0
	case fmKeyDue:
		t, _ := v.(time.Time)
		return t.IsZero()
	case fmKeyStatus, fmKeyType, fmKeyRecurrence, fmKeyAssignee:
		s, _ := v.(string)
		return s == ""
	case fmKeyPriority, fmKeyPoints:
		n, _ := v.(int)
		return n == 0
	}
	return v == nil
}

// extractWorkflowSubset returns a map containing just the workflow keys
// present in src. Absent keys stay absent. Nil when src is nil or has no
// workflow keys.
func extractWorkflowSubset(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	var out map[string]interface{}
	for _, k := range workflowKeys {
		if v, ok := src[k]; ok {
			if out == nil {
				out = make(map[string]interface{})
			}
			out[k] = v
		}
	}
	return out
}

// workflowKeys is the ordered list of workflow keys the adapter projects.
// The order does not matter for correctness but keeps the preserved map
// stable across runs when callers iterate it.
var workflowKeys = []string{
	fmKeyStatus,
	fmKeyType,
	fmKeyTags,
	fmKeyDependsOn,
	fmKeyDue,
	fmKeyRecurrence,
	fmKeyAssignee,
	fmKeyPriority,
	fmKeyPoints,
}

// applyWorkflowFrontmatter copies presence-bearing workflow fields from fm
// into t. Fields absent from fm stay zero-valued on t so plain docs keep
// their absence semantics through the adapter boundary.
func applyWorkflowFrontmatter(t *Task, fm map[string]interface{}) {
	if t == nil || fm == nil {
		return
	}
	if v, ok := fm[fmKeyStatus]; ok {
		if s, ok := v.(string); ok {
			t.Status = Status(s)
		}
	}
	if v, ok := fm[fmKeyType]; ok {
		if s, ok := v.(string); ok {
			t.Type = Type(s)
		}
	}
	if v, ok := fm[fmKeyTags]; ok {
		t.Tags = coerceStringSlice(v)
	}
	if v, ok := fm[fmKeyDependsOn]; ok {
		t.DependsOn = coerceStringSlice(v)
	}
	if v, ok := fm[fmKeyDue]; ok {
		if due, ok := v.(time.Time); ok {
			t.Due = due
		}
	}
	if v, ok := fm[fmKeyAssignee]; ok {
		if s, ok := v.(string); ok {
			t.Assignee = s
		}
	}
	if v, ok := fm[fmKeyRecurrence]; ok {
		if s, ok := v.(string); ok {
			t.Recurrence = Recurrence(s)
		}
	}
	if v, ok := fm[fmKeyPriority]; ok {
		t.Priority = coerceInt(v)
	}
	if v, ok := fm[fmKeyPoints]; ok {
		t.Points = coerceInt(v)
	}
}

// workflowFrontmatterFromTask produces a Frontmatter map containing only
// the workflow fields the Task actually carries. A Task with IsWorkflow=false
// and all zero-valued workflow fields produces a nil map, so ToDocument of a
// plain Task returns a Document that classifies as plain.
func workflowFrontmatterFromTask(t *Task) map[string]interface{} {
	if t == nil {
		return nil
	}
	var fm map[string]interface{}
	put := func(k string, v interface{}) {
		if fm == nil {
			fm = make(map[string]interface{})
		}
		fm[k] = v
	}
	if s := string(t.Status); s != "" {
		put(fmKeyStatus, s)
	}
	if s := string(t.Type); s != "" {
		put(fmKeyType, s)
	}
	if len(t.Tags) > 0 {
		put(fmKeyTags, append([]string(nil), t.Tags...))
	}
	if len(t.DependsOn) > 0 {
		put(fmKeyDependsOn, append([]string(nil), t.DependsOn...))
	}
	if !t.Due.IsZero() {
		put(fmKeyDue, t.Due)
	}
	if t.Assignee != "" {
		put(fmKeyAssignee, t.Assignee)
	}
	if s := string(t.Recurrence); s != "" {
		put(fmKeyRecurrence, s)
	}
	if t.Priority != 0 {
		put(fmKeyPriority, t.Priority)
	}
	if t.Points != 0 {
		put(fmKeyPoints, t.Points)
	}
	return fm
}

// coerceStringSlice handles the two shapes YAML decoding leaves behind:
// either []interface{} of strings (fresh decode) or []string (already-
// normalized). Anything else becomes nil so a malformed field does not
// silently corrupt the Task.
func coerceStringSlice(v interface{}) []string {
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

// coerceInt accepts int/int64/float64 because yaml.v3 decodes numerics
// inconsistently. Non-numeric values coerce to 0 so absent-equivalent
// preserves the presence-aware contract.
func coerceInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		if n == float64(int64(n)) {
			return int(n)
		}
		return 0
	default:
		return 0
	}
}

// cloneMap returns a shallow copy; the Task <-> Document boundary never
// needs deep copies of custom-field values because they are already
// immutable-ish (scalars, or []string slices normalized at the store edge).
func cloneMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
