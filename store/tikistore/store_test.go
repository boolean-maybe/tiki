package tikistore

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
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

func TestSearch_MatchesTaskID(t *testing.T) {
	store := &TikiStore{
		tasks: map[string]*taskpkg.Task{
			"TIKI-ABC123": {
				ID:       "TIKI-ABC123",
				Title:    "Unrelated Title",
				Status:   taskpkg.StatusBacklog,
				Priority: 1,
			},
			"TIKI-DEF456": {
				ID:       "TIKI-DEF456",
				Title:    "Another Title",
				Status:   taskpkg.StatusReady,
				Priority: 2,
			},
		},
	}

	results := store.Search("abc123", nil)
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].Task.ID != "TIKI-ABC123" {
		t.Errorf("results[0].Task.ID = %q, want %q", results[0].Task.ID, "TIKI-ABC123")
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

func TestSearch_MatchesTags(t *testing.T) {
	store := &TikiStore{
		tasks: map[string]*taskpkg.Task{
			"TIKI-TAG001": {
				ID:       "TIKI-TAG001",
				Title:    "Tagged Task",
				Status:   taskpkg.StatusBacklog,
				Priority: 1,
				Tags:     []string{"frontend", "ui"},
			},
			"TIKI-TAG002": {
				ID:       "TIKI-TAG002",
				Title:    "Untagged Task",
				Status:   taskpkg.StatusReady,
				Priority: 2,
			},
		},
	}

	results := store.Search("frontend", nil)
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].Task.ID != "TIKI-TAG001" {
		t.Errorf("results[0].Task.ID = %q, want %q", results[0].Task.ID, "TIKI-TAG001")
	}

	results = store.Search("backend", nil)
	if len(results) != 0 {
		t.Fatalf("result count = %d, want 0", len(results))
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
				return
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
				return
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

			filePath := filepath.Join(tmpDir, "tiki-recsvr.md")
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
				return
			}
			if loaded.Recurrence != task.Recurrence {
				t.Errorf("round-trip failed: saved %q, loaded %q", task.Recurrence, loaded.Recurrence)
			}

		})
	}
}

func TestMatchesQuery(t *testing.T) {
	tests := []struct {
		name     string
		task     *taskpkg.Task
		query    string
		expected bool
	}{
		{
			name:     "nil task returns false",
			task:     nil,
			query:    "foo",
			expected: false,
		},
		{
			name:     "empty query returns false",
			task:     &taskpkg.Task{ID: "TIKI-MQ0001", Title: "Hello"},
			query:    "",
			expected: false,
		},
		{
			name:     "match by ID case-insensitive",
			task:     &taskpkg.Task{ID: "TIKI-ABC123"},
			query:    "tiki-abc",
			expected: true,
		},
		{
			name:     "match by title",
			task:     &taskpkg.Task{ID: "TIKI-MQ0002", Title: "Hello World"},
			query:    "hello",
			expected: true,
		},
		{
			name:     "match by description",
			task:     &taskpkg.Task{ID: "TIKI-MQ0003", Description: "some text here"},
			query:    "some",
			expected: true,
		},
		{
			name:     "match by first tag",
			task:     &taskpkg.Task{ID: "TIKI-MQ0004", Tags: []string{"frontend"}},
			query:    "frontend",
			expected: true,
		},
		{
			name:     "match by second tag",
			task:     &taskpkg.Task{ID: "TIKI-MQ0005", Tags: []string{"a", "backend"}},
			query:    "backend",
			expected: true,
		},
		{
			name:     "no match",
			task:     &taskpkg.Task{ID: "TIKI-X", Title: "foo"},
			query:    "zzz",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesQuery(tt.task, tt.query)
			if got != tt.expected {
				t.Errorf("matchesQuery() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSearch_WithFilterFunc(t *testing.T) {
	buildStore := func() *TikiStore {
		return &TikiStore{
			tasks: map[string]*taskpkg.Task{
				"TIKI-F00001": {ID: "TIKI-F00001", Title: "Alpha", Priority: 1},
				"TIKI-F00002": {ID: "TIKI-F00002", Title: "Beta", Priority: 2},
				"TIKI-F00003": {ID: "TIKI-F00003", Title: "Gamma", Priority: 3},
			},
		}
	}

	t.Run("filter excludes all returns empty", func(t *testing.T) {
		s := buildStore()
		results := s.Search("", func(*taskpkg.Task) bool { return false })
		if len(results) != 0 {
			t.Errorf("got %d results, want 0", len(results))
		}
	})

	t.Run("filter includes subset returns subset", func(t *testing.T) {
		s := buildStore()
		results := s.Search("", func(t *taskpkg.Task) bool { return t.ID == "TIKI-F00001" })
		if len(results) != 1 {
			t.Errorf("got %d results, want 1", len(results))
		}
		if results[0].Task.ID != "TIKI-F00001" {
			t.Errorf("got ID %q, want TIKI-F00001", results[0].Task.ID)
		}
	})

	t.Run("filter + query intersection", func(t *testing.T) {
		s := buildStore()
		// filter allows F00001 and F00002, query matches only "Beta"
		results := s.Search("beta", func(t *taskpkg.Task) bool {
			return t.ID == "TIKI-F00001" || t.ID == "TIKI-F00002"
		})
		if len(results) != 1 {
			t.Errorf("got %d results, want 1", len(results))
		}
		if results[0].Task.ID != "TIKI-F00002" {
			t.Errorf("got ID %q, want TIKI-F00002", results[0].Task.ID)
		}
	})

	t.Run("nil filter + empty query returns all tasks", func(t *testing.T) {
		s := &TikiStore{
			tasks: map[string]*taskpkg.Task{
				"TIKI-G00001": {ID: "TIKI-G00001", Title: "One"},
				"TIKI-G00002": {ID: "TIKI-G00002", Title: "Two"},
			},
		}
		results := s.Search("", nil)
		if len(results) != 2 {
			t.Errorf("got %d results, want 2", len(results))
		}
	})
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
			filePath := filepath.Join(tmpDir, "tiki-save01.md")
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

func TestCustomFieldRoundTrip(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	// register custom fields
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium", "high"}},
		{Name: "score", Type: workflow.TypeInt},
		{Name: "active", Type: workflow.TypeBool},
		{Name: "notes", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("register custom fields: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	tmpDir := t.TempDir()
	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	original := &taskpkg.Task{
		ID:       "TIKI-CUSTOM",
		Title:    "Custom field test",
		Status:   taskpkg.StatusReady,
		Type:     "story",
		Priority: 2,
		CustomFields: map[string]interface{}{
			"severity": "high",
			"score":    42,
			"active":   true,
			"notes":    "some notes here",
		},
	}

	// save
	if err := store.saveTask(original); err != nil {
		t.Fatalf("saveTask: %v", err)
	}

	// reload
	path := store.taskFilePath(original.ID)
	loaded, err := store.loadTaskFile(path, nil, nil)
	if err != nil {
		t.Fatalf("loadTaskFile: %v", err)
	}

	// verify custom fields survived
	if loaded.CustomFields == nil {
		t.Fatal("CustomFields is nil after reload")
	}
	if loaded.CustomFields["severity"] != "high" {
		t.Errorf("severity = %v, want high", loaded.CustomFields["severity"])
	}
	if loaded.CustomFields["score"] != 42 {
		t.Errorf("score = %v, want 42", loaded.CustomFields["score"])
	}
	if loaded.CustomFields["active"] != true {
		t.Errorf("active = %v, want true", loaded.CustomFields["active"])
	}
	if loaded.CustomFields["notes"] != "some notes here" {
		t.Errorf("notes = %v, want 'some notes here'", loaded.CustomFields["notes"])
	}

	// verify built-in fields are also correct
	if loaded.Title != "Custom field test" {
		t.Errorf("title = %q, want %q", loaded.Title, "Custom field test")
	}
	if loaded.Priority != 2 {
		t.Errorf("priority = %d, want 2", loaded.Priority)
	}
}

func TestCustomFieldRoundTrip_AmbiguousStrings(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "notes", Type: workflow.TypeString},
		{Name: "labels", Type: workflow.TypeListString},
		{Name: "yesno", Type: workflow.TypeEnum, AllowedValues: []string{"true", "false", "maybe"}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	tests := []struct {
		name   string
		fields map[string]interface{}
		check  func(t *testing.T, cf map[string]interface{})
	}{
		{
			name:   "string that looks like bool",
			fields: map[string]interface{}{"notes": "true"},
			check: func(t *testing.T, cf map[string]interface{}) {
				if v, ok := cf["notes"].(string); !ok || v != "true" {
					t.Errorf("notes = %v (%T), want string \"true\"", cf["notes"], cf["notes"])
				}
			},
		},
		{
			name:   "string that looks like date",
			fields: map[string]interface{}{"notes": "2026-05-15"},
			check: func(t *testing.T, cf map[string]interface{}) {
				if v, ok := cf["notes"].(string); !ok || v != "2026-05-15" {
					t.Errorf("notes = %v (%T), want string \"2026-05-15\"", cf["notes"], cf["notes"])
				}
			},
		},
		{
			name:   "string with colon",
			fields: map[string]interface{}{"notes": "a: b"},
			check: func(t *testing.T, cf map[string]interface{}) {
				if v, ok := cf["notes"].(string); !ok || v != "a: b" {
					t.Errorf("notes = %v (%T), want string \"a: b\"", cf["notes"], cf["notes"])
				}
			},
		},
		{
			name:   "string that looks like int",
			fields: map[string]interface{}{"notes": "42"},
			check: func(t *testing.T, cf map[string]interface{}) {
				if v, ok := cf["notes"].(string); !ok || v != "42" {
					t.Errorf("notes = %v (%T), want string \"42\"", cf["notes"], cf["notes"])
				}
			},
		},
		{
			name:   "list with ambiguous items",
			fields: map[string]interface{}{"labels": []string{"true", "42", "2026-05-15"}},
			check: func(t *testing.T, cf map[string]interface{}) {
				want := []string{"true", "42", "2026-05-15"}
				got, ok := cf["labels"].([]string)
				if !ok {
					t.Fatalf("labels type = %T, want []string", cf["labels"])
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("labels = %v, want %v", got, want)
				}
			},
		},
		{
			name:   "enum value that looks like bool",
			fields: map[string]interface{}{"yesno": "true"},
			check: func(t *testing.T, cf map[string]interface{}) {
				if v, ok := cf["yesno"].(string); !ok || v != "true" {
					t.Errorf("yesno = %v (%T), want string \"true\"", cf["yesno"], cf["yesno"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store, err := NewTikiStore(tmpDir)
			if err != nil {
				t.Fatalf("NewTikiStore: %v", err)
			}

			original := &taskpkg.Task{
				ID:           "TIKI-AMBIG1",
				Title:        "Ambiguous round-trip",
				Status:       taskpkg.StatusReady,
				Type:         "story",
				Priority:     2,
				CustomFields: tt.fields,
			}

			if err := store.saveTask(original); err != nil {
				t.Fatalf("saveTask: %v", err)
			}

			path := store.taskFilePath(original.ID)
			loaded, err := store.loadTaskFile(path, nil, nil)
			if err != nil {
				t.Fatalf("loadTaskFile: %v", err)
			}
			if loaded.CustomFields == nil {
				t.Fatal("CustomFields is nil after reload")
			}
			tt.check(t, loaded.CustomFields)
		})
	}
}

func TestExtractCustomFields_PreservesStaleKeys(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium", "high"}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	fmMap := map[string]interface{}{
		"title":         "Test",
		"status":        "ready",
		"severity":      "high",
		"removed_field": "old_value",
	}

	custom, unknown, err := extractCustomFields(fmMap, "test.md")
	if err != nil {
		t.Fatalf("extractCustomFields returned error: %v", err)
	}
	if custom["severity"] != "high" {
		t.Errorf("severity = %v, want high", custom["severity"])
	}
	if _, exists := custom["removed_field"]; exists {
		t.Error("stale key 'removed_field' should not appear in custom fields")
	}
	if unknown["removed_field"] != "old_value" {
		t.Errorf("unknown[removed_field] = %v, want old_value", unknown["removed_field"])
	}
}

func TestLoadTaskFile_StaleCustomField(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	// register only "severity" — the file will also contain "old_field"
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium", "high"}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	tmpDir := t.TempDir()
	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// write a file with a stale custom field
	content := `---
title: Stale field test
type: story
status: ready
priority: 2
severity: high
old_field: leftover_value
---
Description here`

	filePath := filepath.Join(tmpDir, "tiki-stale1.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	loaded, err := store.loadTaskFile(filePath, nil, nil)
	if err != nil {
		t.Fatalf("loadTaskFile should succeed with stale field, got: %v", err)
	}
	if loaded.ID != "TIKI-STALE1" {
		t.Errorf("ID = %q, want TIKI-STALE1", loaded.ID)
	}
	if loaded.CustomFields == nil || loaded.CustomFields["severity"] != "high" {
		t.Errorf("severity = %v, want high", loaded.CustomFields["severity"])
	}
	if _, exists := loaded.CustomFields["old_field"]; exists {
		t.Error("stale key 'old_field' should not appear in CustomFields")
	}
	if loaded.UnknownFields == nil || loaded.UnknownFields["old_field"] != "leftover_value" {
		t.Errorf("UnknownFields[old_field] = %v, want leftover_value", loaded.UnknownFields["old_field"])
	}
}

func TestExtractCustomFields_StaleEnumValueDemotedToUnknown(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	// register severity with only "low" and "medium" — "high" was removed
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium"}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	fmMap := map[string]interface{}{
		"title":    "Test",
		"status":   "ready",
		"severity": "high", // stale value no longer in allowed values
	}

	custom, unknown, err := extractCustomFields(fmMap, "test.md")
	if err != nil {
		t.Fatalf("extractCustomFields should not error on stale enum value, got: %v", err)
	}
	if _, exists := custom["severity"]; exists {
		t.Error("stale enum value should not appear in custom fields")
	}
	if unknown == nil || unknown["severity"] != "high" {
		t.Errorf("unknown[severity] = %v, want 'high' (preserved for repair)", unknown["severity"])
	}
}

func TestLoadTaskFile_StaleEnumValue_TaskStillLoads(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	// register severity without "critical"
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium", "high"}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	tmpDir := t.TempDir()
	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	content := `---
title: Task with stale enum
type: story
status: ready
priority: 2
severity: critical
---
Description`

	filePath := filepath.Join(tmpDir, "tiki-stale2.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	loaded, err := store.loadTaskFile(filePath, nil, nil)
	if err != nil {
		t.Fatalf("loadTaskFile should succeed with stale enum value, got: %v", err)
	}
	if loaded.ID != "TIKI-STALE2" {
		t.Errorf("ID = %q, want TIKI-STALE2", loaded.ID)
	}
	// stale value should be in UnknownFields, not CustomFields
	if _, exists := loaded.CustomFields["severity"]; exists {
		t.Error("stale enum value should not be in CustomFields")
	}
	if loaded.UnknownFields == nil || loaded.UnknownFields["severity"] != "critical" {
		t.Errorf("UnknownFields[severity] = %v, want 'critical'", loaded.UnknownFields["severity"])
	}
}

func TestCoerceCustomValue_IntRejectsFractional(t *testing.T) {
	fd := workflow.FieldDef{Name: "score", Type: workflow.TypeInt}

	_, err := coerceCustomValue(fd, 1.5)
	if err == nil {
		t.Fatal("expected error for fractional float, got nil")
	}
	if !strings.Contains(err.Error(), "not a whole number") {
		t.Errorf("error = %q, want substring %q", err.Error(), "not a whole number")
	}
}

func TestCoerceCustomValue_IntAcceptsWholeFloat(t *testing.T) {
	fd := workflow.FieldDef{Name: "score", Type: workflow.TypeInt}

	val, err := coerceCustomValue(fd, 3.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 3 {
		t.Errorf("val = %v, want 3", val)
	}
}

func TestExtractCustomFields_ErrorWithoutRegistry(t *testing.T) {
	config.ResetRegistriesLoadedForTest()
	t.Cleanup(func() {
		config.MarkRegistriesLoadedForTest()
		workflow.ClearCustomFields()
	})

	fmMap := map[string]interface{}{
		"title":    "Test",
		"status":   "ready",
		"severity": "high",
	}

	_, _, err := extractCustomFields(fmMap, "test.md")
	if err == nil {
		t.Fatal("expected error when registries not loaded, got nil")
	}
	if !strings.Contains(err.Error(), "workflow registries not loaded") {
		t.Errorf("error = %q, want substring %q", err.Error(), "workflow registries not loaded")
	}
}

func TestExtractCustomFields_OnlyBuiltinKeys_NoRegistryRequired(t *testing.T) {
	config.ResetRegistriesLoadedForTest()
	t.Cleanup(func() {
		config.MarkRegistriesLoadedForTest()
		workflow.ClearCustomFields()
	})

	// frontmatter with only built-in keys — should not require registry
	fmMap := map[string]interface{}{
		"title":    "Test",
		"status":   "ready",
		"priority": 2,
	}

	custom, unknown, err := extractCustomFields(fmMap, "test.md")
	if err != nil {
		t.Fatalf("extractCustomFields should succeed with only built-in keys: %v", err)
	}
	if len(custom) != 0 {
		t.Errorf("expected no custom fields, got %v", custom)
	}
	if len(unknown) != 0 {
		t.Errorf("expected no unknown fields, got %v", unknown)
	}
}

func TestExtractCustomFields_NilMap_NoRegistryRequired(t *testing.T) {
	config.ResetRegistriesLoadedForTest()
	t.Cleanup(func() {
		config.MarkRegistriesLoadedForTest()
		workflow.ClearCustomFields()
	})

	custom, unknown, err := extractCustomFields(nil, "test.md")
	if err != nil {
		t.Fatalf("extractCustomFields(nil) should succeed: %v", err)
	}
	if custom != nil || unknown != nil {
		t.Errorf("expected nils for nil input, got custom=%v unknown=%v", custom, unknown)
	}
}

func TestSaveTask_PreservesUnknownFields(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium", "high"}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	tmpDir := t.TempDir()
	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// write a file with a known custom field and an unknown field
	content := "---\ntitle: Roundtrip test\ntype: story\nstatus: ready\npriority: 2\npoints: 3\nseverity: high\nold_field: leftover\n---\nBody text"
	filePath := filepath.Join(tmpDir, "tiki-round1.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// load, then save without modification
	loaded, err := store.loadTaskFile(filePath, nil, nil)
	if err != nil {
		t.Fatalf("loadTaskFile: %v", err)
	}
	if err := store.saveTask(loaded); err != nil {
		t.Fatalf("saveTask: %v", err)
	}

	// re-read the file and verify the unknown field survived
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	fileContent := string(data)
	if !strings.Contains(fileContent, "old_field: leftover") {
		t.Errorf("unknown field lost after round-trip:\n%s", fileContent)
	}
	if !strings.Contains(fileContent, "severity: high") {
		t.Errorf("custom field lost after round-trip:\n%s", fileContent)
	}
}
