package filter

import "fmt"

// filterParser is a recursive descent parser for filter expressions
type filterParser struct {
	tokens []Token
	pos    int
}

// newFilterParser creates a new parser with the given tokens
func newFilterParser(tokens []Token) *filterParser {
	return &filterParser{tokens: tokens, pos: 0}
}

func (p *filterParser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *filterParser) advance() {
	p.pos++
}

func (p *filterParser) expect(t TokenType) error {
	tok := p.current()
	if tok.Type != t {
		return fmt.Errorf("expected token type %d, got %d (%s)", t, tok.Type, tok.Value)
	}
	p.advance()
	return nil
}

// parseLeftAssociativeBinary parses left-associative binary operations
// like "a AND b AND c" -> ((a AND b) AND c)
func (p *filterParser) parseLeftAssociativeBinary(
	operatorType TokenType,
	operatorStr string,
	subExprParser func() (FilterExpr, error),
) (FilterExpr, error) {
	left, err := subExprParser()
	if err != nil {
		return nil, err
	}

	for p.current().Type == operatorType {
		p.advance()
		right, err := subExprParser()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: operatorStr, Left: left, Right: right}
	}

	return left, nil
}

// parseExpr parses: expr = orExpr
func (p *filterParser) parseExpr() (FilterExpr, error) {
	return p.parseOrExpr()
}

// parseOrExpr parses: orExpr = andExpr (OR andExpr)*
func (p *filterParser) parseOrExpr() (FilterExpr, error) {
	return p.parseLeftAssociativeBinary(TokenOr, "OR", p.parseAndExpr)
}

// parseAndExpr parses: andExpr = notExpr (AND notExpr)*
func (p *filterParser) parseAndExpr() (FilterExpr, error) {
	return p.parseLeftAssociativeBinary(TokenAnd, "AND", p.parseNotExpr)
}

// parseNotExpr parses: notExpr = NOT notExpr | primaryExpr
func (p *filterParser) parseNotExpr() (FilterExpr, error) {
	if p.current().Type == TokenNot {
		p.advance()
		expr, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "NOT", Expr: expr}, nil
	}
	return p.parsePrimaryExpr()
}

// parsePrimaryExpr parses: primaryExpr = '(' expr ')' | inExpr | comparison
func (p *filterParser) parsePrimaryExpr() (FilterExpr, error) {
	if p.current().Type == TokenLParen {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(TokenRParen); err != nil {
			return nil, fmt.Errorf("expected closing parenthesis: %w", err)
		}
		return expr, nil
	}

	// Try to parse as IN expression or regular comparison
	// We need to look ahead to distinguish:
	//   field IN [...]     -> InExpr
	//   field NOT IN [...] -> InExpr
	//   field = value      -> CompareExpr

	// Check if this starts with an identifier (field name)
	if p.current().Type == TokenIdent {
		fieldName := p.current().Value
		p.advance()

		// Check next token for IN or NOT IN
		nextTok := p.current()

		if nextTok.Type == TokenIn {
			p.advance()
			return p.parseInExpr(fieldName, false)
		}
		if nextTok.Type == TokenNotIn {
			p.advance()
			return p.parseInExpr(fieldName, true)
		}

		// Otherwise, backtrack and parse as regular comparison
		p.pos--
		return p.parseComparison()
	}

	// Not an identifier, try parsing as comparison
	return p.parseComparison()
}
