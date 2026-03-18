package tikistore

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
			name: "same priority and title - tiebreak by ID",
			tasks: []*taskpkg.Task{
				{ID: "TIKI-ccc333", Title: "Same", Priority: 2},
				{ID: "TIKI-aaa111", Title: "Same", Priority: 2},
				{ID: "TIKI-bbb222", Title: "Same", Priority: 2},
			},
			expected: []string{"TIKI-aaa111", "TIKI-bbb222", "TIKI-ccc333"},
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

		})
	}
}

func TestLoadTaskFile_Due(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		fileContent string
		expectZero  bool
		expectValue string // YYYY-MM-DD format if not zero
		shouldLoad  bool
	}{
		{
			name: "valid due date",
			fileContent: `---
title: Test Task
type: story
status: backlog
due: 2026-03-16
---
Task description`,
			expectZero:  false,
			expectValue: "2026-03-16",
			shouldLoad:  true,
		},
		{
			name: "valid due date with quotes",
			fileContent: `---
title: Test Task
type: story
status: backlog
due: '2026-03-16'
---
Task description`,
			expectZero:  false,
			expectValue: "2026-03-16",
			shouldLoad:  true,
		},
		{
			name: "missing due field",
			fileContent: `---
title: Test Task
type: story
status: backlog
---
Task description`,
			expectZero: true,
			shouldLoad: true,
		},
		{
			name: "empty due field",
			fileContent: `---
title: Test Task
type: story
status: backlog
due: ''
---
Task description`,
			expectZero: true,
			shouldLoad: true,
		},
		{
			name: "invalid due date format",
			fileContent: `---
title: Test Task
type: story
status: backlog
due: 03/16/2026
---
Task description`,
			expectZero: true, // Should default to zero
			shouldLoad: true,
		},
		{
			name: "invalid due date - number",
			fileContent: `---
title: Test Task
type: story
status: backlog
due: 20260316
---
Task description`,
			expectZero: true, // Should default to zero
			shouldLoad: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create task file
			testFile := filepath.Join(tmpDir, "TIKI-TEST01.md")
			if err := os.WriteFile(testFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			// Load store
			store, err := NewTikiStore(tmpDir)
			if err != nil {
				t.Fatalf("NewTikiStore() error = %v", err)
			}

			// Get task
			task := store.GetTask("TIKI-TEST01")
			if !tt.shouldLoad {
				if task != nil {
					t.Error("expected nil task, but got one")
				}
				return
			}

			if task == nil {
				t.Fatal("GetTask() returned nil")
			}

			if tt.expectZero {
				if !task.Due.IsZero() {
					t.Errorf("expected zero due time, got = %v", task.Due)
				}
			} else {
				if task.Due.IsZero() {
					t.Error("expected non-zero due time, got zero")
				}
				got := task.Due.Format(taskpkg.DateFormat)
				if got != tt.expectValue {
					t.Errorf("due date got = %v, expected %v", got, tt.expectValue)
				}
			}

			// Clean up test file

		})
	}
}

func TestLoadTaskFile_Recurrence(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		fileContent string
		expectValue taskpkg.Recurrence
		shouldLoad  bool
	}{
		{
			name: "valid recurrence daily",
			fileContent: `---
title: Test Task
type: story
status: backlog
recurrence: "0 0 * * *"
---
Task description`,
			expectValue: taskpkg.RecurrenceDaily,
			shouldLoad:  true,
		},
		{
			name: "valid recurrence weekly",
			fileContent: `---
title: Test Task
type: story
status: backlog
recurrence: "0 0 * * MON"
---
Task description`,
			expectValue: "0 0 * * MON",
			shouldLoad:  true,
		},
		{
			name: "missing recurrence field",
			fileContent: `---
title: Test Task
type: story
status: backlog
---
Task description`,
			expectValue: taskpkg.RecurrenceNone,
			shouldLoad:  true,
		},
		{
			name: "invalid recurrence defaults to empty",
			fileContent: `---
title: Test Task
type: story
status: backlog
recurrence: "every tuesday"
---
Task description`,
			expectValue: taskpkg.RecurrenceNone,
			shouldLoad:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "TIKI-REC001.md")
			if err := os.WriteFile(testFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			store, err := NewTikiStore(tmpDir)
			if err != nil {
				t.Fatalf("NewTikiStore() error = %v", err)
			}

			task := store.GetTask("TIKI-REC001")
			if !tt.shouldLoad {
				if task != nil {
					t.Error("expected nil task, but got one")
				}
				return
			}

			if task == nil {
				t.Fatal("GetTask() returned nil")
			}

			if task.Recurrence != tt.expectValue {
				t.Errorf("recurrence got = %q, expected %q", task.Recurrence, tt.expectValue)
			}

		})
	}
}

func TestSaveTask_Recurrence(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore() error = %v", err)
	}

	tests := []struct {
		name       string
		recurrence taskpkg.Recurrence
		expectInFM bool
	}{
		{
			name:       "with recurrence",
			recurrence: "0 0 * * MON",
			expectInFM: true,
		},
		{
			name:       "without recurrence",
			recurrence: taskpkg.RecurrenceNone,
			expectInFM: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &taskpkg.Task{
				ID:          "TIKI-RECSVR",
				Title:       "Test Save Recurrence",
				Type:        taskpkg.TypeStory,
				Status:      "backlog",
				Priority:    3,
				Recurrence:  tt.recurrence,
				Description: "Test description",
			}

			if err := store.CreateTask(task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			filePath := filepath.Join(tmpDir, "TIKI-RECSVR.md")
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed to read saved file: %v", err)
			}

			fileStr := string(content)
			hasRecLine := strings.Contains(fileStr, "recurrence:")

			if tt.expectInFM && !hasRecLine {
				t.Errorf("expected 'recurrence:' in frontmatter, but not found.\nFile content:\n%s", fileStr)
			}
			if !tt.expectInFM && hasRecLine {
				t.Errorf("did not expect 'recurrence:' in frontmatter (omitempty), but found.\nFile content:\n%s", fileStr)
			}

			// Verify round-trip
			loaded := store.GetTask("TIKI-RECSVR")
			if loaded == nil {
				t.Fatal("GetTask() returned nil")
			}
			if loaded.Recurrence != task.Recurrence {
				t.Errorf("round-trip failed: saved %q, loaded %q", task.Recurrence, loaded.Recurrence)
			}

		})
	}
}

func TestSaveTask_Due(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore() error = %v", err)
	}

	tests := []struct {
		name       string
		dueValue   string // YYYY-MM-DD or empty
		expectInFM bool   // should appear in frontmatter
	}{
		{
			name:       "with due date",
			dueValue:   "2026-03-16",
			expectInFM: true,
		},
		{
			name:       "without due date",
			dueValue:   "",
			expectInFM: false, // omitempty should exclude it
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create task
			var dueTime time.Time
			if tt.dueValue != "" {
				dueTime, _ = time.Parse(taskpkg.DateFormat, tt.dueValue)
			}

			task := &taskpkg.Task{
				ID:          "TIKI-SAVE01",
				Title:       "Test Save",
				Type:        taskpkg.TypeStory,
				Status:      "backlog",
				Priority:    3,
				Due:         dueTime,
				Description: "Test description",
			}

			// Save task
			if err := store.CreateTask(task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			// Read file and check frontmatter
			filePath := filepath.Join(tmpDir, "TIKI-SAVE01.md")
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed to read saved file: %v", err)
			}

			fileStr := string(content)
			hasDueLine := strings.Contains(fileStr, "due:")

			if tt.expectInFM && !hasDueLine {
				t.Errorf("expected 'due:' in frontmatter, but not found.\nFile content:\n%s", fileStr)
			}
			if !tt.expectInFM && hasDueLine {
				t.Errorf("did not expect 'due:' in frontmatter (omitempty), but found.\nFile content:\n%s", fileStr)
			}

			// Verify round-trip
			loaded := store.GetTask("TIKI-SAVE01")
			if loaded == nil {
				t.Fatal("GetTask() returned nil")
			}

			if !loaded.Due.Equal(task.Due) {
				t.Errorf("round-trip failed: saved %v, loaded %v", task.Due, loaded.Due)
			}

			// Clean up

		})
	}
}
