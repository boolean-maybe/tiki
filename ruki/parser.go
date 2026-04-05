package ruki

import (
	"github.com/alecthomas/participle/v2"
)

// Schema provides the canonical field catalog and normalization functions
// that the parser uses for validation. Production code adapts this from
// workflow.Fields(), workflow.StatusRegistry, and workflow.TypeRegistry.
type Schema interface {
	// Field returns the field spec for a given field name.
	Field(name string) (FieldSpec, bool)
	// NormalizeStatus validates and normalizes a raw status string.
	// returns the canonical key and true, or ("", false) for unknown values.
	NormalizeStatus(raw string) (string, bool)
	// NormalizeType validates and normalizes a raw type string.
	// returns the canonical key and true, or ("", false) for unknown values.
	NormalizeType(raw string) (string, bool)
}

// ValueType identifies the semantic type of a field in the DSL.
type ValueType int

const (
	ValueString     ValueType = iota
	ValueInt                  // priority, points
	ValueDate                 // due
	ValueTimestamp            // createdAt, updatedAt
	ValueDuration             // duration literals
	ValueBool                 // boolean result type
	ValueID                   // task identifier
	ValueRef                  // reference to another task
	ValueRecurrence           // recurrence pattern
	ValueListString           // tags
	ValueListRef              // dependsOn
	ValueStatus               // status enum
	ValueTaskType             // type enum
)

// FieldSpec describes a single task field for the parser.
type FieldSpec struct {
	Name string
	Type ValueType
}

// Parser parses ruki DSL statements and triggers.
type Parser struct {
	stmtParser    *participle.Parser[statementGrammar]
	triggerParser *participle.Parser[triggerGrammar]
	schema        Schema
	qualifiers    qualifierPolicy // set before each validation pass
}

// NewParser constructs a Parser with the given schema for validation.
// panics if the grammar is invalid (programming error, not user error).
func NewParser(schema Schema) *Parser {
	opts := []participle.Option{
		participle.Lexer(rukiLexer),
		participle.Elide("Comment", "Whitespace"),
		participle.UseLookahead(3),
	}
	return &Parser{
		stmtParser:    participle.MustBuild[statementGrammar](opts...),
		triggerParser: participle.MustBuild[triggerGrammar](opts...),
		schema:        schema,
	}
}

// ParseStatement parses a CRUD statement and returns a validated AST.
func (p *Parser) ParseStatement(input string) (*Statement, error) {
	g, err := p.stmtParser.ParseString("", input)
	if err != nil {
		return nil, err
	}
	stmt, err := lowerStatement(g)
	if err != nil {
		return nil, err
	}
	p.qualifiers = noQualifiers
	if err := p.validateStatement(stmt); err != nil {
		return nil, err
	}
	return stmt, nil
}

// ParseTrigger parses a reactive trigger rule and returns a validated AST.
func (p *Parser) ParseTrigger(input string) (*Trigger, error) {
	g, err := p.triggerParser.ParseString("", input)
	if err != nil {
		return nil, err
	}
	trig, err := lowerTrigger(g)
	if err != nil {
		return nil, err
	}
	p.qualifiers = triggerQualifiers(trig.Event)
	if err := p.validateTrigger(trig); err != nil {
		return nil, err
	}
	return trig, nil
}
