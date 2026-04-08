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
	stmtParser        *participle.Parser[statementGrammar]
	triggerParser     *participle.Parser[triggerGrammar]
	timeTriggerParser *participle.Parser[timeTriggerGrammar]
	ruleParser        *participle.Parser[ruleGrammar]
	schema            Schema
	qualifiers        qualifierPolicy // set before each validation pass
	requireQualifiers bool            // when true, bare FieldRef is a parse error (trigger where-guards)
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
		stmtParser:        participle.MustBuild[statementGrammar](opts...),
		triggerParser:     participle.MustBuild[triggerGrammar](opts...),
		timeTriggerParser: participle.MustBuild[timeTriggerGrammar](opts...),
		ruleParser:        participle.MustBuild[ruleGrammar](opts...),
		schema:            schema,
	}
}

// ParseStatement parses a CRUD statement and performs syntax, structural,
// built-in signature/arity, and type validation.
// Semantic runtime validation is a separate step.
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

// ParseTrigger parses a reactive trigger rule and performs syntax, structural,
// built-in signature/arity, and type validation.
// Semantic runtime validation is a separate step.
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

// ParseTimeTrigger parses a periodic time trigger and performs syntax,
// structural, built-in signature/arity, and type validation.
// Semantic runtime validation is a separate step.
func (p *Parser) ParseTimeTrigger(input string) (*TimeTrigger, error) {
	g, err := p.timeTriggerParser.ParseString("", input)
	if err != nil {
		return nil, err
	}
	tt, err := lowerTimeTrigger(g)
	if err != nil {
		return nil, err
	}
	if err := p.validateTimeTrigger(tt); err != nil {
		return nil, err
	}
	return tt, nil
}

// ParseRule parses a trigger definition that is either an event trigger
// (before/after) or a time trigger (every), and performs syntax, structural,
// built-in signature/arity, and type validation.
// Semantic runtime validation is a separate step after branching.
func (p *Parser) ParseRule(input string) (*Rule, error) {
	g, err := p.ruleParser.ParseString("", input)
	if err != nil {
		return nil, err
	}
	rule, err := lowerRule(g)
	if err != nil {
		return nil, err
	}
	if err := p.validateRule(rule); err != nil {
		return nil, err
	}
	return rule, nil
}
