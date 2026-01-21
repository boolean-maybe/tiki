package plugin

import (
	"reflect"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

func TestSplitTopLevelCommas(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr string
	}{
		{
			name:  "simple split",
			input: "status=todo, type=bug",
			want:  []string{"status=todo", "type=bug"},
		},
		{
			name:  "comma in quotes",
			input: "assignee='O,Brien', status=done",
			want:  []string{"assignee='O,Brien'", "status=done"},
		},
		{
			name:  "comma in brackets",
			input: "tags+=[one,two], status=done",
			want:  []string{"tags+=[one,two]", "status=done"},
		},
		{
			name:  "mixed quotes and brackets",
			input: `tags+=[one,"two,three"], status=done`,
			want:  []string{`tags+=[one,"two,three"]`, "status=done"},
		},
		{
			name:    "unterminated quote",
			input:   "status='todo, type=bug",
			wantErr: "unterminated quotes or brackets",
		},
		{
			name:    "unterminated brackets",
			input:   "tags+=[one,two, status=done",
			wantErr: "unterminated quotes or brackets",
		},
		{
			name:    "unexpected closing bracket",
			input:   "status=todo], type=bug",
			wantErr: "unexpected ']'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitTopLevelCommas(tc.input)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestParsePaneAction(t *testing.T) {
	action, err := ParsePaneAction("status=done, type=bug, priority=2, points=3, assignee='Alice', tags+=[frontend,'needs review']")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(action.Ops) != 6 {
		t.Fatalf("expected 6 ops, got %d", len(action.Ops))
	}

	gotFields := []ActionField{
		action.Ops[0].Field,
		action.Ops[1].Field,
		action.Ops[2].Field,
		action.Ops[3].Field,
		action.Ops[4].Field,
		action.Ops[5].Field,
	}
	wantFields := []ActionField{
		ActionFieldStatus,
		ActionFieldType,
		ActionFieldPriority,
		ActionFieldPoints,
		ActionFieldAssignee,
		ActionFieldTags,
	}
	if !reflect.DeepEqual(gotFields, wantFields) {
		t.Fatalf("expected fields %v, got %v", wantFields, gotFields)
	}

	if action.Ops[0].StrValue != "done" {
		t.Fatalf("expected status value 'done', got %q", action.Ops[0].StrValue)
	}
	if action.Ops[1].StrValue != "bug" {
		t.Fatalf("expected type value 'bug', got %q", action.Ops[1].StrValue)
	}
	if action.Ops[2].IntValue != 2 {
		t.Fatalf("expected priority 2, got %d", action.Ops[2].IntValue)
	}
	if action.Ops[3].IntValue != 3 {
		t.Fatalf("expected points 3, got %d", action.Ops[3].IntValue)
	}
	if action.Ops[4].StrValue != "Alice" {
		t.Fatalf("expected assignee Alice, got %q", action.Ops[4].StrValue)
	}
	if !reflect.DeepEqual(action.Ops[5].Tags, []string{"frontend", "needs review"}) {
		t.Fatalf("expected tags [frontend needs review], got %v", action.Ops[5].Tags)
	}
}

func TestParsePaneAction_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "empty segment",
			input:   "status=done,,type=bug",
			wantErr: "empty action segment",
		},
		{
			name:    "missing operator",
			input:   "statusdone",
			wantErr: "missing operator",
		},
		{
			name:    "tags assign not allowed",
			input:   "tags=[one]",
			wantErr: "tags action only supports",
		},
		{
			name:    "status add not allowed",
			input:   "status+=done",
			wantErr: "status action only supports",
		},
		{
			name:    "unknown field",
			input:   "owner=me",
			wantErr: "unknown action field",
		},
		{
			name:    "invalid status",
			input:   "status=unknown",
			wantErr: "invalid status value",
		},
		{
			name:    "invalid type",
			input:   "type=unknown",
			wantErr: "invalid type value",
		},
		{
			name:    "priority out of range",
			input:   "priority=10",
			wantErr: "priority value out of range",
		},
		{
			name:    "points out of range",
			input:   "points=-1",
			wantErr: "points value out of range",
		},
		{
			name:    "tags missing brackets",
			input:   "tags+={one}",
			wantErr: "tags value must be in brackets",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePaneAction(tc.input)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestApplyPaneAction(t *testing.T) {
	base := &task.Task{
		ID:       "TASK-1",
		Title:    "Task",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: task.PriorityMedium,
		Points:   1,
		Tags:     []string{"existing"},
		Assignee: "Bob",
	}

	action, err := ParsePaneAction("status=done, type=bug, priority=2, points=3, assignee=Alice, tags+=[moved]")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	updated, err := ApplyPaneAction(base, action, "")
	if err != nil {
		t.Fatalf("unexpected apply error: %v", err)
	}

	if updated.Status != task.StatusDone {
		t.Fatalf("expected status done, got %v", updated.Status)
	}
	if updated.Type != task.TypeBug {
		t.Fatalf("expected type bug, got %v", updated.Type)
	}
	if updated.Priority != 2 {
		t.Fatalf("expected priority 2, got %d", updated.Priority)
	}
	if updated.Points != 3 {
		t.Fatalf("expected points 3, got %d", updated.Points)
	}
	if updated.Assignee != "Alice" {
		t.Fatalf("expected assignee Alice, got %q", updated.Assignee)
	}
	if !reflect.DeepEqual(updated.Tags, []string{"existing", "moved"}) {
		t.Fatalf("expected tags [existing moved], got %v", updated.Tags)
	}
	if base.Status != task.StatusBacklog {
		t.Fatalf("expected base task unchanged, got %v", base.Status)
	}
	if !reflect.DeepEqual(base.Tags, []string{"existing"}) {
		t.Fatalf("expected base tags unchanged, got %v", base.Tags)
	}
}

func TestApplyPaneAction_InvalidResult(t *testing.T) {
	base := &task.Task{
		ID:       "TASK-1",
		Title:    "Task",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: task.PriorityMedium,
		Points:   1,
	}

	action := PaneAction{
		Ops: []PaneActionOp{
			{
				Field:    ActionFieldPriority,
				Operator: ActionOperatorAssign,
				IntValue: 99,
			},
		},
	}

	_, err := ApplyPaneAction(base, action, "")
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestApplyPaneAction_AssigneeCurrentUser(t *testing.T) {
	base := &task.Task{
		ID:       "TASK-1",
		Title:    "Task",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: task.PriorityMedium,
		Points:   1,
		Assignee: "Bob",
	}

	action, err := ParsePaneAction("assignee=CURRENT_USER")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	updated, err := ApplyPaneAction(base, action, "Alex")
	if err != nil {
		t.Fatalf("unexpected apply error: %v", err)
	}
	if updated.Assignee != "Alex" {
		t.Fatalf("expected assignee Alex, got %q", updated.Assignee)
	}
}

func TestApplyPaneAction_AssigneeCurrentUserMissing(t *testing.T) {
	base := &task.Task{
		ID:       "TASK-1",
		Title:    "Task",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: task.PriorityMedium,
		Points:   1,
	}

	action, err := ParsePaneAction("assignee=CURRENT_USER")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	_, err = ApplyPaneAction(base, action, "")
	if err == nil {
		t.Fatalf("expected error for missing current user")
	}
	if !strings.Contains(err.Error(), "current user") {
		t.Fatalf("expected current user error, got %v", err)
	}
}
