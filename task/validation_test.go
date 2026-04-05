package task

import (
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/workflow"
)

func init() {
	// set up the default status registry for tests.
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "in_progress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "review", Label: "Review", Emoji: "👀", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	})
}

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"valid title", &Task{Title: "Valid Task"}, false},
		{"empty title", &Task{Title: ""}, true},
		{"whitespace title", &Task{Title: "   "}, true},
		{"very long title", &Task{Title: strings.Repeat("a", 201)}, true},
		{"max length title", &Task{Title: strings.Repeat("a", 200)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ValidateTitle(tt.task)
			if (msg != "") != tt.wantErr {
				t.Errorf("ValidateTitle() = %q, wantErr %v", msg, tt.wantErr)
			}
		})
	}
}

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"valid backlog", &Task{Status: StatusBacklog}, false},
		{"valid ready", &Task{Status: StatusReady}, false},
		{"valid in_progress", &Task{Status: StatusInProgress}, false},
		{"valid review", &Task{Status: StatusReview}, false},
		{"valid done", &Task{Status: StatusDone}, false},
		{"invalid status", &Task{Status: "invalid"}, true},
		{"empty status", &Task{Status: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ValidateStatus(tt.task)
			if (msg != "") != tt.wantErr {
				t.Errorf("ValidateStatus() = %q, wantErr %v", msg, tt.wantErr)
			}
		})
	}
}

func TestValidateType(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"valid story", &Task{Type: TypeStory}, false},
		{"valid bug", &Task{Type: TypeBug}, false},
		{"valid spike", &Task{Type: TypeSpike}, false},
		{"valid epic", &Task{Type: TypeEpic}, false},
		{"invalid type", &Task{Type: "invalid"}, true},
		{"empty type", &Task{Type: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ValidateType(tt.task)
			if (msg != "") != tt.wantErr {
				t.Errorf("ValidateType() = %q, wantErr %v", msg, tt.wantErr)
			}
		})
	}
}

func TestValidatePriority(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"valid priority 1", &Task{Priority: 1}, false},
		{"valid priority 3", &Task{Priority: 3}, false},
		{"valid priority 5", &Task{Priority: 5}, false},
		{"invalid priority 0", &Task{Priority: 0}, true},
		{"invalid priority 6", &Task{Priority: 6}, true},
		{"invalid priority -1", &Task{Priority: -1}, true},
		{"invalid priority 10", &Task{Priority: 10}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ValidatePriority(tt.task)
			if (msg != "") != tt.wantErr {
				t.Errorf("ValidatePriority() = %q, wantErr %v", msg, tt.wantErr)
			}
		})
	}
}

func TestValidatePoints(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"valid points 0 (unestimated)", &Task{Points: 0}, false},
		{"valid points 1", &Task{Points: 1}, false},
		{"valid points 5", &Task{Points: 5}, false},
		{"valid points 10", &Task{Points: 10}, false},
		{"invalid points -1", &Task{Points: -1}, true},
		{"invalid points 11", &Task{Points: 11}, true},
		{"invalid points 100", &Task{Points: 100}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ValidatePoints(tt.task)
			if (msg != "") != tt.wantErr {
				t.Errorf("ValidatePoints() = %q, wantErr %v", msg, tt.wantErr)
			}
		})
	}
}

func TestValidateDependsOn(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"empty dependsOn", &Task{DependsOn: nil}, false},
		{"valid single dependency", &Task{DependsOn: []string{"TIKI-ABC123"}}, false},
		{"valid multiple dependencies", &Task{DependsOn: []string{"TIKI-ABC123", "TIKI-DEF456"}}, false},
		{"invalid format - lowercase", &Task{DependsOn: []string{"tiki-abc123"}}, true},
		{"invalid format - wrong prefix", &Task{DependsOn: []string{"TASK-ABC123"}}, true},
		{"invalid format - too short", &Task{DependsOn: []string{"TIKI-ABC"}}, true},
		{"invalid format - too long", &Task{DependsOn: []string{"TIKI-ABC1234"}}, true},
		{"invalid format - special chars", &Task{DependsOn: []string{"TIKI-ABC12!"}}, true},
		{"mixed valid and invalid", &Task{DependsOn: []string{"TIKI-ABC123", "bad-id"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ValidateDependsOn(tt.task)
			if (msg != "") != tt.wantErr {
				t.Errorf("ValidateDependsOn() = %q, wantErr %v", msg, tt.wantErr)
			}
		})
	}
}

func TestValidateDue(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{
			name:    "no due date (zero time)",
			task:    &Task{Title: "Test", Type: TypeStory, Status: "backlog", Priority: DefaultPriority},
			wantErr: false,
		},
		{
			name: "valid due date (midnight UTC)",
			task: &Task{
				Title:    "Test",
				Type:     TypeStory,
				Status:   "backlog",
				Priority: DefaultPriority,
				Due:      mustParseDate("2026-03-16"),
			},
			wantErr: false,
		},
		{
			name: "invalid due date (has time component)",
			task: &Task{
				Title:    "Test",
				Type:     TypeStory,
				Status:   "backlog",
				Priority: DefaultPriority,
				Due:      mustParseDateTime("2026-03-16T15:04:05Z"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ValidateDue(tt.task)
			if (msg != "") != tt.wantErr {
				t.Errorf("ValidateDue() = %q, wantErr %v", msg, tt.wantErr)
			}
		})
	}
}

func TestValidateRecurrence(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"empty recurrence (none)", &Task{Recurrence: RecurrenceNone}, false},
		{"valid daily", &Task{Recurrence: RecurrenceDaily}, false},
		{"valid weekly monday", &Task{Recurrence: "0 0 * * MON"}, false},
		{"valid monthly", &Task{Recurrence: RecurrenceMonthly}, false},
		{"invalid cron pattern", &Task{Recurrence: "*/5 * * * *"}, true},
		{"invalid string", &Task{Recurrence: "every day"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ValidateRecurrence(tt.task)
			if (msg != "") != tt.wantErr {
				t.Errorf("ValidateRecurrence() = %q, wantErr %v", msg, tt.wantErr)
			}
		})
	}
}

func TestAllValidators_MultipleErrors(t *testing.T) {
	tk := &Task{
		Title:    "",        // invalid: empty
		Status:   "invalid", // invalid: not a valid enum
		Type:     "bad",     // invalid: not a valid enum
		Priority: 10,        // invalid: out of range
		Points:   -5,        // invalid: negative
	}

	var errors []string
	for _, fn := range AllValidators() {
		if msg := fn(tk); msg != "" {
			errors = append(errors, msg)
		}
	}

	if len(errors) != 5 {
		t.Errorf("expected 5 errors, got %d: %v", len(errors), errors)
	}
}

func TestAllValidators_ValidTask(t *testing.T) {
	tk := &Task{
		Title:    "Valid Task",
		Status:   StatusReady,
		Type:     TypeStory,
		Priority: 3,
		Points:   5,
	}

	for _, fn := range AllValidators() {
		if msg := fn(tk); msg != "" {
			t.Errorf("unexpected validation error: %s", msg)
		}
	}
}

func TestIsValidTikiIDFormat(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"TIKI-ABC123", true},
		{"TIKI-000000", true},
		{"tiki-abc123", false},
		{"TASK-ABC123", false},
		{"TIKI-ABC", false},
		{"TIKI-ABC1234", false},
		{"TIKI-ABC12!", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := IsValidTikiIDFormat(tt.id); got != tt.want {
				t.Errorf("IsValidTikiIDFormat(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

// Helper for tests
func mustParseDate(s string) time.Time {
	t, err := time.Parse(DateFormat, s)
	if err != nil {
		panic(err)
	}
	return t
}

func mustParseDateTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
