package ruki

import "strings"

// reservedKeywords is the canonical, immutable list of ruki reserved words.
// the lexer regex and IsReservedKeyword both derive from this single source.
var reservedKeywords = [...]string{
	"select", "create", "update", "delete",
	"where", "set", "order", "by",
	"asc", "desc",
	"before", "after", "deny", "run",
	"and", "or", "not",
	"is", "empty", "in",
	"any", "all",
	"old", "new",
}

// reservedSet is a case-insensitive lookup built from reservedKeywords.
var reservedSet map[string]struct{}

func init() {
	reservedSet = make(map[string]struct{}, len(reservedKeywords))
	for _, kw := range reservedKeywords {
		reservedSet[strings.ToLower(kw)] = struct{}{}
	}
}

// IsReservedKeyword reports whether name is a ruki reserved word (case-insensitive).
func IsReservedKeyword(name string) bool {
	_, ok := reservedSet[strings.ToLower(name)]
	return ok
}

// ReservedKeywordsList returns a copy of the reserved keyword list.
func ReservedKeywordsList() []string {
	result := make([]string, len(reservedKeywords))
	copy(result, reservedKeywords[:])
	return result
}

// keywordPattern returns the regex alternation for the lexer Keyword rule.
// called once during lexer init.
func keywordPattern() string {
	return `\b(` + strings.Join(reservedKeywords[:], "|") + `)\b`
}
