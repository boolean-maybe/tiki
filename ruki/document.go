package ruki

import "time"

// Document is the unit ruki reads and mutates. Hosts implement it on their own
// document type. ID/Path and the timestamps are read-only from ruki's view;
// Title and Body are written only by the create path; all declared-field
// mutation flows through Set/Delete.
type Document interface {
	ID() string
	Title() string
	Body() string
	Path() string
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetTitle(v string)
	SetBody(v string)

	Get(name string) (value interface{}, ok bool)
	Set(name string, value interface{})
	Has(name string) bool
	Delete(name string)

	Clone() Document
}

// DocumentFactory builds a blank Document for create statements.
type DocumentFactory func() Document

// fieldDependsOn is the well-known field name the blocks() builtin scans to
// find dependency edges. ruki references it by name rather than depending on
// any host's field-name registry.
const fieldDependsOn = "dependsOn"

// IsIdentityField reports whether name is one of ruki's reserved identity/audit
// field names that live outside the generic field map and are immutable via Set.
func IsIdentityField(name string) bool {
	switch name {
	case "id", "title", "description", "body",
		"createdBy", "createdAt", "updatedAt",
		"filepath", "path":
		return true
	}
	return false
}
