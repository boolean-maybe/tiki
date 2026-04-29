// Package document defines the unified document model shared by tikis and
// doki docs. Phase 1 introduces the model, a reusable frontmatter parser, and
// centralized ID validation. Later phases migrate the store, plugin, and view
// layers onto it.
package document

import (
	"fmt"
	"regexp"
	"strings"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

// IDLength is the number of characters in a bare document ID.
// Changing it to 8 later should only require updating this constant and the
// embedded length in idPattern below.
const IDLength = 6

// idAlphabet is the character set used to generate new bare document IDs.
// Uppercase + digits. Visually similar characters (e.g. O/0, I/1) remain in
// the alphabet to keep the generator simple; collisions are resolved via the
// store's unique-id retry loop.
const idAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// idPattern is the single authoritative regex for bare document IDs.
// Keep the {6} width in sync with IDLength.
var idPattern = regexp.MustCompile(`^[A-Z0-9]{6}$`)

// NewID returns a new bare document ID using the canonical alphabet and length.
// Callers MUST NOT add any prefix; identity is the raw ID. There is no
// TIKI- compatibility layer — legacy identifiers are only recognized by the
// repair command.
func NewID() string {
	id, err := gonanoid.Generate(idAlphabet, IDLength)
	if err != nil {
		// deterministic fallback so callers never observe an empty ID; the
		// uniqueness loop in the store will retry until a free slot exists.
		return strings.Repeat("0", IDLength)
	}
	return id
}

// NormalizeID trims whitespace and upper-cases the input. It does not validate.
func NormalizeID(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}

// IsValidID reports whether id matches the canonical bare document ID format.
// Legacy TIKI-XXXXXX values are NOT valid — the repair command is the only
// place in the codebase that recognizes them, and it does so privately.
func IsValidID(id string) bool {
	return idPattern.MatchString(id)
}

// ValidateID returns nil when id is a bare document ID and a descriptive error
// otherwise.
func ValidateID(id string) error {
	if !IsValidID(id) {
		return fmt.Errorf("invalid document id %q: expected %d uppercase alphanumeric characters", id, IDLength)
	}
	return nil
}

// Index tracks the set of document IDs currently loaded so duplicate detection
// and unique-ID generation can share a single source of truth. The zero value
// is not usable — use NewIndex.
type Index struct {
	ids map[string]string // id -> path
}

// NewIndex returns an empty, ready-to-use Index.
func NewIndex() *Index {
	return &Index{ids: make(map[string]string)}
}

// Register records id as observed at path. It returns an error if id is
// already registered at a different path, which callers should surface as a
// duplicate-ID load failure.
func (ix *Index) Register(id, path string) error {
	if ix == nil || ix.ids == nil {
		return fmt.Errorf("document index not initialized")
	}
	id = NormalizeID(id)
	if existing, ok := ix.ids[id]; ok && existing != path {
		return fmt.Errorf("duplicate document id %s at %s (already seen at %s)", id, path, existing)
	}
	ix.ids[id] = path
	return nil
}

// Unregister removes id from the index if present. Safe to call with an unknown id.
func (ix *Index) Unregister(id string) {
	if ix == nil || ix.ids == nil {
		return
	}
	delete(ix.ids, NormalizeID(id))
}

// Contains reports whether id is currently registered.
func (ix *Index) Contains(id string) bool {
	if ix == nil || ix.ids == nil {
		return false
	}
	_, ok := ix.ids[NormalizeID(id)]
	return ok
}

// PathFor returns the path registered for id, or the empty string if unknown.
func (ix *Index) PathFor(id string) string {
	if ix == nil || ix.ids == nil {
		return ""
	}
	return ix.ids[NormalizeID(id)]
}
