package ruki

import "github.com/boolean-maybe/tiki/ruki/keyword"

// IsReservedKeyword reports whether name is a ruki reserved word (case-insensitive).
func IsReservedKeyword(name string) bool {
	return keyword.IsReserved(name)
}

// ReservedKeywordsList returns a copy of the reserved keyword list.
func ReservedKeywordsList() []string {
	return keyword.List()
}

// keywordPattern returns the regex alternation for the lexer Keyword rule.
func keywordPattern() string {
	return keyword.Pattern()
}
