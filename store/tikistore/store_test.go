package tikistore

import (
	"os"
	"reflect"
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

func TestSortTasks(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []*taskpkg.Task
		expected []string // expected order of IDs
	}{
		{
			name: "sort by priority first, then title",
			tasks: []*taskpkg.Task{
				{ID: "TIKI-abc123", Title: "Zebra Task", Priority: 2},
				{ID: "TIKI-def456", Title: "Alpha Task", Priority: 1},
				{ID: "TIKI-ghi789", Title: "Beta Task", Priority: 1},
			},
			expected: []string{"TIKI-def456", "TIKI-ghi789", "TIKI-abc123"}, // Alpha, Beta (both P1), then Zebra (P2)
		},
		{
			name: "same priority - alphabetical by title",
			tasks: []*taskpkg.Task{
				{ID: "TIKI-abc10z", Title: "Zebra", Priority: 3},
				{ID: "TIKI-abc2zz", Title: "Apple", Priority: 3},
				{ID: "TIKI-abc1zz", Title: "Mango", Priority: 3},
			},
			expected: []string{"TIKI-abc2zz", "TIKI-abc1zz", "TIKI-abc10z"}, // Apple, Mango, Zebra
		},
		{
			name:     "empty task list",
			tasks:    []*taskpkg.Task{},
			expected: []string{},
		},
		{
			name: "single task",
			tasks: []*taskpkg.Task{
				{ID: "TIKI-abc1zz", Title: "Only Task", Priority: 3},
			},
			expected: []string{"TIKI-abc1zz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortTasks(tt.tasks)

			if len(tt.tasks) != len(tt.expected) {
				t.Fatalf("task count = %d, want %d", len(tt.tasks), len(tt.expected))
			}

			for i, task := range tt.tasks {
				if task.ID != tt.expected[i] {
					t.Errorf("tasks[%d].ID = %q, want %q", i, task.ID, tt.expected[i])
				}
			}
		})
	}
}

func TestSearch_AllTasksIncludesDescription(t *testing.T) {
	store := &TikiStore{
		tasks: map[string]*taskpkg.Task{
			"TIKI-aaa111": {
				ID:          "TIKI-aaa111",
				Title:       "Alpha Task",
				Description: "Contains the keyword needle",
				Status:      taskpkg.StatusBacklog,
				Priority:    2,
			},
			"TIKI-bbb222": {
				ID:          "TIKI-bbb222",
				Title:       "Beta Task",
				Description: "No match here",
				Status:      taskpkg.StatusReady,
				Priority:    1,
			},
			"TIKI-ccc333": {
				ID:          "TIKI-ccc333",
				Title:       "Gamma Task",
				Description: "Another needle appears",
				Status:      taskpkg.StatusReview,
				Priority:    3,
			},
		},
	}

	results := store.Search("needle", nil)
	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}

	expectedIDs := []string{"TIKI-aaa111", "TIKI-ccc333"} // sorted by priority then title
	for i, result := range results {
		if result.Task.ID != expectedIDs[i] {
			t.Errorf("results[%d].Task.ID = %q, want %q", i, result.Task.ID, expectedIDs[i])
		}
		if result.Score != 1.0 {
			t.Errorf("results[%d].Score = %f, want 1.0", i, result.Score)
		}
	}
}
func TestLoadTaskFile_DependsOn(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name              string
		fileContent       string
		expectedDependsOn []string
		shouldLoad        bool
	}{
		{
			name: "valid dependsOn list",
			fileContent: `---
title: Test Task
type: story
status: backlog
dependsOn:
  - TIKI-ABC123
  - TIKI-DEF456
---
Task description`,
			expectedDependsOn: []string{"TIKI-ABC123", "TIKI-DEF456"},
			shouldLoad:        true,
		},
		{
			name: "lowercase IDs uppercased",
			fileContent: `---
title: Test Task
type: story
status: backlog
dependsOn:
  - tiki-abc123
---
Task description`,
			expectedDependsOn: []string{"TIKI-ABC123"},
			shouldLoad:        true,
		},
		{
			name: "missing dependsOn field",
			fileContent: `---
title: Test Task
type: story
status: backlog
---
Task description`,
			expectedDependsOn: []string{},
			shouldLoad:        true,
		},
		{
			name: "empty dependsOn array",
			fileContent: `---
title: Test Task
type: story
status: backlog
dependsOn: []
---
Task description`,
			expectedDependsOn: []string{},
			shouldLoad:        true,
		},
		{
			name: "invalid dependsOn - scalar",
			fileContent: `---
title: Test Task
type: story
status: backlog
dependsOn: not-a-list
---
Task description`,
			expectedDependsOn: []string{},
			shouldLoad:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := tmpDir + "/test-task.md"
			err := os.WriteFile(testFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			store, storeErr := NewTikiStore(tmpDir)
			if storeErr != nil {
				t.Fatalf("Failed to create TikiStore: %v", storeErr)
			}

			task, err := store.loadTaskFile(testFile, nil, nil)

			if tt.shouldLoad {
				if err != nil {
					t.Fatalf("loadTaskFile() unexpected error = %v", err)
				}
				if task == nil {
					t.Fatal("loadTaskFile() returned nil task")
				}

				if !reflect.DeepEqual(task.DependsOn, tt.expectedDependsOn) {
					t.Errorf("task.DependsOn = %v, expected %v", task.DependsOn, tt.expectedDependsOn)
				}
			} else {
				if err == nil {
					t.Error("loadTaskFile() expected error but got none")
				}
			}

			_ = os.Remove(testFile)
		})
	}
}

func TestLoadTaskFile_InvalidTags(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		fileContent  string
		expectedTags []string
		shouldLoad   bool
	}{
		{
			name: "valid tags list",
			fileContent: `---
title: Test Task
type: story
status: backlog
tags:
  - frontend
  - backend
---
Task description`,
			expectedTags: []string{"frontend", "backend"},
			shouldLoad:   true,
		},
		{
			name: "invalid tags - scalar string",
			fileContent: `---
title: Test Task
type: story
status: backlog
tags: not-a-list
---
Task description`,
			expectedTags: []string{},
			shouldLoad:   true,
		},
		{
			name: "invalid tags - number",
			fileContent: `---
title: Test Task
type: story
status: backlog
tags: 123
---
Task description`,
			expectedTags: []string{},
			shouldLoad:   true,
		},
		{
			name: "invalid tags - boolean",
			fileContent: `---
title: Test Task
type: story
status: backlog
tags: true
---
Task description`,
			expectedTags: []string{},
			shouldLoad:   true,
		},
		{
			name: "invalid tags - object",
			fileContent: `---
title: Test Task
type: story
status: backlog
tags:
  key: value
---
Task description`,
			expectedTags: []string{},
			shouldLoad:   true,
		},
		{
			name: "missing tags field",
			fileContent: `---
title: Test Task
type: story
status: backlog
---
Task description`,
			expectedTags: []string{},
			shouldLoad:   true,
		},
		{
			name: "empty tags array",
			fileContent: `---
title: Test Task
type: story
status: backlog
tags: []
---
Task description`,
			expectedTags: []string{},
			shouldLoad:   true,
		},
		{
			name: "tags with empty strings filtered",
			fileContent: `---
title: Test Task
type: story
status: backlog
tags:
  - frontend
  - ""
  - backend
---
Task description`,
			expectedTags: []string{"frontend", "backend"},
			shouldLoad:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := tmpDir + "/test-task.md"
			err := os.WriteFile(testFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Create TikiStore
			store, storeErr := NewTikiStore(tmpDir)
			if storeErr != nil {
				t.Fatalf("Failed to create TikiStore: %v", storeErr)
			}

			// Load the task file directly
			task, err := store.loadTaskFile(testFile, nil, nil)

			if tt.shouldLoad {
				if err != nil {
					t.Fatalf("loadTaskFile() unexpected error = %v", err)
				}
				if task == nil {
					t.Fatal("loadTaskFile() returned nil task")
				}

				// Verify tags
				if !reflect.DeepEqual(task.Tags, tt.expectedTags) {
					t.Errorf("task.Tags = %v, expected %v", task.Tags, tt.expectedTags)
				}

				// Verify other fields still work
				if task.Title != "Test Task" {
					t.Errorf("task.Title = %q, expected %q", task.Title, "Test Task")
				}
				if task.Type != taskpkg.TypeStory {
					t.Errorf("task.Type = %q, expected %q", task.Type, taskpkg.TypeStory)
				}
				if task.Status != taskpkg.StatusBacklog {
					t.Errorf("task.Status = %q, expected %q", task.Status, taskpkg.StatusBacklog)
				}
			} else {
				if err == nil {
					t.Error("loadTaskFile() expected error but got none")
				}
			}

			// Clean up test file
			_ = os.Remove(testFile)
		})
	}
}
