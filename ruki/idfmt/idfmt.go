// Package idfmt holds the canonical bare-document-ID *format* rules: the
// fixed length, the validation regex, and the membership test. It is the
// single source of truth for "is this string a valid bare document id".
//
// ID *generation* (the gonanoid-backed NewID) intentionally lives in the
// document package, not here — idfmt knows only what a valid id looks like,
// not how to mint one.
package idfmt

import "regexp"

// IDLength is the number of characters in a bare document ID.
// Changing it to 8 later should only require updating this constant and the
// embedded length in idPattern below.
const IDLength = 6

// idPattern is the single authoritative regex for bare document IDs.
// Keep the {6} width in sync with IDLength.
var idPattern = regexp.MustCompile(`^[A-Z0-9]{6}$`)

// IsValidID reports whether id matches the canonical bare document ID format.
// Legacy TIKI-XXXXXX values are NOT valid and have no compatibility shim.
func IsValidID(id string) bool {
	return idPattern.MatchString(id)
}
