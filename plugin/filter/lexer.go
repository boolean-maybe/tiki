package filter

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/boolean-maybe/tiki/util/parsing"
)

// TokenType represents the type of a lexer token
type TokenType int

const (
	TokenEOF      TokenType = iota
	TokenIdent              // field names like status, type, NOW, CURRENT_USER
	TokenString             // 'value' or "value"
	TokenNumber             // integer literals
	TokenDuration           // 24hour, 1week, etc.
	TokenOperator           // =, ==, !=, >, <, >=, <=, +, -
	TokenAnd
	TokenOr
	TokenNot
	TokenIn    // IN keyword
	TokenNotIn // NOT IN keyword combination
	TokenLParen
	TokenRParen
	TokenLBracket // [ for list literals
	TokenRBracket // ] for list literals
	TokenComma    // , for list elements
)

// Token represents a lexer token
type Token struct {
	Type  TokenType
	Value string
}

// Multi-character operators mapped to their token types
var multiCharOps = map[string]TokenType{
	"==": TokenOperator,
	"!=": TokenOperator,
	">=": TokenOperator,
	"<=": TokenOperator,
}

// Keywords mapped to their token types
var keywords = map[string]TokenType{
	"AND": TokenAnd,
	"OR":  TokenOr,
	"NOT": TokenNot,
	"IN":  TokenIn,
}

// Time field names (uppercase)
var timeFields = map[string]bool{
	"NOW":       true,
	"CREATEDAT": true,
	"UPDATEDAT": true,
}

// isTimeField checks if a given identifier is a time field (case-insensitive)
func isTimeField(name string) bool {
	return timeFields[strings.ToUpper(name)]
}

// Tokenize breaks the expression into tokens
func Tokenize(expr string) ([]Token, error) {
	var tokens []Token
	i := 0

	for i < len(expr) {
		// Skip whitespace
		i = parsing.SkipWhitespace(expr, i)
		if i >= len(expr) {
			break
		}

		// Check for two-character operators first
		if i+1 < len(expr) {
			twoChar := expr[i : i+2]
			if tokType, ok := multiCharOps[twoChar]; ok {
				tokens = append(tokens, Token{Type: tokType, Value: twoChar})
				i += 2
				continue
			}
		}

		// Single character tokens
		switch expr[i] {
		case '(':
			tokens = append(tokens, Token{Type: TokenLParen, Value: "("})
			i++
			continue
		case ')':
			tokens = append(tokens, Token{Type: TokenRParen, Value: ")"})
			i++
			continue
		case '[':
			tokens = append(tokens, Token{Type: TokenLBracket, Value: "["})
			i++
			continue
		case ']':
			tokens = append(tokens, Token{Type: TokenRBracket, Value: "]"})
			i++
			continue
		case ',':
			tokens = append(tokens, Token{Type: TokenComma, Value: ","})
			i++
			continue
		case '=', '>', '<', '+', '-':
			tokens = append(tokens, Token{Type: TokenOperator, Value: string(expr[i])})
			i++
			continue
		case '\'', '"':
			// String literal
			quote := expr[i]
			i++
			start := i
			for i < len(expr) && expr[i] != quote {
				i++
			}
			if i >= len(expr) {
				return nil, fmt.Errorf("unterminated string literal")
			}
			tokens = append(tokens, Token{Type: TokenString, Value: expr[start:i]})
			i++ // skip closing quote
			continue
		}

		// Check for identifiers, keywords, numbers, and durations
		if unicode.IsLetter(rune(expr[i])) || expr[i] == '_' {
			word, newPos := parsing.ReadWhile(expr, i, func(r rune) bool {
				return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
			})
			i = newPos
			wordUpper := strings.ToUpper(word)

			// Check for "NOT IN" keyword combination
			if wordUpper == "NOT" {
				if matched, endPos := parsing.PeekKeyword(expr, i, "IN"); matched {
					i = endPos
					tokens = append(tokens, Token{Type: TokenNotIn, Value: "NOT IN"})
					continue
				}
			}

			// Check if it's a keyword
			if tokType, ok := keywords[wordUpper]; ok {
				tokens = append(tokens, Token{Type: tokType, Value: word})
			} else {
				tokens = append(tokens, Token{Type: TokenIdent, Value: word})
			}
			continue
		}

		// Check for numbers (which might be followed by duration unit)
		if unicode.IsDigit(rune(expr[i])) {
			numStr, newPos := parsing.ReadWhile(expr, i, unicode.IsDigit)
			i = newPos

			// Check if followed by duration unit
			if i < len(expr) && unicode.IsLetter(rune(expr[i])) {
				unitStr, unitEnd := parsing.ReadWhile(expr, i, unicode.IsLetter)
				fullWord := numStr + unitStr
				// Check if it's a valid duration
				if IsDurationLiteral(fullWord) {
					tokens = append(tokens, Token{Type: TokenDuration, Value: fullWord})
					i = unitEnd
				} else {
					// Not a valid duration, just a number
					tokens = append(tokens, Token{Type: TokenNumber, Value: numStr})
				}
			} else {
				tokens = append(tokens, Token{Type: TokenNumber, Value: numStr})
			}
			continue
		}

		return nil, fmt.Errorf("unexpected character at position %d: %c", i, expr[i])
	}

	tokens = append(tokens, Token{Type: TokenEOF, Value: ""})
	return tokens, nil
}
