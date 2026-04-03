package ruki

import (
	"strings"
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
)

// tokenize lexes input and returns non-whitespace, non-EOF tokens.
func tokenize(t *testing.T, input string) []lexer.Token {
	t.Helper()
	lex, err := rukiLexer.Lex("", strings.NewReader(input))
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	all, err := lexer.ConsumeAll(lex)
	if err != nil {
		t.Fatalf("consume error: %v", err)
	}
	symbols := rukiLexer.Symbols()
	wsType := symbols["Whitespace"]
	var result []lexer.Token
	for _, tok := range all {
		if tok.Type != wsType && tok.Type != lexer.EOF {
			result = append(result, tok)
		}
	}
	return result
}

func TestTokenizeKeywords(t *testing.T) {
	keywordType := rukiLexer.Symbols()["Keyword"]

	for _, kw := range reservedKeywords {
		t.Run(kw, func(t *testing.T) {
			tokens := tokenize(t, kw)
			if len(tokens) != 1 {
				t.Fatalf("expected 1 token, got %d: %v", len(tokens), tokens)
			}
			if tokens[0].Type != keywordType {
				t.Errorf("expected Keyword token for %q, got type %d", kw, tokens[0].Type)
			}
			if tokens[0].Value != kw {
				t.Errorf("expected value %q, got %q", kw, tokens[0].Value)
			}
		})
	}
}

func TestTokenizeWordBoundary(t *testing.T) {
	identType := rukiLexer.Symbols()["Ident"]

	cases := []struct {
		input     string
		rationale string
	}{
		{"selectAll", "keyword 'select' as prefix"},
		{"nowhere", "keyword 'where' as suffix"},
		{"ordering", "keyword 'order' as prefix"},
		{"inline", "keyword 'in' as prefix"},
		{"orchestra", "keyword 'or' as prefix"},
		{"newsletter", "keyword 'new' as prefix"},
		{"dataset", "keyword 'set' as suffix"},
		{"island", "keyword 'is' as prefix"},
		{"notify", "keyword 'not' as prefix"},
		{"bygone", "keyword 'by' as prefix"},
		{"descendant", "keyword 'desc' as prefix"},
		{"ascend", "keyword 'asc' as prefix"},
		{"inbound", "keyword 'in' as prefix"},
		{"android", "keyword 'and' as prefix"},
		{"emptySet", "keyword 'empty' as prefix"},
		{"denying", "keyword 'deny' as prefix"},
		{"running", "keyword 'run' as prefix"},
		{"anyone", "keyword 'any' as prefix"},
		{"overall", "keyword 'all' as suffix"},
		{"oldest", "keyword 'old' as prefix"},
		{"afterword", "keyword 'after' as prefix"},
		{"beforehand", "keyword 'before' as prefix"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			tokens := tokenize(t, tc.input)
			if len(tokens) != 1 {
				t.Fatalf("expected 1 Ident token for %q (%s), got %d tokens: %v",
					tc.input, tc.rationale, len(tokens), tokens)
			}
			if tokens[0].Type != identType {
				t.Errorf("expected Ident for %q (%s), got type %d",
					tc.input, tc.rationale, tokens[0].Type)
			}
			if tokens[0].Value != tc.input {
				t.Errorf("expected value %q, got %q", tc.input, tokens[0].Value)
			}
		})
	}
}

func TestKeywordInIdentPosition_ParseError(t *testing.T) {
	p := newTestParser()

	cases := []struct {
		name  string
		input string
	}{
		// statement keywords as field refs
		{"select as field ref", `select where select = "done"`},
		{"where as field ref", `select where where = "done"`},
		{"delete as field ref", `select where delete = "done"`},
		{"create as field ref", `select where create = "done"`},
		{"update as field ref", `select where update = "done"`},

		// clause keywords as field refs
		{"set as assignment target", `create set="value"`},
		{"order as order-by field", `select order by order`},
		{"where as order-by field", `select order by where desc`},
		{"by as field ref", `select where by = "x"`},

		// logical keywords as field refs
		{"and as field ref", `select where and = "x"`},
		{"or as field ref", `select where or = "x"`},
		{"not as field ref", `select where not = "x"`},

		// test keywords as field refs
		{"is as field ref", `select where is = "x"`},
		{"in as field ref", `select where in = "x"`},
		// note: 'empty' is intentionally omitted — it's a valid expression literal
		// (emptyExpr in grammar), so `select where empty = "x"` is valid grammar
		// that fails at validation, not parsing.

		// quantifier keywords as field refs
		{"any as field ref", `select where any = "x"`},
		{"all as field ref", `select where all = "x"`},

		// sort keywords as field refs
		{"asc as field ref", `select where asc = "x"`},
		{"desc as field ref", `select where desc = "x"`},

		// trigger keywords as field refs
		{"before as field ref", `select where before = "x"`},
		{"after as field ref", `select where after = "x"`},
		{"deny as field ref", `select where deny = "x"`},
		{"run as field ref", `select where run = "x"`},

		// qualifier keywords as field refs
		{"old as field ref", `select where old = "x"`},
		{"new as field ref", `select where new = "x"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.ParseStatement(tc.input)
			if err == nil {
				t.Errorf("expected parse error for %q, got nil", tc.input)
			}
		})
	}
}
