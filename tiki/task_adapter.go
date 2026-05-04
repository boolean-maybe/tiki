package tiki

import (
	"time"

	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

// FromTask projects a *task.Task into a *tiki.Tiki for ruki execution.
//
// Phase 4 adapter at the task/tiki boundary: store callers continue to hold
// *task.Task, ruki operates on *tiki.Tiki.
//
// Schema-known fields land in Fields with canonical Go types (int for
// priority/points, string for status/type/recurrence/assignee, []string for
// tags/dependsOn, time.Time for due). Custom and unknown fields are merged
// into Fields verbatim. Presence mirrors the source task's presence semantics:
//
//   - IsWorkflow + WorkflowFrontmatter present → only keys listed in the
//     presence map land in Fields (sparse fidelity).
//   - IsWorkflow without WorkflowFrontmatter (in-memory workflow template from
//     NewTaskTemplate / hand-built test fixture) → schema-known fields land
//     iff their typed value is non-zero, matching the pre-Phase-3
//     "value-derived presence" rule. Exceptions: status and type always land
//     so classification survives at least one workflow marker.
//   - Not IsWorkflow → no schema-known fields land; only identity +
//     custom/unknown Fields.
//
// Identity/audit fields (ID, Title, Body=Description, Path, CreatedAt,
// UpdatedAt, LoadedMtime) always populate.
func FromTask(t *task.Task) *Tiki {
	if t == nil {
		return nil
	}
	out := New()
	out.ID = t.ID
	out.Title = t.Title
	out.Body = t.Description
	out.Path = t.FilePath
	out.LoadedMtime = t.LoadedMtime
	out.CreatedAt = t.CreatedAt
	out.UpdatedAt = t.UpdatedAt

	// CreatedBy is identity/audit metadata. Tiki's canonical model does
	// not carry it as a first-class field, but store it in Fields under
	// the "createdBy" key so ruki's identity-field reads on *tiki.Tiki
	// continue to resolve.
	if t.CreatedBy != "" {
		out.Set("createdBy", t.CreatedBy)
	}

	// comments are stored in Fields under "comments" so the round-trip
	// through Tiki does not lose in-memory comment state.
	if len(t.Comments) > 0 {
		cp := make([]task.Comment, len(t.Comments))
		copy(cp, t.Comments)
		out.Set("comments", cp)
	}

	if t.IsWorkflow {
		if t.WorkflowFrontmatter != nil {
			applyPresenceMap(out, t, t.WorkflowFrontmatter)
		} else {
			applyDerivedPresence(out, t)
		}
	}

	for k, v := range t.CustomFields {
		out.Set(k, cloneFieldValue(v))
	}
	// UnknownFields carries two kinds of entries: truly-unregistered keys
	// (round-trip naturally through ToTask's workflow.Field() lookup),
	// and registered-Custom keys whose values failed coercion on load
	// (demoted by persistence to preserve the original bytes for repair).
	// For the second kind, mark the Fields entry stale so ToTask routes
	// it back to UnknownFields — otherwise validateCustomFields on save
	// would reject the never-repaired value.
	for k, v := range t.UnknownFields {
		out.Set(k, cloneFieldValue(v))
		if fd, registered := workflow.Field(k); registered && fd.Custom {
			out.markStale(k)
		}
	}

	return out
}

// applyPresenceMap copies the presence-marked subset of typed workflow fields
// from t into out.Fields. Mirrors the sparse-emission rule used by the
// persistence save path.
func applyPresenceMap(out *Tiki, t *task.Task, presence map[string]interface{}) {
	for k := range presence {
		setWorkflowFieldFromTask(out, t, k)
	}
}

// applyDerivedPresence populates out.Fields from t's typed values using
// value-derived presence: status and type always land; the rest only when
// non-zero. Matches the pre-Phase-3 yaml-struct behavior for in-memory
// workflow tasks whose WorkflowFrontmatter is nil.
func applyDerivedPresence(out *Tiki, t *task.Task) {
	setWorkflowFieldFromTask(out, t, FieldStatus)
	setWorkflowFieldFromTask(out, t, FieldType)
	if len(t.Tags) > 0 {
		setWorkflowFieldFromTask(out, t, FieldTags)
	}
	if len(t.DependsOn) > 0 {
		setWorkflowFieldFromTask(out, t, FieldDependsOn)
	}
	if !t.Due.IsZero() {
		setWorkflowFieldFromTask(out, t, FieldDue)
	}
	if string(t.Recurrence) != "" {
		setWorkflowFieldFromTask(out, t, FieldRecurrence)
	}
	if t.Assignee != "" {
		setWorkflowFieldFromTask(out, t, FieldAssignee)
	}
	if t.Priority != 0 {
		setWorkflowFieldFromTask(out, t, FieldPriority)
	}
	if t.Points != 0 {
		setWorkflowFieldFromTask(out, t, FieldPoints)
	}
}

// setWorkflowFieldFromTask writes the named typed field from t into out in
// its canonical Go type.
func setWorkflowFieldFromTask(out *Tiki, t *task.Task, name string) {
	switch name {
	case FieldStatus:
		out.Set(name, string(t.Status))
	case FieldType:
		out.Set(name, string(t.Type))
	case FieldTags:
		out.Set(name, append([]string(nil), t.Tags...))
	case FieldDependsOn:
		out.Set(name, append([]string(nil), t.DependsOn...))
	case FieldDue:
		out.Set(name, t.Due)
	case FieldRecurrence:
		out.Set(name, string(t.Recurrence))
	case FieldAssignee:
		out.Set(name, t.Assignee)
	case FieldPriority:
		out.Set(name, t.Priority)
	case FieldPoints:
		out.Set(name, t.Points)
	}
}

// ToTask projects a *tiki.Tiki back to a *task.Task. Phase 4 adapter at the
// ruki/store boundary for mutation results: ruki produces updated tikis,
// store callers need tasks to hand to the mutation gate.
//
// Body → Description. Fields keys that match a schema-known workflow name
// populate the typed Task field. Remaining keys are split via the workflow
// registry: registered custom fields land in Task.CustomFields, everything
// else lands in Task.UnknownFields. This split is load-bearing because
// store/tikistore/persistence.go:validateCustomFields rejects saves whose
// CustomFields hold unregistered keys — laundering an unknown key through
// CustomFields would fail every save for tikis with preserved
// round-trippable frontmatter.
//
// IsWorkflow is set iff any schema-known workflow key is present in Fields;
// WorkflowFrontmatter snapshots that presence set so persistence's
// sparse-marshal retains fidelity on save.
func ToTask(t *Tiki) *task.Task {
	if t == nil {
		return nil
	}
	out := &task.Task{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Body,
		FilePath:    t.Path,
		LoadedMtime: t.LoadedMtime,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}

	if v, ok := t.Fields["createdBy"]; ok {
		if s, isStr := v.(string); isStr {
			out.CreatedBy = s
		}
	}

	if v, ok := t.Fields["comments"]; ok {
		if comments, isSlice := v.([]task.Comment); isSlice {
			cp := make([]task.Comment, len(comments))
			copy(cp, comments)
			out.Comments = cp
		}
	}

	presence := map[string]interface{}{}
	for _, name := range SchemaKnownFields {
		v, ok := t.Fields[name]
		if !ok {
			continue
		}
		// Store the actual field value (not a struct{}{} presence marker) so
		// document round-trips through task.FromDocument/applyWorkflowFrontmatter
		// can recover the typed value. applyPresenceMap in FromTask only reads
		// the map keys (for presence), so actual values are ignored there; the
		// real typed value is read from the Task fields by setWorkflowFieldFromTask.
		presence[name] = v
		applyWorkflowFieldToTask(out, name, v)
	}

	if len(presence) > 0 {
		out.IsWorkflow = true
		out.WorkflowFrontmatter = presence
		// Mirror the pre-Phase-3 convention so downstream nil-guards and
		// reflect.DeepEqual comparisons keep working: workflow tasks always
		// carry non-nil Tags and DependsOn slices.
		if out.Tags == nil {
			out.Tags = []string{}
		}
		if out.DependsOn == nil {
			out.DependsOn = []string{}
		}
	}

	for k, v := range t.Fields {
		if IsSchemaKnown(k) || k == "createdBy" || k == "comments" {
			continue
		}
		fd, registered := workflow.Field(k)
		if registered && fd.Custom && !t.isStale(k) {
			if out.CustomFields == nil {
				out.CustomFields = map[string]interface{}{}
			}
			out.CustomFields[k] = cloneFieldValue(v)
			continue
		}
		if registered && !fd.Custom {
			// synthetic built-in (filepath etc.) — persistence drops these
			// on load, so do the same on save to match disk state.
			continue
		}
		// Either the key is unregistered, or it was registered Custom but
		// loaded as UnknownFields (stale provenance). Route to Unknown so
		// persistence's validateCustomFields does not reject the save.
		if out.UnknownFields == nil {
			out.UnknownFields = map[string]interface{}{}
		}
		out.UnknownFields[k] = cloneFieldValue(v)
	}

	return out
}

// applyWorkflowFieldToTask writes a Fields value onto the typed Task field.
// Type coercions are best-effort: ruki's executor has already validated the
// shape, so mismatches here are programming errors.
func applyWorkflowFieldToTask(out *task.Task, name string, v interface{}) {
	switch name {
	case FieldStatus:
		if s, ok := v.(string); ok {
			out.Status = task.Status(s)
		}
	case FieldType:
		if s, ok := v.(string); ok {
			out.Type = task.Type(s)
		}
	case FieldTags:
		out.Tags = coerceAdapterStringSlice(v)
	case FieldDependsOn:
		out.DependsOn = coerceAdapterStringSlice(v)
	case FieldDue:
		if d, ok := v.(time.Time); ok {
			out.Due = d
		}
	case FieldRecurrence:
		if s, ok := v.(string); ok {
			out.Recurrence = task.Recurrence(s)
		}
	case FieldAssignee:
		if s, ok := v.(string); ok {
			out.Assignee = s
		}
	case FieldPriority:
		if n, ok := v.(int); ok {
			out.Priority = n
		}
	case FieldPoints:
		if n, ok := v.(int); ok {
			out.Points = n
		}
	}
}

func coerceAdapterStringSlice(v interface{}) []string {
	switch vv := v.(type) {
	case []string:
		out := make([]string, len(vv))
		copy(out, vv)
		return out
	case []interface{}:
		out := make([]string, 0, len(vv))
		for _, e := range vv {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// IsIdentityField reports whether name refers to a special identity/audit
// field that always has a value on a loaded Tiki (and therefore never
// hard-errors on absent reads). Covers id, title, description, body,
// createdBy, createdAt, updatedAt, filepath, path.
func IsIdentityField(name string) bool {
	switch name {
	case "id", "title", "description", "body",
		"createdBy", "createdAt", "updatedAt",
		"filepath", "path":
		return true
	}
	return false
}
