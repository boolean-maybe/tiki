package keyword

import "strings"

// reserved is the canonical, immutable list of ruki reserved words.
var reserved = [...]string{
	"select", "create", "update", "delete",
	"where", "set", "order", "by",
	"asc", "desc", "limit",
	"before", "after", "deny", "run",
	"every", "clipboard",
	"and", "or", "not",
	"is", "empty", "in",
	"any", "all",
	"old", "new", "outer",
}

var reservedSet map[string]struct{}

func init() {
	reservedSet = make(map[string]struct{}, len(reserved))
	for _, kw := range reserved {
		reservedSet[strings.ToLower(kw)] = struct{}{}
	}
}

// IsReserved reports whether name is a ruki reserved word (case-insensitive).
func IsReserved(name string) bool {
	_, ok := reservedSet[strings.ToLower(name)]
	return ok
}

// List returns a copy of the reserved keyword list.
func List() []string {
	result := make([]string, len(reserved))
	copy(result, reserved[:])
	return result
}

// IdentPattern is the anchored regex for valid ruki identifiers.
// Kept in sync with the Ident rule in the ruki lexer.
const IdentPattern = `^[a-zA-Z_][a-zA-Z0-9_]*$`

// Pattern returns the regex alternation for the lexer Keyword rule.
func Pattern() string {
	return `\b(` + strings.Join(reserved[:], "|") + `)\b`
}
