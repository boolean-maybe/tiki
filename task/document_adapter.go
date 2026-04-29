package task

import (
	"strings"

	"github.com/boolean-maybe/tiki/document"
)

// FromDocument projects a Document into a Task using the Phase 1 mapping
// rules. It is a temporary seam: later phases flip storage to be
// document-native and this function will be inverted (the store will own
// Documents directly; Task will become a thin adapter for legacy callers).
//
// Mapping rules (from plan Phase 4):
//   - id         → Task.ID
//   - title      → Task.Title
//   - body       → Task.Description (trimmed whitespace, matching loadTaskFile)
//   - path/times → FilePath / LoadedMtime / CreatedAt / UpdatedAt
//
// Workflow fields (status, type, priority, etc.) are NOT mapped here because
// they live in Frontmatter with presence-awareness — unpacking them requires
// the workflow field registry and value coercion that currently live in
// store/tikistore. Callers that need a workflow-capable Task should continue
// to use TikiStore.loadTaskFile for now; this adapter is a pure shape
// conversion for documents that have already been promoted elsewhere.
//
// FromDocument does NOT default-promote a plain Document into a workflow
// item: fields absent from the source stay zero-valued, and callers are
// responsible for deciding whether the resulting Task should be treated as
// workflow-capable (see the "presence-aware workflow fields" section of the
// plan).
func FromDocument(d *document.Document) *Task {
	if d == nil {
		return nil
	}
	return &Task{
		ID:            d.ID,
		Title:         d.Title,
		Description:   strings.TrimSpace(d.Body),
		CustomFields:  cloneMap(d.CustomFields),
		UnknownFields: cloneMap(d.UnknownFields),
		LoadedMtime:   d.LoadedMtime,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
		FilePath:      d.Path,
	}
}

// ToDocument projects a Task into a Document. It is lossy in the same way
// FromDocument is: workflow fields typed on the Task do not round-trip into
// the returned Document's Frontmatter — callers that need that must build
// the frontmatter map themselves (the store owns the canonical workflow
// encoding today). Phase 4 will make this symmetric.
func ToDocument(t *Task) *document.Document {
	if t == nil {
		return nil
	}
	return &document.Document{
		ID:            t.ID,
		Title:         t.Title,
		Body:          t.Description,
		Path:          t.FilePath,
		CustomFields:  cloneMap(t.CustomFields),
		UnknownFields: cloneMap(t.UnknownFields),
		LoadedMtime:   t.LoadedMtime,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
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
