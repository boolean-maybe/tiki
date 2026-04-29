package document

import "time"

// Document is the unified representation of any managed markdown file under
// .doc/. It is intentionally workflow-agnostic: workflow fields live in
// Frontmatter/CustomFields with explicit presence, so ordinary docs can be
// loaded without being accidentally promoted to workflow items.
//
// Phase 1 introduces this type alongside task.Task; later phases migrate the
// store, ruki, and view layers to operate on Document directly.
type Document struct {
	// ID is the bare uppercase document identifier. Always set for loaded
	// documents; generated on creation if absent.
	ID string

	// Title comes from frontmatter.title, or is empty when omitted.
	Title string

	// Body is the document content after the closing --- delimiter. For
	// pure-markdown files without frontmatter this is the whole file.
	Body string

	// Path is the on-disk path at load time. Used as the authoritative file
	// target for saves; mutable as the user moves the file around.
	Path string

	// Frontmatter preserves the raw parsed YAML map, including keys the
	// Document type doesn't otherwise model. This is the source of truth for
	// presence-aware workflow fields in later phases.
	Frontmatter map[string]interface{}

	// CustomFields holds coerced values for workflow.yaml-declared custom
	// fields. Populated by the store boundary, not the parser.
	CustomFields map[string]interface{}

	// UnknownFields holds frontmatter keys that are neither built-in nor
	// registered custom fields. Preserved verbatim so metadata-only edits
	// round-trip cleanly.
	UnknownFields map[string]interface{}

	// LoadedMtime is the file mtime at load time; used for optimistic locking.
	LoadedMtime time.Time

	// CreatedAt is best-effort creation time (git history → file mtime).
	CreatedAt time.Time

	// UpdatedAt is max(file mtime, latest git commit time).
	UpdatedAt time.Time
}
