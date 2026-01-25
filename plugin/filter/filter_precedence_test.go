package filter

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/task"
)

// TestOperatorPrecedence tests that operators follow correct precedence: NOT > AND > OR
func TestOperatorPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		// NOT has highest precedence
		{
			name:   "NOT before AND",
			expr:   "NOT status = 'done' AND type = 'bug'",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug},
			expect: true, // (NOT (status = 'done')) AND (type = 'bug') = true AND true = true
		},
		{
			name:   "NOT before AND - false case",
			expr:   "NOT status = 'done' AND type = 'bug'",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory},
			expect: false, // (NOT (status = 'done')) AND (type = 'bug') = true AND false = false
		},
		{
			name:   "NOT before AND - with done status",
			expr:   "NOT status = 'done' AND type = 'bug'",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeBug},
			expect: false, // (NOT (status = 'done')) AND (type = 'bug') = false AND true = false
		},

		// AND before OR - left side
		{
			name:   "AND before OR - left match",
			expr:   "status = 'ready' OR status = 'in_progress' AND type = 'bug'",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory},
			expect: true, // status = 'ready' OR (status = 'in_progress' AND type = 'bug') = true OR false = true
		},
		{
			name:   "AND before OR - right match",
			expr:   "status = 'ready' OR status = 'in_progress' AND type = 'bug'",
			task:   &task.Task{Status: task.StatusInProgress, Type: task.TypeBug},
			expect: true, // status = 'ready' OR (status = 'in_progress' AND type = 'bug') = false OR true = true
		},
		{
			name:   "AND before OR - no match",
			expr:   "status = 'ready' OR status = 'in_progress' AND type = 'bug'",
			task:   &task.Task{Status: task.StatusInProgress, Type: task.TypeStory},
			expect: false, // status = 'ready' OR (status = 'in_progress' AND type = 'bug') = false OR false = false
		},

		// AND before OR - right side
		{
			name:   "AND before OR - right side left match",
			expr:   "status = 'in_progress' AND type = 'bug' OR status = 'ready'",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory},
			expect: true, // (status = 'in_progress' AND type = 'bug') OR status = 'ready' = false OR true = true
		},
		{
			name:   "AND before OR - right side right match",
			expr:   "status = 'in_progress' AND type = 'bug' OR status = 'ready'",
			task:   &task.Task{Status: task.StatusInProgress, Type: task.TypeBug},
			expect: true, // (status = 'in_progress' AND type = 'bug') OR status = 'ready' = true OR false = true
		},

		// Parentheses override precedence
		{
			name:   "parentheses override AND/OR precedence - no match",
			expr:   "(status = 'ready' OR status = 'in_progress') AND type = 'bug'",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory},
			expect: false, // (status = 'ready' OR status = 'in_progress') AND type = 'bug' = true AND false = false
		},
		{
			name:   "parentheses override AND/OR precedence - match",
			expr:   "(status = 'ready' OR status = 'in_progress') AND type = 'bug'",
			task:   &task.Task{Status: task.StatusInProgress, Type: task.TypeBug},
			expect: true, // (status = 'ready' OR status = 'in_progress') AND type = 'bug' = true AND true = true
		},

		// NOT with OR
		{
			name:   "NOT before OR",
			expr:   "NOT status = 'done' OR type = 'bug'",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeStory},
			expect: false, // (NOT (status = 'done')) OR (type = 'bug') = false OR false = false
		},
		{
			name:   "NOT before OR - match on NOT",
			expr:   "NOT status = 'done' OR type = 'bug'",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory},
			expect: true, // (NOT (status = 'done')) OR (type = 'bug') = true OR false = true
		},
		{
			name:   "NOT before OR - match on type",
			expr:   "NOT status = 'done' OR type = 'bug'",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeBug},
			expect: true, // (NOT (status = 'done')) OR (type = 'bug') = false OR true = true
		},

		// Complex precedence: NOT > AND > OR
		{
			name:   "NOT > AND > OR - all operators",
			expr:   "NOT status = 'done' AND type = 'bug' OR priority > 3",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug, Priority: 5},
			expect: true, // ((NOT (status = 'done')) AND type = 'bug') OR priority > 3 = true OR true = true
		},
		{
			name:   "NOT > AND > OR - match on OR only",
			expr:   "NOT status = 'done' AND type = 'bug' OR priority > 3",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory, Priority: 5},
			expect: true, // ((NOT (status = 'done')) AND type = 'bug') OR priority > 3 = false OR true = true
		},
		{
			name:   "NOT > AND > OR - match on AND only",
			expr:   "NOT status = 'done' AND type = 'bug' OR priority > 3",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug, Priority: 2},
			expect: true, // ((NOT (status = 'done')) AND type = 'bug') OR priority > 3 = true OR false = true
		},
		{
			name:   "NOT > AND > OR - no match",
			expr:   "NOT status = 'done' AND type = 'bug' OR priority > 3",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeStory, Priority: 1},
			expect: false, // ((NOT (status = 'done')) AND type = 'bug') OR priority > 3 = false OR false = false
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
				t.Errorf("Expected %v, got %v for expression: %s\nTask: status=%s, type=%s, priority=%d",
					tt.expect, result, tt.expr, tt.task.Status, tt.task.Type, tt.task.Priority)
			}
		})
	}
}

// TestNestedParentheses tests deeply nested parentheses expressions
func TestNestedParentheses(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		// Double nesting
		{
			name:   "double nested parentheses - match",
			expr:   "((status = 'ready' OR status = 'in_progress') AND type = 'bug')",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug},
			expect: true,
		},
		{
			name:   "double nested parentheses - no match on type",
			expr:   "((status = 'ready' OR status = 'in_progress') AND type = 'bug')",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory},
			expect: false,
		},
		{
			name:   "double nested parentheses - no match on status",
			expr:   "((status = 'ready' OR status = 'in_progress') AND type = 'bug')",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeBug},
			expect: false,
		},

		// Triple nesting with NOT
		{
			name:   "triple nested with NOT - match",
			expr:   "NOT (((status = 'done' OR status = 'cancelled') AND priority < 3))",
			task:   &task.Task{Status: task.StatusReady, Priority: 2},
			expect: true, // NOT ((false OR false) AND true) = NOT (false AND true) = NOT false = true
		},
		{
			name:   "triple nested with NOT - no match",
			expr:   "NOT (((status = 'done' OR status = 'cancelled') AND priority < 3))",
			task:   &task.Task{Status: task.StatusDone, Priority: 2},
			expect: false, // NOT ((true OR false) AND true) = NOT (true AND true) = NOT true = false
		},
		{
			name:   "triple nested with NOT - no match on priority",
			expr:   "NOT (((status = 'done' OR status = 'cancelled') AND priority < 3))",
			task:   &task.Task{Status: task.StatusDone, Priority: 5},
			expect: true, // NOT ((true OR false) AND false) = NOT (true AND false) = NOT false = true
		},

		// Mixed nesting depth
		{
			name:   "mixed nesting depth - OR at end",
			expr:   "(status = 'ready' AND (type = 'bug' OR type = 'story')) OR status = 'in_progress'",
			task:   &task.Task{Status: task.StatusInProgress, Type: task.TypeBug},
			expect: true, // (false AND (true OR false)) OR true = false OR true = true
		},
		{
			name:   "mixed nesting depth - match on nested OR",
			expr:   "(status = 'ready' AND (type = 'bug' OR type = 'story')) OR status = 'in_progress'",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug},
			expect: true, // (true AND (true OR false)) OR false = true OR false = true
		},
		{
			name:   "mixed nesting depth - match on final OR",
			expr:   "(status = 'ready' AND (type = 'bug' OR type = 'story')) OR status = 'in_progress'",
			task:   &task.Task{Status: task.StatusInProgress, Type: task.TypeBug},
			expect: true, // (false AND (true OR false)) OR true = false OR true = true
		},

		// Complex nested with multiple operations
		{
			name:   "complex nested - all conditions",
			expr:   "((status = 'ready' OR status = 'in_progress') AND (type = 'bug' OR priority > 3))",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory, Priority: 5},
			expect: true, // (true OR false) AND (false OR true) = true AND true = true
		},
		{
			name:   "complex nested - left fails",
			expr:   "((status = 'ready' OR status = 'in_progress') AND (type = 'bug' OR priority > 3))",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeStory, Priority: 5},
			expect: false, // (false OR false) AND (false OR true) = false AND true = false
		},
		{
			name:   "complex nested - right fails",
			expr:   "((status = 'ready' OR status = 'in_progress') AND (type = 'bug' OR priority > 3))",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory, Priority: 2},
			expect: false, // (true OR false) AND (false OR false) = true AND false = false
		},

		// Nested with NOT at different levels
		{
			name:   "NOT outside nested expression",
			expr:   "NOT ((status = 'done' AND type = 'bug') OR priority < 2)",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeBug, Priority: 3},
			expect: false, // NOT ((true AND true) OR false) = NOT (true OR false) = NOT true = false
		},
		{
			name:   "NOT inside nested expression",
			expr:   "(NOT status = 'done' AND type = 'bug') OR priority > 5",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug, Priority: 3},
			expect: true, // ((NOT false) AND true) OR false = (true AND true) OR false = true
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

// TestComplexBooleanChains tests multiple operators chained together
func TestComplexBooleanChains(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		// Triple AND chain
		{
			name:   "triple AND chain - all match",
			expr:   "status = 'ready' AND type = 'bug' AND priority > 3",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug, Priority: 5},
			expect: true,
		},
		{
			name:   "triple AND chain - first fails",
			expr:   "status = 'ready' AND type = 'bug' AND priority > 3",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeBug, Priority: 5},
			expect: false,
		},
		{
			name:   "triple AND chain - middle fails",
			expr:   "status = 'ready' AND type = 'bug' AND priority > 3",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory, Priority: 5},
			expect: false,
		},
		{
			name:   "triple AND chain - last fails",
			expr:   "status = 'ready' AND type = 'bug' AND priority > 3",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug, Priority: 2},
			expect: false,
		},

		// Triple OR chain
		{
			name:   "triple OR chain - first matches",
			expr:   "status = 'ready' OR status = 'in_progress' OR status = 'in_progress'",
			task:   &task.Task{Status: task.StatusReady},
			expect: true,
		},
		{
			name:   "triple OR chain - middle matches",
			expr:   "status = 'ready' OR status = 'in_progress' OR status = 'in_progress'",
			task:   &task.Task{Status: task.StatusInProgress},
			expect: true,
		},
		{
			name:   "triple OR chain - last matches",
			expr:   "status = 'ready' OR status = 'in_progress' OR status = 'in_progress'",
			task:   &task.Task{Status: task.StatusInProgress},
			expect: true,
		},
		{
			name:   "triple OR chain - none match",
			expr:   "status = 'ready' OR status = 'in_progress' OR status = 'in_progress'",
			task:   &task.Task{Status: task.StatusDone},
			expect: false,
		},

		// Mixed chain without parentheses - tests precedence
		{
			name:   "mixed chain A OR B AND C - match on A",
			expr:   "status = 'ready' OR type = 'bug' AND priority > 3",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory, Priority: 2},
			expect: true, // status = 'ready' OR (type = 'bug' AND priority > 3) = true OR false = true
		},
		{
			name:   "mixed chain A OR B AND C - match on B AND C",
			expr:   "status = 'ready' OR type = 'bug' AND priority > 3",
			task:   &task.Task{Status: task.StatusInProgress, Type: task.TypeBug, Priority: 5},
			expect: true, // status = 'ready' OR (type = 'bug' AND priority > 3) = false OR true = true
		},
		{
			name:   "mixed chain A OR B AND C - no match",
			expr:   "status = 'ready' OR type = 'bug' AND priority > 3",
			task:   &task.Task{Status: task.StatusInProgress, Type: task.TypeStory, Priority: 2},
			expect: false, // status = 'ready' OR (type = 'bug' AND priority > 3) = false OR false = false
		},

		// Longer chains
		{
			name:   "four operator chain - AND heavy",
			expr:   "status = 'ready' AND type = 'bug' AND priority > 3 AND points < 10",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug, Priority: 5, Points: 8},
			expect: true,
		},
		{
			name:   "four operator chain - mixed AND/OR",
			expr:   "status = 'ready' AND type = 'bug' OR status = 'in_progress' AND priority > 3",
			task:   &task.Task{Status: task.StatusInProgress, Type: task.TypeStory, Priority: 5},
			expect: true, // (status = 'ready' AND type = 'bug') OR (status = 'in_progress' AND priority > 3) = false OR true = true
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
