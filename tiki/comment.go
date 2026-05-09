package tiki

import "time"

// Comment represents a comment on a tiki. Comments live in the in-memory
// Fields map under the "comments" key and are not persisted to frontmatter.
type Comment struct {
	ID        string
	Author    string
	Text      string
	CreatedAt time.Time
}
