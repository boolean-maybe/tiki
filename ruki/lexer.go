package ruki

import "github.com/alecthomas/participle/v2/lexer"

// rukiLexer defines the token rules for the ruki DSL.
// rule ordering is critical: longer/more-specific patterns first.
var rukiLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `--[^\n]*`},
	{Name: "Whitespace", Pattern: `\s+`},
	{Name: "Duration", Pattern: `\d+(?:sec|min|hour|day|week|month|year)s?`},
	{Name: "Date", Pattern: `\d{4}-\d{2}-\d{2}`},
	{Name: "Int", Pattern: `\d+`},
	{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`},
	{Name: "CompareOp", Pattern: `!=|<=|>=|[=<>]`},
	{Name: "Plus", Pattern: `\+`},
	{Name: "Minus", Pattern: `-`},
	{Name: "Dot", Pattern: `\.`},
	{Name: "LParen", Pattern: `\(`},
	{Name: "RParen", Pattern: `\)`},
	{Name: "LBracket", Pattern: `\[`},
	{Name: "RBracket", Pattern: `\]`},
	{Name: "Comma", Pattern: `,`},
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
})
