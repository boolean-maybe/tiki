package store

import (
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// Store is the interface for task storage engines.
// Implementations must be thread-safe and notify listeners on changes.
type Store interface {
	ReadStore

	// CreateTiki adds a new tiki to the store.
	// Returns error if save fails (IO error, ErrConflict).
	CreateTiki(tk *tikipkg.Tiki) error

	// UpdateTiki updates an existing tiki using exact-presence semantics:
	// the tiki's field map is authoritative — absent fields are deleted.
	// Use this for ruki-result callers that have already computed the full
	// intended post-mutation state.
	// Returns error if save fails (IO error, ErrConflict).
	UpdateTiki(tk *tikipkg.Tiki) error

	// DeleteTiki removes a tiki from the store and deletes its file.
	DeleteTiki(id string)
}

// ChangeListener is called when the store's data changes
type ChangeListener func()

// Stat represents a statistic to be displayed in the header
type Stat struct {
	Name  string
	Value string
	Order int
}
