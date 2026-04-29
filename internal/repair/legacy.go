// Package repair implements the `tiki repair ids` command. It is the only
// place in the codebase that knows about legacy TIKI-XXXXXX identifiers —
// every other component treats them as invalid. Keeping legacy detection
// private to this package prevents accidental reintroduction of a
// compatibility layer elsewhere.
package repair

import (
	"regexp"
)

// legacyTikiIDPattern matches pre-unification TIKI-XXXXXX identifiers. Only
// the repair command uses this; nothing else in the codebase should depend
// on legacy identity.
var legacyTikiIDPattern = regexp.MustCompile(`^TIKI-[A-Z0-9]{6}$`)

// isLegacyTikiID reports whether id is a pre-unification TIKI-XXXXXX value.
func isLegacyTikiID(id string) bool {
	return legacyTikiIDPattern.MatchString(id)
}
