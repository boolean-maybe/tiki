package filter

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/task"
)

// TestTimeExpressions tests time-based filter expressions like "NOW - CreatedAt < 24hour"
func TestTimeExpressions(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		// NOW - UpdatedAt comparisons
		{
			name:   "recent task - under 2 hours",
			expr:   "NOW - UpdatedAt < 2hours",
			task:   &task.Task{UpdatedAt: now.Add(-1 * time.Hour)},
			expect: true, // Updated 1 hour ago, less than 2 hours
		},
		{
			name:   "old task - over 2 hours",
			expr:   "NOW - UpdatedAt < 2hours",
			task:   &task.Task{UpdatedAt: now.Add(-3 * time.Hour)},
			expect: false, // Updated 3 hours ago, more than 2 hours
		},
		{
			name:   "exact boundary - 2 hours",
			expr:   "NOW - UpdatedAt < 2hours",
			task:   &task.Task{UpdatedAt: now.Add(-2 * time.Hour)},
			expect: false, // Updated exactly 2 hours ago, not less than 2 hours
		},

		// Greater than comparisons
		{
			name:   "old task - over 1 week",
			expr:   "NOW - UpdatedAt > 1week",
			task:   &task.Task{UpdatedAt: now.Add(-10 * 24 * time.Hour)},
			expect: true, // Updated 10 days ago, more than 1 week
		},
		{
			name:   "recent task - under 1 week",
			expr:   "NOW - UpdatedAt > 1week",
			task:   &task.Task{UpdatedAt: now.Add(-3 * 24 * time.Hour)},
			expect: false, // Updated 3 days ago, less than 1 week
		},

		// NOW - CreatedAt comparisons
		{
			name:   "recently created - under 1 month",
			expr:   "NOW - CreatedAt < 1month",
			task:   &task.Task{CreatedAt: now.Add(-15 * 24 * time.Hour)},
			expect: true, // Created 15 days ago, less than 30 days (1 month)
		},
		{
			name:   "old creation - over 1 month",
			expr:   "NOW - CreatedAt < 1month",
			task:   &task.Task{CreatedAt: now.Add(-40 * 24 * time.Hour)},
			expect: false, // Created 40 days ago, more than 30 days
		},

		// Less than or equal comparisons
		{
			name:   "task age <= 24 hours - exact match",
			expr:   "NOW - UpdatedAt <= 24hours",
			task:   &task.Task{UpdatedAt: now.Add(-24 * time.Hour)},
			expect: true, // Updated exactly 24 hours ago
		},
		{
			name:   "task age <= 24 hours - under",
			expr:   "NOW - UpdatedAt <= 24hours",
			task:   &task.Task{UpdatedAt: now.Add(-12 * time.Hour)},
			expect: true, // Updated 12 hours ago
		},
		{
			name:   "task age <= 24 hours - over",
			expr:   "NOW - UpdatedAt <= 24hours",
			task:   &task.Task{UpdatedAt: now.Add(-30 * time.Hour)},
			expect: false, // Updated 30 hours ago
		},

		// Greater than or equal comparisons
		{
			name:   "task age >= 1 day - exact match",
			expr:   "NOW - CreatedAt >= 1day",
			task:   &task.Task{CreatedAt: now.Add(-24 * time.Hour)},
			expect: true, // Created exactly 1 day ago
		},
		{
			name:   "task age >= 1 day - over",
			expr:   "NOW - CreatedAt >= 1day",
			task:   &task.Task{CreatedAt: now.Add(-48 * time.Hour)},
			expect: true, // Created 2 days ago
		},
		{
			name:   "task age >= 1 day - under",
			expr:   "NOW - CreatedAt >= 1day",
			task:   &task.Task{CreatedAt: now.Add(-12 * time.Hour)},
			expect: false, // Created 12 hours ago
		},

		// Different duration units
		{
			name:   "minutes - under threshold",
			expr:   "NOW - UpdatedAt < 60min",
			task:   &task.Task{UpdatedAt: now.Add(-30 * time.Minute)},
			expect: true, // Updated 30 minutes ago
		},
		{
			name:   "days - over threshold",
			expr:   "NOW - UpdatedAt > 7days",
			task:   &task.Task{UpdatedAt: now.Add(-10 * 24 * time.Hour)},
			expect: true, // Updated 10 days ago
		},
		{
			name:   "weeks - under threshold",
			expr:   "NOW - CreatedAt < 2weeks",
			task:   &task.Task{CreatedAt: now.Add(-10 * 24 * time.Hour)},
			expect: true, // Created 10 days ago, less than 14 days
		},

		// Combined with other conditions
		{
			name:   "time condition AND status",
			expr:   "NOW - UpdatedAt < 24hours AND status = 'ready'",
			task:   &task.Task{UpdatedAt: now.Add(-12 * time.Hour), Status: task.StatusReady},
			expect: true,
		},
		{
			name:   "time condition AND status - status mismatch",
			expr:   "NOW - UpdatedAt < 24hours AND status = 'ready'",
			task:   &task.Task{UpdatedAt: now.Add(-12 * time.Hour), Status: task.StatusDone},
			expect: false,
		},
		{
			name:   "time condition AND status - time mismatch",
			expr:   "NOW - UpdatedAt < 24hours AND status = 'ready'",
			task:   &task.Task{UpdatedAt: now.Add(-48 * time.Hour), Status: task.StatusReady},
			expect: false,
		},

		// Time condition OR other conditions
		{
			name:   "time condition OR type - time matches",
			expr:   "NOW - UpdatedAt < 1hour OR type = 'bug'",
			task:   &task.Task{UpdatedAt: now.Add(-30 * time.Minute), Type: task.TypeStory},
			expect: true,
		},
		{
			name:   "time condition OR type - type matches",
			expr:   "NOW - UpdatedAt < 1hour OR type = 'bug'",
			task:   &task.Task{UpdatedAt: now.Add(-5 * time.Hour), Type: task.TypeBug},
			expect: true,
		},
		{
			name:   "time condition OR type - neither matches",
			expr:   "NOW - UpdatedAt < 1hour OR type = 'bug'",
			task:   &task.Task{UpdatedAt: now.Add(-5 * time.Hour), Type: task.TypeStory},
			expect: false,
		},

		// NOT with time conditions
		{
			name:   "NOT time condition - should match",
			expr:   "NOT (NOW - UpdatedAt < 24hours)",
			task:   &task.Task{UpdatedAt: now.Add(-48 * time.Hour)},
			expect: true, // Updated 48 hours ago, NOT less than 24 hours
		},
		{
			name:   "NOT time condition - should not match",
			expr:   "NOT (NOW - UpdatedAt < 24hours)",
			task:   &task.Task{UpdatedAt: now.Add(-12 * time.Hour)},
			expect: false, // Updated 12 hours ago, NOT (less than 24 hours) = false
		},

		// Equality (rarely useful but should work)
		{
			name:   "time equality - not equal",
			expr:   "NOW - UpdatedAt = 24hours",
			task:   &task.Task{UpdatedAt: now.Add(-25 * time.Hour)},
			expect: false, // Small timing differences make exact equality unlikely
		},

		// Inequality
		{
			name:   "time inequality - not equal",
			expr:   "NOW - UpdatedAt != 0min",
			task:   &task.Task{UpdatedAt: now.Add(-5 * time.Minute)},
			expect: true, // Updated 5 minutes ago, not equal to 0
		},

		// Edge case: very recent update (near zero duration)
		{
			name:   "very recent update",
			expr:   "NOW - UpdatedAt < 1min",
			task:   &task.Task{UpdatedAt: now.Add(-5 * time.Second)},
			expect: true, // Updated 5 seconds ago
		},

		// Edge case: future time (shouldn't normally happen, but test negative duration)
		{
			name:   "future time - negative duration",
			expr:   "NOW - UpdatedAt < 1hour",
			task:   &task.Task{UpdatedAt: now.Add(1 * time.Hour)},
			expect: true, // Future time results in negative duration, which is < 1hour
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
				t.Errorf("Expected %v, got %v for expression: %s\nTask UpdatedAt: %v, CreatedAt: %v",
					tt.expect, result, tt.expr, tt.task.UpdatedAt, tt.task.CreatedAt)
			}
		})
	}
}

// TestTimeExpressionParsing tests that time expressions parse correctly
func TestTimeExpressionParsing(t *testing.T) {
	tests := []struct {
		name        string
		expr        string
		shouldError bool
	}{
		{
			name:        "valid NOW - UpdatedAt",
			expr:        "NOW - UpdatedAt < 24hours",
			shouldError: false,
		},
		{
			name:        "valid NOW - CreatedAt",
			expr:        "NOW - CreatedAt > 1week",
			shouldError: false,
		},
		{
			name:        "valid with minutes",
			expr:        "NOW - UpdatedAt < 30min",
			shouldError: false,
		},
		{
			name:        "valid with days",
			expr:        "NOW - CreatedAt >= 7days",
			shouldError: false,
		},
		{
			name:        "valid with months",
			expr:        "NOW - UpdatedAt < 2months",
			shouldError: false,
		},
		{
			name:        "valid with parentheses",
			expr:        "(NOW - UpdatedAt < 1hour)",
			shouldError: false,
		},
		{
			name:        "valid combined with AND",
			expr:        "NOW - UpdatedAt < 1day AND status = 'ready'",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.expr)
			if tt.shouldError && err == nil {
				t.Error("Expected parsing error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if !tt.shouldError && filter == nil {
				t.Error("Expected filter but got nil")
			}
		})
	}
}

// TestMultipleTimeConditions tests filters with multiple time-based conditions
func TestMultipleTimeConditions(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		expr   string
		task   *task.Task
		expect bool
	}{
		{
			name: "both time conditions true",
			expr: "NOW - CreatedAt > 7days AND NOW - UpdatedAt < 24hours",
			task: &task.Task{
				CreatedAt: now.Add(-10 * 24 * time.Hour),
				UpdatedAt: now.Add(-12 * time.Hour),
			},
			expect: true,
		},
		{
			name: "first time condition false",
			expr: "NOW - CreatedAt > 7days AND NOW - UpdatedAt < 24hours",
			task: &task.Task{
				CreatedAt: now.Add(-3 * 24 * time.Hour),
				UpdatedAt: now.Add(-12 * time.Hour),
			},
			expect: false,
		},
		{
			name: "second time condition false",
			expr: "NOW - CreatedAt > 7days AND NOW - UpdatedAt < 24hours",
			task: &task.Task{
				CreatedAt: now.Add(-10 * 24 * time.Hour),
				UpdatedAt: now.Add(-48 * time.Hour),
			},
			expect: false,
		},
		{
			name: "time conditions with OR",
			expr: "NOW - UpdatedAt < 1hour OR NOW - CreatedAt < 1day",
			task: &task.Task{
				CreatedAt: now.Add(-10 * 24 * time.Hour),
				UpdatedAt: now.Add(-30 * time.Minute),
			},
			expect: true, // Updated recently
		},
		{
			name: "complex time expression with status",
			expr: "(NOW - UpdatedAt < 2hours OR NOW - CreatedAt < 1day) AND status = 'ready'",
			task: &task.Task{
				CreatedAt: now.Add(-10 * 24 * time.Hour),
				UpdatedAt: now.Add(-1 * time.Hour),
				Status:    task.StatusReady,
			},
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
