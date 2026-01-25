package filter

import (
	"fmt"
	"strings"
)

// ParseFilter parses a filter expression string into an AST.
// This is the main public entry point for filter parsing.
//
// Example expressions:
//   - status = 'done'
//   - type = 'bug' AND priority > 2
//   - status IN ['ready', 'in_progress']
//   - NOW - CreatedAt < 24hour
//   - (status = 'ready' OR status = 'in_progress') AND priority >= 3
//
// Returns nil expression for empty string (no filtering).
func ParseFilter(expr string) (FilterExpr, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil // empty filter = no filtering (all tasks pass)
	}

	tokens, err := Tokenize(expr)
	if err != nil {
		return nil, err
	}

	parser := newFilterParser(tokens)
	result, err := parser.parseExpr()
	if err != nil {
		return nil, err
	}

	// Ensure we consumed all tokens
	if parser.pos < len(parser.tokens) && parser.tokens[parser.pos].Type != TokenEOF {
		return nil, fmt.Errorf("unexpected token at position %d: %s", parser.pos, parser.tokens[parser.pos].Value)
	}

	return result, nil
}
