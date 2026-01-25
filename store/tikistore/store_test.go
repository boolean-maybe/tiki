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

func TestSearchBacklog(t *testing.T) {
	// Create a test store with tasks
	store := &TikiStore{
		tasks: map[string]*taskpkg.Task{
			"TIKI-abc123": {
				ID:       "TIKI-abc123",
				Title:    "Testing Task",
				Status:   taskpkg.StatusBacklog,
				Priority: 2,
			},
			"TIKI-def456": {
				ID:       "TIKI-def456",
				Title:    "Bug in Tests",
				Status:   taskpkg.StatusBacklog,
				Priority: 1,
			},
			"TIKI-ghi789": {
				ID:       "TIKI-ghi789",
				Title:    "Feature Request",
				Status:   taskpkg.StatusBacklog,
				Priority: 3,
			},
			"TIKI-jkl012": {
				ID:       "TIKI-jkl012",
				Title:    "In Progress Task",
				Status:   taskpkg.StatusReady, // not backlog
				Priority: 1,
			},
		},
	}

	tests := []struct {
		name          string
		query         string
		expectedIDs   []string
		expectedCount int
	}{
		{
			name:          "case insensitive substring match",
			query:         "test",
			expectedIDs:   []string{"TIKI-def456", "TIKI-abc123"}, // sorted by priority (1 before 2)
			expectedCount: 2,
		},
		{
			name:          "uppercase query",
			query:         "TEST",
			expectedIDs:   []string{"TIKI-def456", "TIKI-abc123"}, // sorted by priority (1 before 2)
			expectedCount: 2,
		},
		{
			name:          "partial word match",
			query:         "Task",
			expectedIDs:   []string{"TIKI-abc123"},
			expectedCount: 1,
		},
		{
			name:          "empty query returns all backlog sorted",
			query:         "",
			expectedIDs:   []string{"TIKI-def456", "TIKI-abc123", "TIKI-ghi789"}, // P1, P2, P3
			expectedCount: 3,
		},
		{
			name:          "whitespace query returns all backlog",
			query:         "   ",
			expectedIDs:   []string{"TIKI-def456", "TIKI-abc123", "TIKI-ghi789"}, // P1, P2, P3
			expectedCount: 3,
		},
		{
			name:          "no matches",
			query:         "nonexistent",
			expectedIDs:   []string{},
			expectedCount: 0,
		},
		{
			name:          "only searches backlog status",
			query:         "Progress", // matches TIKI-jkl012 title but wrong status
			expectedIDs:   []string{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := store.SearchBacklog(tt.query)

			if len(results) != tt.expectedCount {
				t.Errorf("result count = %d, want %d", len(results), tt.expectedCount)
			}

			for i, result := range results {
				if result.Task.ID != tt.expectedIDs[i] {
					t.Errorf("results[%d].Task.ID = %q, want %q", i, result.Task.ID, tt.expectedIDs[i])
				}

				// All scores should be 1.0
				if result.Score != 1.0 {
					t.Errorf("results[%d].Score = %f, want 1.0", i, result.Score)
				}
			}
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
