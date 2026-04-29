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
				{ID: "ABC123", Title: "Zebra Task", Priority: 2},
				{ID: "DEF456", Title: "Alpha Task", Priority: 1},
				{ID: "GHI789", Title: "Beta Task", Priority: 1},
			},
			expected: []string{"DEF456", "GHI789", "ABC123"}, // Alpha, Beta (both P1), then Zebra (P2)
		},
		{
			name: "same priority - alphabetical by title",
			tasks: []*taskpkg.Task{
				{ID: "ABC10Z", Title: "Zebra", Priority: 3},
				{ID: "ABC2ZZ", Title: "Apple", Priority: 3},
				{ID: "ABC1ZZ", Title: "Mango", Priority: 3},
			},
			expected: []string{"ABC2ZZ", "ABC1ZZ", "ABC10Z"}, // Apple, Mango, Zebra
		},
		{
			name: "same priority and title - tiebreak by ID",
			tasks: []*taskpkg.Task{
				{ID: "CCC333", Title: "Same", Priority: 2},
				{ID: "AAA111", Title: "Same", Priority: 2},
				{ID: "BBB222", Title: "Same", Priority: 2},
			},
			expected: []string{"AAA111", "BBB222", "CCC333"},
		},
		{
			name:     "empty task list",
			tasks:    []*taskpkg.Task{},
			expected: []string{},
		},
		{
			name: "single task",
			tasks: []*taskpkg.Task{
				{ID: "ABC1ZZ", Title: "Only Task", Priority: 3},
			},
			expected: []string{"ABC1ZZ"},
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
			"ABC123": {
				ID:         "ABC123",
				Title:      "Unrelated Title",
				Status:     taskpkg.StatusBacklog,
				Priority:   1,
				IsWorkflow: true,
			},
			"DEF456": {
				ID:         "DEF456",
				Title:      "Another Title",
				Status:     taskpkg.StatusReady,
				Priority:   2,
				IsWorkflow: true,
			},
		},
	}

	results := store.Search("abc123", nil)
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].Task.ID != "ABC123" {
		t.Errorf("results[0].Task.ID = %q, want %q", results[0].Task.ID, "ABC123")
	}
}

func TestSearch_AllTasksIncludesDescription(t *testing.T) {
	store := &TikiStore{
		tasks: map[string]*taskpkg.Task{
			"AAA111": {
				ID:          "AAA111",
				Title:       "Alpha Task",
				Description: "Contains the keyword needle",
				Status:      taskpkg.StatusBacklog,
				Priority:    2,
				IsWorkflow:  true,
			},
			"BBB222": {
				ID:          "BBB222",
				Title:       "Beta Task",
				Description: "No match here",
				Status:      taskpkg.StatusReady,
				Priority:    1,
				IsWorkflow:  true,
			},
			"CCC333": {
				ID:          "CCC333",
				Title:       "Gamma Task",
				Description: "Another needle appears",
				Status:      taskpkg.StatusReview,
				Priority:    3,
				IsWorkflow:  true,
			},
		},
	}

	results := store.Search("needle", nil)
	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}

	expectedIDs := []string{"AAA111", "CCC333"} // sorted by priority then title
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
			"TAG001": {
				ID:         "TAG001",
				Title:      "Tagged Task",
				Status:     taskpkg.StatusBacklog,
				Priority:   1,
				Tags:       []string{"frontend", "ui"},
				IsWorkflow: true,
			},
			"TAG002": {
				ID:         "TAG002",
				Title:      "Untagged Task",
				Status:     taskpkg.StatusReady,
				Priority:   2,
				IsWorkflow: true,
			},
		},
	}

	results := store.Search("frontend", nil)
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].Task.ID != "TAG001" {
		t.Errorf("results[0].Task.ID = %q, want %q", results[0].Task.ID, "TAG001")
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
id: ABC123
title: Test Task
type: story
status: backlog
dependsOn:
  - ABC123
  - DEF456
---
Task description`,
			expectedDependsOn: []string{"ABC123", "DEF456"},
			shouldLoad:        true,
		},
		{
			name: "lowercase IDs uppercased",
			fileContent: `---
id: ABC123
title: Test Task
type: story
status: backlog
dependsOn:
  - abc123
---
Task description`,
			expectedDependsOn: []string{"ABC123"},
			shouldLoad:        true,
		},
		{
			name: "duplicate dependencies deduped",
			fileContent: `---
id: ABC123
title: Test Task
type: story
status: backlog
dependsOn:
  - ABC123
  - abc123
  - DEF456
---
Task description`,
			expectedDependsOn: []string{"ABC123", "DEF456"},
			shouldLoad:        true,
		},
		{
			name: "missing dependsOn field",
			fileContent: `---
id: ABC123
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
id: ABC123
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
id: ABC123
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
id: ABC123
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
id: ABC123
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
id: ABC123
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
id: ABC123
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
id: ABC123
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
id: ABC123
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
id: ABC123
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
id: ABC123
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
		{
			name: "duplicate tags deduped",
			fileContent: `---
id: ABC123
title: Test Task
type: story
status: backlog
tags:
  - frontend
  - backend
  - frontend
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
id: TEST01
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
id: TEST01
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
id: TEST01
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
id: TEST01
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
id: TEST01
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
id: TEST01
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
			testFile := filepath.Join(tmpDir, "TEST01.md")
			if err := os.WriteFile(testFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			// Load store
			store, err := NewTikiStore(tmpDir)
			if err != nil {
				t.Fatalf("NewTikiStore() error = %v", err)
			}

			// Get task
			task := store.GetTask("TEST01")
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
id: REC001
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
id: REC001
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
id: REC001
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
id: REC001
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
			testFile := filepath.Join(tmpDir, "REC001.md")
			if err := os.WriteFile(testFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			store, err := NewTikiStore(tmpDir)
			if err != nil {
				t.Fatalf("NewTikiStore() error = %v", err)
			}

			task := store.GetTask("REC001")
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
				ID:          "RECSVR",
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

			filePath := filepath.Join(tmpDir, "RECSVR.md")
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
			loaded := store.GetTask("RECSVR")
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
			task:     &taskpkg.Task{ID: "MQ0001", Title: "Hello"},
			query:    "",
			expected: false,
		},
		{
			name:     "match by ID case-insensitive",
			task:     &taskpkg.Task{ID: "ABC123"},
			query:    "abc",
			expected: true,
		},
		{
			name:     "match by title",
			task:     &taskpkg.Task{ID: "MQ0002", Title: "Hello World"},
			query:    "hello",
			expected: true,
		},
		{
			name:     "match by description",
			task:     &taskpkg.Task{ID: "MQ0003", Description: "some text here"},
			query:    "some",
			expected: true,
		},
		{
			name:     "match by first tag",
			task:     &taskpkg.Task{ID: "MQ0004", Tags: []string{"frontend"}},
			query:    "frontend",
			expected: true,
		},
		{
			name:     "match by second tag",
			task:     &taskpkg.Task{ID: "MQ0005", Tags: []string{"a", "backend"}},
			query:    "backend",
			expected: true,
		},
		{
			name:     "no match",
			task:     &taskpkg.Task{ID: "X", Title: "foo"},
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
				"F00001": {ID: "F00001", Title: "Alpha", Priority: 1},
				"F00002": {ID: "F00002", Title: "Beta", Priority: 2},
				"F00003": {ID: "F00003", Title: "Gamma", Priority: 3},
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
		results := s.Search("", func(t *taskpkg.Task) bool { return t.ID == "F00001" })
		if len(results) != 1 {
			t.Errorf("got %d results, want 1", len(results))
		}
		if results[0].Task.ID != "F00001" {
			t.Errorf("got ID %q, want F00001", results[0].Task.ID)
		}
	})

	t.Run("filter + query intersection", func(t *testing.T) {
		s := buildStore()
		// filter allows F00001 and F00002, query matches only "Beta"
		results := s.Search("beta", func(t *taskpkg.Task) bool {
			return t.ID == "F00001" || t.ID == "F00002"
		})
		if len(results) != 1 {
			t.Errorf("got %d results, want 1", len(results))
		}
		if results[0].Task.ID != "F00002" {
			t.Errorf("got ID %q, want F00002", results[0].Task.ID)
		}
	})

	t.Run("nil filter + empty query returns all workflow tasks", func(t *testing.T) {
		s := &TikiStore{
			tasks: map[string]*taskpkg.Task{
				"G00001": {ID: "G00001", Title: "One", IsWorkflow: true},
				"G00002": {ID: "G00002", Title: "Two", IsWorkflow: true},
			},
		}
		results := s.Search("", nil)
		if len(results) != 2 {
			t.Errorf("got %d results, want 2", len(results))
		}
	})

	// L4 regression: a plain doc (IsWorkflow=false) must NOT be returned
	// when filterFunc is nil. Callers that want plain docs pass an explicit
	// filter.
	t.Run("nil filter excludes plain docs", func(t *testing.T) {
		s := &TikiStore{
			tasks: map[string]*taskpkg.Task{
				"WORK01": {ID: "WORK01", Title: "workflow item", IsWorkflow: true},
				"PLAIN1": {ID: "PLAIN1", Title: "plain doc", IsWorkflow: false},
			},
		}
		results := s.Search("", nil)
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		if results[0].Task.ID != "WORK01" {
			t.Errorf("expected WORK01, got %q — plain doc leaked through nil filter", results[0].Task.ID)
		}

		// non-nil filter → caller is trusted, plain doc is included.
		all := s.Search("", func(*taskpkg.Task) bool { return true })
		if len(all) != 2 {
			t.Errorf("explicit filter got %d, want 2 (both plain + workflow)", len(all))
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
				ID:          "SAVE01",
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
			filePath := filepath.Join(tmpDir, "SAVE01.md")
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
			loaded := store.GetTask("SAVE01")
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
		ID:         "CUSTOM",
		Title:      "Custom field test",
		Status:     taskpkg.StatusReady,
		Type:       "story",
		Priority:   2,
		IsWorkflow: true,
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
				ID:           "AMBIG1",
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
id: STALE1
title: Stale field test
type: story
status: ready
priority: 2
severity: high
old_field: leftover_value
---
Description here`

	filePath := filepath.Join(tmpDir, "STALE1.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	loaded, err := store.loadTaskFile(filePath, nil, nil)
	if err != nil {
		t.Fatalf("loadTaskFile should succeed with stale field, got: %v", err)
	}
	if loaded.ID != "STALE1" {
		t.Errorf("ID = %q, want STALE1", loaded.ID)
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
id: STALE2
title: Task with stale enum
type: story
status: ready
priority: 2
severity: critical
---
Description`

	filePath := filepath.Join(tmpDir, "STALE2.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	loaded, err := store.loadTaskFile(filePath, nil, nil)
	if err != nil {
		t.Fatalf("loadTaskFile should succeed with stale enum value, got: %v", err)
	}
	if loaded.ID != "STALE2" {
		t.Errorf("ID = %q, want STALE2", loaded.ID)
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
	content := "---\nid: ROUND1\ntitle: Roundtrip test\ntype: story\nstatus: ready\npriority: 2\npoints: 3\nseverity: high\nold_field: leftover\n---\nBody text"
	filePath := filepath.Join(tmpDir, "ROUND1.md")
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

func TestSaveTask_DedupesBuiltInCollections(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	input := &taskpkg.Task{
		ID:          "SET001",
		Title:       "dedupe built-ins",
		Type:        taskpkg.TypeStory,
		Status:      taskpkg.StatusBacklog,
		Priority:    3,
		Tags:        []string{"frontend", "backend", "frontend", " backend "},
		DependsOn:   []string{"aaa001", "AAA001", " BBB002 "},
		Description: "body",
		IsWorkflow:  true,
	}

	if err := store.saveTask(input); err != nil {
		t.Fatalf("saveTask: %v", err)
	}

	path := store.taskFilePath(input.ID)
	loaded, err := store.loadTaskFile(path, nil, nil)
	if err != nil {
		t.Fatalf("loadTaskFile: %v", err)
	}

	if !reflect.DeepEqual(loaded.Tags, []string{"backend", "frontend"}) {
		t.Errorf("loaded tags = %v, want [backend frontend]", loaded.Tags)
	}
	if !reflect.DeepEqual(loaded.DependsOn, []string{"AAA001", "BBB002"}) {
		t.Errorf("loaded dependsOn = %v, want [AAA001 BBB002]", loaded.DependsOn)
	}
}

func TestSaveTask_DedupesCustomListFields(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "labels", Type: workflow.TypeListString},
		{Name: "related", Type: workflow.TypeListRef},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	tmpDir := t.TempDir()
	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	input := &taskpkg.Task{
		ID:       "SET002",
		Title:    "dedupe custom",
		Type:     taskpkg.TypeStory,
		Status:   taskpkg.StatusBacklog,
		Priority: 3,
		CustomFields: map[string]interface{}{
			"labels":  []string{"backend", "backend", " frontend ", ""},
			"related": []string{"aaa001", "AAA001", "bbb002"},
		},
	}

	if err := store.saveTask(input); err != nil {
		t.Fatalf("saveTask: %v", err)
	}

	path := store.taskFilePath(input.ID)
	loaded, err := store.loadTaskFile(path, nil, nil)
	if err != nil {
		t.Fatalf("loadTaskFile: %v", err)
	}

	labels, ok := loaded.CustomFields["labels"].([]string)
	if !ok {
		t.Fatalf("labels type = %T, want []string", loaded.CustomFields["labels"])
	}
	if !reflect.DeepEqual(labels, []string{"backend", "frontend"}) {
		t.Errorf("labels = %v, want [backend frontend]", labels)
	}

	related, ok := loaded.CustomFields["related"].([]string)
	if !ok {
		t.Fatalf("related type = %T, want []string", loaded.CustomFields["related"])
	}
	if !reflect.DeepEqual(related, []string{"AAA001", "BBB002"}) {
		t.Errorf("related = %v, want [AAA001 BBB002]", related)
	}
}

func TestLoadTaskFile_FilePathAbsolute(t *testing.T) {
	tmpDir := t.TempDir()
	fileName := "FP0001.md"
	content := `---
id: ABC123
title: Filepath Test
type: story
status: backlog
---
body`
	testFile := filepath.Join(tmpDir, fileName)
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// load with a relative path to prove loadTaskFile still resolves to absolute
	rel, err := filepath.Rel(filepath.Dir(tmpDir), testFile)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()
	if err := os.Chdir(filepath.Dir(tmpDir)); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	tk, err := store.loadTaskFile(rel, nil, nil)
	if err != nil {
		t.Fatalf("loadTaskFile: %v", err)
	}
	if !filepath.IsAbs(tk.FilePath) {
		t.Errorf("FilePath is not absolute: %q", tk.FilePath)
	}
	if !strings.HasSuffix(tk.FilePath, fileName) {
		t.Errorf("FilePath does not end with expected filename: %q", tk.FilePath)
	}
}

func TestLoadSave_DropsStaleFilepathFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()

	// seed a file that already contains a stale filepath: key in frontmatter
	fileName := "FP0003.md"
	testFile := filepath.Join(tmpDir, fileName)
	stale := `---
id: FP0003
title: Stale filepath
type: story
status: backlog
priority: 3
filepath: /stale/path/FP0003.md
---
body`
	if err := os.WriteFile(testFile, []byte(stale), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk := store.GetTask("FP0003")
	if tk == nil {
		t.Fatal("GetTask returned nil")
	}
	// loaded FilePath must be the real absolute path, not the stale string
	expectedAbs, _ := filepath.Abs(testFile)
	if tk.FilePath != expectedAbs {
		t.Errorf("FilePath = %q, want %q (real path, not stale)", tk.FilePath, expectedAbs)
	}
	// stale key must not leak into unknownFields
	if _, exists := tk.UnknownFields["filepath"]; exists {
		t.Errorf("stale filepath should not survive in UnknownFields, got %v", tk.UnknownFields["filepath"])
	}

	// save and re-read the file: stale key must be gone
	if err := store.UpdateTask(tk); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(content), "filepath:") {
		t.Errorf("stale 'filepath:' still present after save:\n%s", content)
	}
}

func TestSaveTask_FilePathRefreshedAndNotSerialized(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewTikiStore(tmpDir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk := &taskpkg.Task{
		ID:       "FP0002",
		Title:    "Save Filepath Test",
		Type:     taskpkg.TypeStory,
		Status:   "backlog",
		Priority: 3,
	}
	if err := store.CreateTask(tk); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if tk.FilePath == "" {
		t.Fatal("FilePath not set after CreateTask")
	}
	if !filepath.IsAbs(tk.FilePath) {
		t.Errorf("FilePath is not absolute: %q", tk.FilePath)
	}
	expectedPath := filepath.Join(tmpDir, "FP0002.md")
	expectedAbs, _ := filepath.Abs(expectedPath)
	if tk.FilePath != expectedAbs {
		t.Errorf("FilePath = %q, want %q", tk.FilePath, expectedAbs)
	}

	// verify filepath is NOT serialized to YAML frontmatter
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if strings.Contains(string(content), "filepath:") {
		t.Errorf("saved file should not contain 'filepath:' frontmatter key:\n%s", content)
	}
}
