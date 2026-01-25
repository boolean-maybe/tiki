package filter

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/task"
)

func TestParseFilterWithIn(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		{
			name:   "tags IN with match",
			expr:   "tags IN ['ui', 'charts']",
			task:   &task.Task{Tags: []string{"ui", "backend"}},
			expect: true,
		},
		{
			name:   "tags IN with no match",
			expr:   "tags IN ['frontend', 'api']",
			task:   &task.Task{Tags: []string{"ui", "backend"}},
			expect: false,
		},
		{
			name:   "tags IN with single value match",
			expr:   "tags IN ['ui']",
			task:   &task.Task{Tags: []string{"ui", "backend"}},
			expect: true,
		},
		{
			name:   "tags IN empty list",
			expr:   "tags IN []",
			task:   &task.Task{Tags: []string{"ui"}},
			expect: false,
		},
		{
			name:   "status IN with match",
			expr:   "status IN ['ready', 'in_progress']",
			task:   &task.Task{Status: task.StatusReady},
			expect: true,
		},
		{
			name:   "status IN with no match",
			expr:   "status IN ['done', 'cancelled']",
			task:   &task.Task{Status: task.StatusReady},
			expect: false,
		},
		{
			name:   "status NOT IN with match",
			expr:   "status NOT IN ['done', 'cancelled']",
			task:   &task.Task{Status: task.StatusReady},
			expect: true,
		},
		{
			name:   "status NOT IN with no match",
			expr:   "status NOT IN ['ready', 'in_progress']",
			task:   &task.Task{Status: task.StatusReady},
			expect: false,
		},
		{
			name:   "type IN with match",
			expr:   "type IN ['story', 'bug']",
			task:   &task.Task{Type: task.TypeStory},
			expect: true,
		},
		{
			name:   "priority IN with integers",
			expr:   "priority IN [1, 2, 3]",
			task:   &task.Task{Priority: 2},
			expect: true,
		},
		{
			name:   "priority IN with no match",
			expr:   "priority IN [1, 3, 5]",
			task:   &task.Task{Priority: 2},
			expect: false,
		},
		{
			name:   "combined with AND",
			expr:   "tags IN ['ui', 'charts'] AND status = 'ready'",
			task:   &task.Task{Tags: []string{"ui"}, Status: task.StatusReady},
			expect: true,
		},
		{
			name:   "combined with AND, no match",
			expr:   "tags IN ['ui', 'charts'] AND status = 'done'",
			task:   &task.Task{Tags: []string{"ui"}, Status: task.StatusReady},
			expect: false,
		},
		{
			name:   "combined with OR",
			expr:   "tags IN ['ui'] OR tags IN ['backend']",
			task:   &task.Task{Tags: []string{"backend"}},
			expect: true,
		},
		{
			name:   "case insensitive tags",
			expr:   "tags IN ['UI', 'Charts']",
			task:   &task.Task{Tags: []string{"ui", "charts"}},
			expect: true,
		},
		{
			name:   "case insensitive status",
			expr:   "status IN ['READY', 'IN_PROGRESS']",
			task:   &task.Task{Status: task.StatusReady},
			expect: true,
		},
		{
			name:   "NOT with IN expression",
			expr:   "NOT (tags IN ['deprecated', 'archived'])",
			task:   &task.Task{Tags: []string{"ui", "active"}},
			expect: true,
		},
		{
			name:   "complex expression",
			expr:   "(tags IN ['ui', 'frontend'] OR type = 'bug') AND status NOT IN ['done']",
			task:   &task.Task{Tags: []string{"ui"}, Type: task.TypeStory, Status: task.StatusReady},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.expr)
			if err != nil {
				t.Fatalf("ParseFilter failed: %v", err)
			}

			result := filter.Evaluate(tt.task, time.Now(), "testuser")
			if result != tt.expect {
				t.Errorf("Expected %v, got %v for expression: %s", tt.expect, result, tt.expr)
			}
		})
	}
}

func TestTokenizeWithIn(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []TokenType
	}{
		{
			name:  "simple IN expression",
			input: "tags IN ['ui']",
			expected: []TokenType{TokenIdent, TokenIn, TokenLBracket,
				TokenString, TokenRBracket, TokenEOF},
		},
		{
			name:  "NOT IN expression",
			input: "status NOT IN ['done']",
			expected: []TokenType{TokenIdent, TokenNotIn, TokenLBracket,
				TokenString, TokenRBracket, TokenEOF},
		},
		{
			name:  "multiple values with commas",
			input: "type IN ['bug', 'story', 'epic']",
			expected: []TokenType{TokenIdent, TokenIn, TokenLBracket,
				TokenString, TokenComma, TokenString,
				TokenComma, TokenString, TokenRBracket, TokenEOF},
		},
		{
			name:  "IN with numbers",
			input: "priority IN [1, 2, 3]",
			expected: []TokenType{TokenIdent, TokenIn, TokenLBracket,
				TokenNumber, TokenComma, TokenNumber,
				TokenComma, TokenNumber, TokenRBracket, TokenEOF},
		},
		{
			name:     "empty list",
			input:    "tags IN []",
			expected: []TokenType{TokenIdent, TokenIn, TokenLBracket, TokenRBracket, TokenEOF},
		},
		{
			name:  "IN with AND",
			input: "tags IN ['ui'] AND status = 'ready'",
			expected: []TokenType{TokenIdent, TokenIn, TokenLBracket, TokenString, TokenRBracket,
				TokenAnd, TokenIdent, TokenOperator, TokenString, TokenEOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := Tokenize(tt.input)
			if err != nil {
				t.Fatalf("tokenize failed: %v", err)
			}

			if len(tokens) != len(tt.expected) {
				t.Fatalf("Expected %d tokens, got %d", len(tt.expected), len(tokens))
			}

			for i, tok := range tokens {
				if tok.Type != tt.expected[i] {
					t.Errorf("Token %d: expected type %d, got %d (value: %s)",
						i, tt.expected[i], tok.Type, tok.Value)
				}
			}
		})
	}
}

func TestParseFilterErrors(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		errMsg string
	}{
		{
			name:   "missing opening bracket",
			expr:   "tags IN 'ui', 'charts']",
			errMsg: "expected '['",
		},
		{
			name:   "missing closing bracket",
			expr:   "tags IN ['ui', 'charts'",
			errMsg: "expected ','",
		},
		{
			name:   "missing comma",
			expr:   "tags IN ['ui' 'charts']",
			errMsg: "expected ','",
		},
		{
			name:   "invalid value in list",
			expr:   "tags IN ['ui', =]",
			errMsg: "unexpected token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFilter(tt.expr)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			// Could check error message contains expected substring if needed
		})
	}
}

func TestInExprWithCurrentUser(t *testing.T) {
	tests := []struct {
		name        string
		expr        string
		task        *task.Task
		currentUser string
		expect      bool
	}{
		{
			name:        "assignee IN with CURRENT_USER match",
			expr:        "assignee IN ['alice', CURRENT_USER, 'bob']",
			task:        &task.Task{Assignee: "testuser"},
			currentUser: "testuser",
			expect:      true,
		},
		{
			name:        "assignee IN with CURRENT_USER no match",
			expr:        "assignee IN ['alice', CURRENT_USER, 'bob']",
			task:        &task.Task{Assignee: "charlie"},
			currentUser: "testuser",
			expect:      false,
		},
		{
			name:        "assignee IN with only CURRENT_USER",
			expr:        "assignee IN [CURRENT_USER]",
			task:        &task.Task{Assignee: "testuser"},
			currentUser: "testuser",
			expect:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.expr)
			if err != nil {
				t.Fatalf("ParseFilter failed: %v", err)
			}

			result := filter.Evaluate(tt.task, time.Now(), tt.currentUser)
			if result != tt.expect {
				t.Errorf("Expected %v, got %v", tt.expect, result)
			}
		})
	}
}

func TestInExprBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		{
			name:   "old style tag comparison",
			expr:   "tag = 'ui'",
			task:   &task.Task{Tags: []string{"ui", "frontend"}},
			expect: true,
		},
		{
			name:   "old style OR chain",
			expr:   "(tag = 'ui' OR tag = 'charts' OR tag = 'viz')",
			task:   &task.Task{Tags: []string{"charts"}},
			expect: true,
		},
		{
			name:   "status comparison",
			expr:   "status = 'ready'",
			task:   &task.Task{Status: task.StatusReady},
			expect: true,
		},
		{
			name:   "complex old style expression",
			expr:   "status = 'ready' AND (tag = 'ui' OR tag = 'backend')",
			task:   &task.Task{Status: task.StatusReady, Tags: []string{"ui"}},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.expr)
			if err != nil {
				t.Fatalf("ParseFilter failed: %v", err)
			}

			result := filter.Evaluate(tt.task, time.Now(), "testuser")
			if result != tt.expect {
				t.Errorf("Expected %v, got %v", tt.expect, result)
			}
		})
	}
}
