package document

import "time"

// Document is a frontmatter+body view of a managed markdown file. The
// runtime model is `tiki.Tiki`; Document is retained for ruki tests and
// helpers that build documents directly without going through the store.
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
