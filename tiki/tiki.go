// Package tiki defines the unified Tiki model that will eventually replace
// task.Task and document.Document across the codebase.
//
// Phase 2 introduces this package alongside the existing task/document split.
// Existing APIs continue to work unchanged; later phases migrate persistence,
// ruki, services, and UI to operate on *Tiki directly.
//
// A Tiki carries three special elements — ID, Title, and Body — that drive
// identity, display, and markdown content. Every other piece of structured
// state lives in the generic Fields map. There is no workflow/plain
// classification at this layer: field presence is just map presence.
package tiki

import "time"

// Tiki is the unified representation of any managed markdown file under .doc/.
// It is intentionally workflow-agnostic: status/type/priority/etc. are ordinary
// entries in Fields with explicit presence, so a document with only id+title
// cannot be accidentally promoted by a body-only edit.
type Tiki struct {
	// ID is the bare uppercase identifier (6 alphanumeric chars). Always set
	// for loaded tikis; generated on creation if absent.
	ID string

	// Title comes from frontmatter.title, or is empty when omitted.
	Title string

	// Body is the markdown content after the closing --- delimiter. For pure
	// markdown files without frontmatter this is the whole file.
	Body string

	// Fields holds every frontmatter key other than id and title, in their
	// raw or coerced form. Schema-known keys (status, type, priority, points,
	// dependsOn, due, recurrence, assignee, tags) use their typed values;
	// unknown keys round-trip verbatim.
	Fields map[string]interface{}

	// Path is the on-disk path at load time. Authoritative file target for
	// saves; mutable as the user moves the file around.
	Path string

	// LoadedMtime is the file mtime at load time; used for optimistic locking.
	LoadedMtime time.Time

	// CreatedAt is best-effort creation time (git history -> file mtime).
	CreatedAt time.Time

	// UpdatedAt is max(file mtime, latest git commit time).
	UpdatedAt time.Time

	// stale records the subset of Fields keys that were loaded from disk
	// as UnknownFields provenance despite matching a registered Custom
	// field (e.g. a value that failed coercion on load and was demoted).
	// These keys round-trip back to Task.UnknownFields through ToTask so
	// persistence's validateCustomFields does not reject the save. Any
	// explicit Set or Delete clears the stale marker — an overwrite is
	// treated as a legitimate repair, restoring normal Custom routing.
	//
	// The map is intentionally unexported: it is provenance metadata, not
	// part of the generic Tiki.Fields contract that ruki and API callers
	// reason about.
	stale map[string]struct{}
}

// SearchResult pairs a tiki with a relevance score (higher is better).
type SearchResult struct {
	Tiki  *Tiki
	Score float64
}

// New returns a freshly allocated *Tiki with an empty Fields map. Using this
// instead of &Tiki{} guarantees callers can Set fields without a nil check.
func New() *Tiki {
	return &Tiki{Fields: map[string]interface{}{}}
}

// Clone returns a deep copy. Slice values inside Fields are copied; maps
// nested inside Fields are copied one level (sufficient for all current
// schema-known values and the unknown-field round-trip case).
func (t *Tiki) Clone() *Tiki {
	if t == nil {
		return nil
	}
	clone := &Tiki{
		ID:          t.ID,
		Title:       t.Title,
		Body:        t.Body,
		Path:        t.Path,
		LoadedMtime: t.LoadedMtime,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
	if t.Fields != nil {
		clone.Fields = make(map[string]interface{}, len(t.Fields))
		for k, v := range t.Fields {
			clone.Fields[k] = cloneFieldValue(v)
		}
	}
	if t.stale != nil {
		clone.stale = make(map[string]struct{}, len(t.stale))
		for k := range t.stale {
			clone.stale[k] = struct{}{}
		}
	}
	return clone
}

// cloneFieldValue deep-copies slice-typed values so mutations through the
// clone do not affect the original. Scalars and time values are returned
// as-is; maps are copied one level deep.
func cloneFieldValue(v interface{}) interface{} {
	switch val := v.(type) {
	case []string:
		cp := make([]string, len(val))
		copy(cp, val)
		return cp
	case []interface{}:
		cp := make([]interface{}, len(val))
		for i, e := range val {
			cp[i] = cloneFieldValue(e)
		}
		return cp
	case map[string]interface{}:
		cp := make(map[string]interface{}, len(val))
		for k, e := range val {
			cp[k] = cloneFieldValue(e)
		}
		return cp
	default:
		return v
	}
}
