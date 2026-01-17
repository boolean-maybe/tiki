package filter

import (
	"fmt"
	"strconv"
)

// parseComparison parses comparison expressions like: field op value
func (p *filterParser) parseComparison() (FilterExpr, error) {
	// Parse left side (typically a field name or time expression)
	leftValue, leftIsTimeExpr, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	// Get operator
	tok := p.current()
	if tok.Type != TokenOperator {
		return nil, fmt.Errorf("expected comparison operator, got %s", tok.Value)
	}
	op := tok.Value
	p.advance()

	// Parse right side
	rightValue, rightIsTimeExpr, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	// Build comparison expression
	// If left side is a time expression like "NOW - CreatedAt", we need to handle it specially
	if leftIsTimeExpr {
		leftTimeExpr, _ := leftValue.(*TimeExpr)
		// The comparison becomes: (NOW - CreatedAt) < 24hour
		// which means: time.Since(CreatedAt) < 24hour
		return &CompareExpr{
			Field: "time_expr",
			Op:    op,
			Value: &timeExprCompare{left: leftTimeExpr, right: rightValue},
		}, nil
	}

	// Normal field comparison
	fieldName, ok := leftValue.(string)
	if !ok {
		return nil, fmt.Errorf("expected field name on left side of comparison")
	}

	// If right side is a time expression, wrap it
	if rightIsTimeExpr {
		return &CompareExpr{
			Field: fieldName,
			Op:    op,
			Value: rightValue,
		}, nil
	}

	return &CompareExpr{
		Field: fieldName,
		Op:    op,
		Value: rightValue,
	}, nil
}

// parseValueGeneric parses a value with optional time expression support
// allowTimeExpr controls whether to parse time expressions like "NOW - 24hour"
// Returns the value and whether it's a time expression
func (p *filterParser) parseValueGeneric(allowTimeExpr bool) (interface{}, bool, error) {
	tok := p.current()

	switch tok.Type {
	case TokenString:
		p.advance()
		return tok.Value, false, nil

	case TokenNumber:
		p.advance()
		num, err := strconv.Atoi(tok.Value)
		if err != nil {
			return nil, false, fmt.Errorf("invalid number: %s", tok.Value)
		}
		return num, false, nil

	case TokenDuration:
		if !allowTimeExpr {
			return nil, false, fmt.Errorf("duration not allowed in this context")
		}
		p.advance()
		dur, err := ParseDuration(tok.Value)
		if err != nil {
			return nil, false, err
		}
		return &DurationValue{Duration: dur}, false, nil

	case TokenIdent:
		ident := tok.Value
		p.advance()

		// Check if this is a time expression (only if allowed)
		if allowTimeExpr && isTimeField(ident) {
			// Check if followed by + or -
			if p.current().Type == TokenOperator {
				opTok := p.current()
				if opTok.Value == "+" || opTok.Value == "-" {
					p.advance()
					// Parse the operand (duration or another time field)
					operand, _, err := p.parseTimeOperand()
					if err != nil {
						return nil, false, err
					}
					return &TimeExpr{Base: ident, Op: opTok.Value, Operand: operand}, true, nil
				}
			}
			// Just a time field reference without arithmetic
			return &TimeExpr{Base: ident}, true, nil
		}

		// Regular identifier (field name or special value like CURRENT_USER)
		return ident, false, nil

	default:
		return nil, false, fmt.Errorf("unexpected token in value: %s", tok.Value)
	}
}

// parseValue parses a value for comparisons (allows time expressions)
func (p *filterParser) parseValue() (interface{}, bool, error) {
	return p.parseValueGeneric(true)
}

// parseTimeOperand parses the operand of a time expression (duration or field name)
func (p *filterParser) parseTimeOperand() (interface{}, bool, error) {
	tok := p.current()

	switch tok.Type {
	case TokenDuration:
		p.advance()
		dur, err := ParseDuration(tok.Value)
		if err != nil {
			return nil, false, err
		}
		return dur, false, nil

	case TokenIdent:
		ident := tok.Value
		p.advance()

		// Time field names
		if isTimeField(ident) {
			return ident, true, nil
		}

		return nil, false, fmt.Errorf("expected duration or time field, got: %s", ident)

	default:
		return nil, false, fmt.Errorf("expected duration or time field, got: %s", tok.Value)
	}
}

// parseInExpr parses: field IN [val1, val2, ...] or field NOT IN [...]
// This is called when we detect the IN pattern during primary expression parsing
func (p *filterParser) parseInExpr(fieldName string, isNotIn bool) (FilterExpr, error) {
	// Expect opening bracket
	if err := p.expect(TokenLBracket); err != nil {
		return nil, fmt.Errorf("expected '[' after IN: %w", err)
	}

	// Parse list of values
	var values []interface{}

	// Handle empty list
	if p.current().Type == TokenRBracket {
		p.advance()
		return &InExpr{Field: fieldName, Not: isNotIn, Values: values}, nil
	}

	for {
		// Parse a value (string, number, or identifier like CURRENT_USER)
		val, err := p.parseListValue()
		if err != nil {
			return nil, err
		}
		values = append(values, val)

		// Check for comma (more values) or closing bracket (done)
		tok := p.current()
		if tok.Type == TokenRBracket {
			p.advance()
			break
		}
		if tok.Type == TokenComma {
			p.advance()
			continue
		}
		return nil, fmt.Errorf("expected ',' or ']' in list, got: %s", tok.Value)
	}

	return &InExpr{Field: fieldName, Not: isNotIn, Values: values}, nil
}

// parseListValue parses a single value in a list (string, number, or identifier)
// Does not allow durations or time expressions
func (p *filterParser) parseListValue() (interface{}, error) {
	val, _, err := p.parseValueGeneric(false)
	return val, err
}
