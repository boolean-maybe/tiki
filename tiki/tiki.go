// Package tiki defines the unified Tiki document model used across the
// codebase: persistence, ruki, services, and UI all operate on *Tiki directly.
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
	// id is the bare uppercase identifier (6 alphanumeric chars). Always set
	// for loaded tikis; generated on creation if absent. Read via ID().
	id string

	// title comes from frontmatter.title, or is empty when omitted. Read via Title().
	title string

	// body is the markdown content after the closing --- delimiter. For pure
	// markdown files without frontmatter this is the whole file. Read via Body().
	body string

	// Fields holds every frontmatter key other than id and title, in their
	// raw or coerced form. Schema-known keys (status, type, priority, points,
	// dependsOn, due, recurrence, assignee, tags) use their typed values;
	// unknown keys round-trip verbatim.
	Fields map[string]interface{}

	// path is the on-disk path at load time. Authoritative file target for
	// saves; mutable as the user moves the file around. Read via Path().
	path string

	// LoadedMtime is the file mtime at load time; used for optimistic locking.
	LoadedMtime time.Time

	// createdAt is best-effort creation time (git history -> file mtime). Read via CreatedAt().
	createdAt time.Time

	// updatedAt is max(file mtime, latest git commit time). Read via UpdatedAt().
	updatedAt time.Time

	// stale records the subset of Fields keys that were loaded from disk
	// with unknown-field provenance despite matching a registered Custom
	// field (e.g. a value that failed coercion on load and was demoted).
	// Persistence preserves these as unknown-bucket entries on save so
	// validateCustomFields does not reject the round-trip. Any explicit
	// Set or Delete clears the stale marker — an overwrite is treated as
	// a legitimate repair, restoring normal Custom routing.
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

// special read-only attribute accessors.
func (t *Tiki) ID() string           { return t.id }
func (t *Tiki) Title() string        { return t.title }
func (t *Tiki) Body() string         { return t.body }
func (t *Tiki) Path() string         { return t.path }
func (t *Tiki) CreatedAt() time.Time { return t.createdAt }
func (t *Tiki) UpdatedAt() time.Time { return t.updatedAt }

// setters for the store/service/template construction paths.
func (t *Tiki) SetID(v string)           { t.id = v }
func (t *Tiki) SetTitle(v string)        { t.title = v }
func (t *Tiki) SetBody(v string)         { t.body = v }
func (t *Tiki) SetPath(v string)         { t.path = v }
func (t *Tiki) SetCreatedAt(v time.Time) { t.createdAt = v }
func (t *Tiki) SetUpdatedAt(v time.Time) { t.updatedAt = v }

// Clone returns a deep copy. Slice values inside Fields are copied; maps
// nested inside Fields are copied one level (sufficient for all current
// workflow-declared values and the unknown-field round-trip case).
func (t *Tiki) Clone() *Tiki {
	if t == nil {
		return nil
	}
	clone := &Tiki{
		id:          t.id,
		title:       t.title,
		body:        t.body,
		path:        t.path,
		LoadedMtime: t.LoadedMtime,
		createdAt:   t.createdAt,
		updatedAt:   t.updatedAt,
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
