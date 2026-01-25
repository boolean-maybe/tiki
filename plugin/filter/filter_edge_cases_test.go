package filter

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/task"
)

// TestDoubleNegation tests that NOT NOT works correctly
func TestDoubleNegation(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		{
			name:   "double negation - should match",
			expr:   "NOT NOT status = 'ready'",
			task:   &task.Task{Status: task.StatusReady},
			expect: true, // NOT NOT true = NOT false = true
		},
		{
			name:   "double negation - should not match",
			expr:   "NOT NOT status = 'ready'",
			task:   &task.Task{Status: task.StatusDone},
			expect: false, // NOT NOT false = NOT true = false
		},
		{
			name:   "double negation with parentheses",
			expr:   "NOT (NOT (status = 'ready'))",
			task:   &task.Task{Status: task.StatusReady},
			expect: true,
		},
		{
			name:   "triple negation - odd number",
			expr:   "NOT NOT NOT status = 'done'",
			task:   &task.Task{Status: task.StatusReady},
			expect: true, // NOT NOT NOT false = NOT NOT true = NOT false = true
		},
		{
			name:   "triple negation - cancels to NOT",
			expr:   "NOT NOT NOT status = 'done'",
			task:   &task.Task{Status: task.StatusDone},
			expect: false, // NOT NOT NOT true = NOT NOT false = NOT true = false
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

// TestEmptyFilter tests handling of empty filter expressions
func TestEmptyFilter(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		expect bool // empty filter should match all tasks
	}{
		{
			name:   "empty string",
			expr:   "",
			expect: true, // nil filter means no filtering
		},
		{
			name:   "whitespace only",
			expr:   "   ",
			expect: true, // trimmed to empty, should be nil filter
		},
	}

	task := &task.Task{Status: task.StatusReady}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.expr)
			if err != nil {
				t.Fatalf("ParseFilter failed: %v", err)
			}

			// Empty filter returns nil
			if filter == nil {
				// Nil filter means match all - this is correct behavior
				return
			}

			result := filter.Evaluate(task, time.Now(), "testuser")
			if result != tt.expect {
				t.Errorf("Expected %v, got %v for expression: %q", tt.expect, result, tt.expr)
			}
		})
	}
}

// TestComplexNOTExpressions tests NOT with various complex expressions
func TestComplexNOTExpressions(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		{
			name:   "NOT with AND - both conditions true",
			expr:   "NOT (status = 'ready' AND type = 'bug')",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug},
			expect: false, // NOT (true AND true) = NOT true = false
		},
		{
			name:   "NOT with AND - one condition false",
			expr:   "NOT (status = 'ready' AND type = 'bug')",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory},
			expect: true, // NOT (true AND false) = NOT false = true
		},
		{
			name:   "NOT with OR - both conditions true",
			expr:   "NOT (status = 'ready' OR type = 'bug')",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug},
			expect: false, // NOT (true OR true) = NOT true = false
		},
		{
			name:   "NOT with OR - one condition true",
			expr:   "NOT (status = 'ready' OR type = 'bug')",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory},
			expect: false, // NOT (true OR false) = NOT true = false
		},
		{
			name:   "NOT with OR - both conditions false",
			expr:   "NOT (status = 'ready' OR type = 'bug')",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeStory},
			expect: true, // NOT (false OR false) = NOT false = true
		},
		{
			name:   "NOT with complex mixed expression",
			expr:   "NOT (status = 'ready' AND type = 'bug' OR status = 'in_progress')",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug},
			expect: false, // NOT ((true AND true) OR false) = NOT (true OR false) = NOT true = false
		},
		{
			name:   "NOT with complex mixed expression - alternative match",
			expr:   "NOT (status = 'ready' AND type = 'bug' OR status = 'in_progress')",
			task:   &task.Task{Status: task.StatusInProgress, Type: task.TypeStory},
			expect: false, // NOT ((false AND false) OR true) = NOT (false OR true) = NOT true = false
		},
		{
			name:   "NOT with complex mixed expression - no match",
			expr:   "NOT (status = 'ready' AND type = 'bug' OR status = 'in_progress')",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeStory},
			expect: true, // NOT ((false AND false) OR false) = NOT false = true
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

// TestAllOperatorsCombined tests expressions using all available operators
func TestAllOperatorsCombined(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		{
			name:   "all operators - all conditions match",
			expr:   "NOT status = 'done' AND (type IN ['bug', 'story'] OR priority > 3) AND tags NOT IN ['deprecated']",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug, Priority: 5, Tags: []string{"active"}},
			expect: true,
		},
		{
			name:   "all operators - NOT fails",
			expr:   "NOT status = 'done' AND (type IN ['bug', 'story'] OR priority > 3) AND tags NOT IN ['deprecated']",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeBug, Priority: 5, Tags: []string{"active"}},
			expect: false,
		},
		{
			name:   "all operators - IN fails",
			expr:   "NOT status = 'done' AND (type IN ['bug', 'story'] OR priority > 3) AND tags NOT IN ['deprecated']",
			task:   &task.Task{Status: task.StatusReady, Type: "epic", Priority: 2, Tags: []string{"active"}},
			expect: false,
		},
		{
			name:   "all operators - NOT IN fails",
			expr:   "NOT status = 'done' AND (type IN ['bug', 'story'] OR priority > 3) AND tags NOT IN ['deprecated']",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug, Priority: 5, Tags: []string{"deprecated"}},
			expect: false,
		},
		{
			name:   "complex with comparisons and IN",
			expr:   "(priority >= 3 AND priority <= 5) OR (status IN ['ready', 'in_progress'] AND type = 'bug')",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug, Priority: 2},
			expect: true, // Second part matches
		},
		{
			name:   "complex with multiple NOT",
			expr:   "NOT status = 'done' AND NOT type = 'epic' AND NOT tags IN ['deprecated', 'archived']",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeBug, Tags: []string{"active"}},
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

// TestVeryLongExpressionChains tests that parser handles long chains correctly
func TestVeryLongExpressionChains(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		{
			name: "five AND chain",
			expr: "status = 'ready' AND type = 'bug' AND priority > 2 AND priority < 6 AND points > 0",
			task: &task.Task{
				Status:   task.StatusReady,
				Type:     task.TypeBug,
				Priority: 4,
				Points:   3,
			},
			expect: true,
		},
		{
			name:   "five OR chain - last matches",
			expr:   "status = 'done' OR status = 'cancelled' OR status = 'in_progress' OR status = 'review' OR status = 'ready'",
			task:   &task.Task{Status: task.StatusReady},
			expect: true,
		},
		{
			name:   "alternating AND/OR chain",
			expr:   "status = 'ready' OR type = 'bug' AND priority > 3 OR points > 10 AND status = 'in_progress'",
			task:   &task.Task{Status: task.StatusReady, Type: task.TypeStory, Priority: 2, Points: 5},
			expect: true, // First OR condition matches
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

// TestComparisonOperators tests all comparison operators comprehensively
func TestComparisonOperators(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		// Equality
		{
			name:   "equality with =",
			expr:   "priority = 3",
			task:   &task.Task{Priority: 3},
			expect: true,
		},
		{
			name:   "equality with ==",
			expr:   "priority == 3",
			task:   &task.Task{Priority: 3},
			expect: true,
		},

		// Inequality
		{
			name:   "inequality - not equal",
			expr:   "priority != 3",
			task:   &task.Task{Priority: 5},
			expect: true,
		},
		{
			name:   "inequality - equal",
			expr:   "priority != 3",
			task:   &task.Task{Priority: 3},
			expect: false,
		},

		// Greater than
		{
			name:   "greater than - true",
			expr:   "priority > 3",
			task:   &task.Task{Priority: 5},
			expect: true,
		},
		{
			name:   "greater than - false equal",
			expr:   "priority > 3",
			task:   &task.Task{Priority: 3},
			expect: false,
		},
		{
			name:   "greater than - false less",
			expr:   "priority > 3",
			task:   &task.Task{Priority: 1},
			expect: false,
		},

		// Less than
		{
			name:   "less than - true",
			expr:   "priority < 3",
			task:   &task.Task{Priority: 1},
			expect: true,
		},
		{
			name:   "less than - false equal",
			expr:   "priority < 3",
			task:   &task.Task{Priority: 3},
			expect: false,
		},
		{
			name:   "less than - false greater",
			expr:   "priority < 3",
			task:   &task.Task{Priority: 5},
			expect: false,
		},

		// Greater than or equal
		{
			name:   "greater or equal - greater",
			expr:   "priority >= 3",
			task:   &task.Task{Priority: 5},
			expect: true,
		},
		{
			name:   "greater or equal - equal",
			expr:   "priority >= 3",
			task:   &task.Task{Priority: 3},
			expect: true,
		},
		{
			name:   "greater or equal - less",
			expr:   "priority >= 3",
			task:   &task.Task{Priority: 1},
			expect: false,
		},

		// Less than or equal
		{
			name:   "less or equal - less",
			expr:   "priority <= 3",
			task:   &task.Task{Priority: 1},
			expect: true,
		},
		{
			name:   "less or equal - equal",
			expr:   "priority <= 3",
			task:   &task.Task{Priority: 3},
			expect: true,
		},
		{
			name:   "less or equal - greater",
			expr:   "priority <= 3",
			task:   &task.Task{Priority: 5},
			expect: false,
		},

		// String comparisons (lexicographic)
		{
			name:   "string greater than",
			expr:   "status > 'in_progress'",
			task:   &task.Task{Status: task.StatusReady},
			expect: true, // "ready" > "in_progress" lexicographically
		},
		{
			name:   "string less than",
			expr:   "status < 'ready'",
			task:   &task.Task{Status: task.StatusDone},
			expect: true, // "done" < "ready" lexicographically
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
				t.Errorf("Expected %v, got %v for expression: %s (priority=%d, status=%s)",
					tt.expect, result, tt.expr, tt.task.Priority, tt.task.Status)
			}
		})
	}
}

// TestRealWorldScenarios tests realistic filter expressions users might write
func TestRealWorldScenarios(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		{
			name:   "recent high-priority bugs",
			expr:   "type = 'bug' AND priority >= 4 AND NOW - CreatedAt < 7days",
			task:   &task.Task{Type: task.TypeBug, Priority: 5, CreatedAt: now.Add(-3 * 24 * time.Hour)},
			expect: true,
		},
		{
			name:   "stale tasks needing attention",
			expr:   "status IN ['ready', 'in_progress', 'in_progress'] AND NOW - UpdatedAt > 14days",
			task:   &task.Task{Status: task.StatusReady, UpdatedAt: now.Add(-20 * 24 * time.Hour)},
			expect: true,
		},
		{
			name:   "UI/UX work in progress",
			expr:   "tags IN ['ui', 'ux', 'design'] AND status IN ['ready', 'in_progress'] AND type != 'epic'",
			task:   &task.Task{Tags: []string{"ui", "frontend"}, Status: task.StatusInProgress, Type: task.TypeStory},
			expect: true,
		},
		{
			name:   "ready for release",
			expr:   "status = 'done' AND type IN ['story', 'bug'] AND tags NOT IN ['not-deployable', 'experimental']",
			task:   &task.Task{Status: task.StatusDone, Type: task.TypeStory, Tags: []string{"ready"}},
			expect: true,
		},
		{
			name:   "blocked high-value items",
			expr:   "status = 'in_progress' AND (priority >= 4 OR points >= 8)",
			task:   &task.Task{Status: task.StatusInProgress, Priority: 2, Points: 10},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.expr)
			if err != nil {
				t.Fatalf("ParseFilter failed: %v", err)
			}

			result := filter.Evaluate(tt.task, now, "testuser")
			if result != tt.expect {
				t.Errorf("Expected %v, got %v for expression: %s", tt.expect, result, tt.expr)
			}
		})
	}
}
