package store

import (
	"github.com/boolean-maybe/tiki/tiki"
)

// TikiReadStore is the tiki-native read surface. Consumers that operate on
// *tiki.Tiki directly (ruki, new services) should depend on this interface.
type TikiReadStore interface {
	// GetTiki retrieves a tiki by ID. Returns nil when not found.
	GetTiki(id string) *tiki.Tiki

	// GetAllTikis returns every loaded tiki, including plain docs.
	GetAllTikis() []*tiki.Tiki

	// NewTikiTemplate returns a new tiki populated with creation defaults.
	NewTikiTemplate() (*tiki.Tiki, error)

	// SearchTikis searches all tikis with an optional tiki-native filter.
	// query matches against id, title, and body. filter is applied before
	// the text match; nil means no pre-filter. Results sorted by title then id.
	SearchTikis(query string, filter func(*tiki.Tiki) bool) []*tiki.Tiki
}

// TikiMutStore is the tiki-native full store surface.
type TikiMutStore interface {
	TikiReadStore

	// CreateTiki adds a new tiki to the store.
	CreateTiki(t *tiki.Tiki) error

	// UpdateTiki updates an existing tiki.
	UpdateTiki(t *tiki.Tiki) error

	// DeleteTiki removes a tiki from the store.
	DeleteTiki(id string)
}
