package parsing

import (
	"strings"
	"unicode"
)

// SkipWhitespace skips whitespace characters starting from pos and returns the new position
func SkipWhitespace(s string, pos int) int {
	for pos < len(s) && unicode.IsSpace(rune(s[pos])) {
		pos++
	}
	return pos
}

// ReadWhile reads characters while the predicate returns true, starting from pos.
// Returns the substring and the new position.
func ReadWhile(s string, pos int, pred func(rune) bool) (string, int) {
	start := pos
	for pos < len(s) && pred(rune(s[pos])) {
		pos++
	}
	return s[start:pos], pos
}

// PeekChar returns the character at pos+offset and whether it exists.
// Returns (0, false) if the position is out of bounds.
func PeekChar(s string, pos, offset int) (rune, bool) {
	target := pos + offset
	if target < 0 || target >= len(s) {
		return 0, false
	}
	return rune(s[target]), true
}

// PeekKeyword checks if the next non-whitespace characters starting from pos
// match the given keyword (case-insensitive).
// Returns true if matched and the position after the keyword.
// Returns false and original position if not matched.
func PeekKeyword(s string, pos int, keyword string) (bool, int) {
	// Skip whitespace
	pos = SkipWhitespace(s, pos)

	// Check if we have enough characters
	if pos+len(keyword) > len(s) {
		return false, pos
	}

	// Check if the keyword matches (case-insensitive)
	nextWord := s[pos : pos+len(keyword)]
	if !strings.EqualFold(nextWord, keyword) {
		return false, pos
	}

	// Check that the keyword is followed by a non-letter (word boundary)
	endPos := pos + len(keyword)
	if endPos < len(s) && unicode.IsLetter(rune(s[endPos])) {
		return false, pos
	}

	return true, endPos
}
