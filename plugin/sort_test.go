package plugin

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/task"
)

func TestSortTasks_NoRules(t *testing.T) {
	tasks := []*task.Task{
		{ID: "TIKI-C", Priority: 3},
		{ID: "TIKI-A", Priority: 1},
		{ID: "TIKI-B", Priority: 2},
	}
	SortTasks(tasks, nil)
	// original order preserved
	if tasks[0].ID != "TIKI-C" || tasks[1].ID != "TIKI-A" || tasks[2].ID != "TIKI-B" {
		t.Errorf("expected original order preserved, got %v %v %v", tasks[0].ID, tasks[1].ID, tasks[2].ID)
	}
}

func TestSortTasks_ByField(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)
	later := now.Add(time.Hour)

	tests := []struct {
		name       string
		tasks      []*task.Task
		rules      []SortRule
		expectedID []string
	}{
		{
			name: "priority ASC",
			tasks: []*task.Task{
				{ID: "TIKI-C", Priority: 3},
				{ID: "TIKI-A", Priority: 1},
				{ID: "TIKI-B", Priority: 2},
			},
			rules:      []SortRule{{Field: "priority", Descending: false}},
			expectedID: []string{"TIKI-A", "TIKI-B", "TIKI-C"},
		},
		{
			name: "priority DESC",
			tasks: []*task.Task{
				{ID: "TIKI-A", Priority: 1},
				{ID: "TIKI-B", Priority: 2},
				{ID: "TIKI-C", Priority: 3},
			},
			rules:      []SortRule{{Field: "priority", Descending: true}},
			expectedID: []string{"TIKI-C", "TIKI-B", "TIKI-A"},
		},
		{
			name: "title ASC case-insensitive",
			tasks: []*task.Task{
				{ID: "TIKI-Z", Title: "Zebra"},
				{ID: "TIKI-A", Title: "apple"},
				{ID: "TIKI-M", Title: "Mango"},
			},
			rules:      []SortRule{{Field: "title", Descending: false}},
			expectedID: []string{"TIKI-A", "TIKI-M", "TIKI-Z"},
		},
		{
			name: "points ASC",
			tasks: []*task.Task{
				{ID: "TIKI-H", Points: 8},
				{ID: "TIKI-L", Points: 1},
				{ID: "TIKI-M", Points: 5},
			},
			rules:      []SortRule{{Field: "points", Descending: false}},
			expectedID: []string{"TIKI-L", "TIKI-M", "TIKI-H"},
		},
		{
			name: "assignee ASC",
			tasks: []*task.Task{
				{ID: "TIKI-Z", Assignee: "Zara"},
				{ID: "TIKI-A", Assignee: "alice"},
				{ID: "TIKI-M", Assignee: "Bob"},
			},
			rules:      []SortRule{{Field: "assignee", Descending: false}},
			expectedID: []string{"TIKI-A", "TIKI-M", "TIKI-Z"},
		},
		{
			name: "status ASC",
			tasks: []*task.Task{
				{ID: "TIKI-R", Status: "ready"},
				{ID: "TIKI-B", Status: "backlog"},
				{ID: "TIKI-D", Status: "done"},
			},
			rules:      []SortRule{{Field: "status", Descending: false}},
			expectedID: []string{"TIKI-B", "TIKI-D", "TIKI-R"},
		},
		{
			name: "type ASC",
			tasks: []*task.Task{
				{ID: "TIKI-S", Type: task.TypeStory},
				{ID: "TIKI-B", Type: task.TypeBug},
			},
			rules:      []SortRule{{Field: "type", Descending: false}},
			expectedID: []string{"TIKI-B", "TIKI-S"},
		},
		{
			name: "id ASC",
			tasks: []*task.Task{
				{ID: "TIKI-C"},
				{ID: "TIKI-A"},
				{ID: "TIKI-B"},
			},
			rules:      []SortRule{{Field: "id", Descending: false}},
			expectedID: []string{"TIKI-A", "TIKI-B", "TIKI-C"},
		},
		{
			name: "createdat ASC",
			tasks: []*task.Task{
				{ID: "TIKI-L", CreatedAt: later},
				{ID: "TIKI-E", CreatedAt: earlier},
				{ID: "TIKI-N", CreatedAt: now},
			},
			rules:      []SortRule{{Field: "createdat", Descending: false}},
			expectedID: []string{"TIKI-E", "TIKI-N", "TIKI-L"},
		},
		{
			name: "updatedat DESC",
			tasks: []*task.Task{
				{ID: "TIKI-E", UpdatedAt: earlier},
				{ID: "TIKI-L", UpdatedAt: later},
				{ID: "TIKI-N", UpdatedAt: now},
			},
			rules:      []SortRule{{Field: "updatedat", Descending: true}},
			expectedID: []string{"TIKI-L", "TIKI-N", "TIKI-E"},
		},
		{
			name: "multi-rule: priority ASC then title ASC",
			tasks: []*task.Task{
				{ID: "TIKI-B2", Priority: 2, Title: "Beta"},
				{ID: "TIKI-A2", Priority: 2, Title: "Alpha"},
				{ID: "TIKI-A1", Priority: 1, Title: "Zeta"},
			},
			rules: []SortRule{
				{Field: "priority", Descending: false},
				{Field: "title", Descending: false},
			},
			expectedID: []string{"TIKI-A1", "TIKI-A2", "TIKI-B2"},
		},
		{
			name: "unknown field — equal comparison, stable order preserved",
			tasks: []*task.Task{
				{ID: "TIKI-X"},
				{ID: "TIKI-Y"},
			},
			rules:      []SortRule{{Field: "nonexistent", Descending: false}},
			expectedID: []string{"TIKI-X", "TIKI-Y"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortTasks(tt.tasks, tt.rules)

			if len(tt.tasks) != len(tt.expectedID) {
				t.Fatalf("task count = %d, want %d", len(tt.tasks), len(tt.expectedID))
			}
			for i, want := range tt.expectedID {
				if tt.tasks[i].ID != want {
					t.Errorf("tasks[%d].ID = %q, want %q", i, tt.tasks[i].ID, want)
				}
			}
		})
	}
}
