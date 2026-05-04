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
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// makeTikiMap builds a map[string]*tikipkg.Tiki for direct injection into
// TikiStore.tikis in tests.
func makeTikiMap(tikis ...*tikipkg.Tiki) map[string]*tikipkg.Tiki {
	out := make(map[string]*tikipkg.Tiki, len(tikis))
	for _, tk := range tikis {
		out[tk.ID] = tk
	}
	return out
}

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
			taskpkg.Sort(tt.tasks)

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

func TestSearchTikis_MatchesID(t *testing.T) {
	mkWF := func(id, title, status string, priority int) *tikipkg.Tiki {
		tk := tikipkg.New()
		tk.ID = id
		tk.Title = title
		tk.Set(tikipkg.FieldStatus, status)
		tk.Set(tikipkg.FieldPriority, priority)
		return tk
	}
	store := &TikiStore{
		tikis: makeTikiMap(
			mkWF("ABC123", "Unrelated Title", "backlog", 1),
			mkWF("DEF456", "Another Title", "ready", 2),
		),
	}

	results := store.SearchTikis("abc123", nil)
	if len(results) != 1 {
		t.Fatalf("result count = %d, want 1", len(results))
	}
	if results[0].ID != "ABC123" {
		t.Errorf("results[0].ID = %q, want %q", results[0].ID, "ABC123")
	}
}

func TestSearchTikis_MatchesBody(t *testing.T) {
	mkWF := func(id, title, body, status string, priority int) *tikipkg.Tiki {
		tk := tikipkg.New()
		tk.ID = id
		tk.Title = title
		tk.Body = body
		tk.Set(tikipkg.FieldStatus, status)
		tk.Set(tikipkg.FieldPriority, priority)
		return tk
	}
	// Description maps to Body in tiki model.
	store := &TikiStore{
		tikis: makeTikiMap(
			mkWF("AAA111", "Alpha Task", "Contains the keyword needle", "backlog", 2),
			mkWF("BBB222", "Beta Task", "No match here", "ready", 1),
			mkWF("CCC333", "Gamma Task", "Another needle appears", "review", 3),
		),
	}

	results := store.SearchTikis("needle", nil)
	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}

	// SearchTikis sorts by title ascending.
	expectedIDs := []string{"AAA111", "CCC333"} // Alpha, Gamma
	for i, result := range results {
		if result.ID != expectedIDs[i] {
			t.Errorf("results[%d].ID = %q, want %q", i, result.ID, expectedIDs[i])
		}
	}
}

// TestSearchTikis_MatchesTags verifies the disk-backed store's SearchTikis
// also queries the tags slice — not just id/title/body — so a tag-only
// query surfaces tikis whose other text fields don't mention the term.
func TestSearchTikis_MatchesTags(t *testing.T) {
	mk := func(id, title, body string, tags []string) *tikipkg.Tiki {
		tk := tikipkg.New()
		tk.ID = id
		tk.Title = title
		tk.Body = body
		if tags != nil {
			tk.Set(tikipkg.FieldTags, tags)
		}
		return tk
	}
	store := &TikiStore{
		tikis: makeTikiMap(
			mk("TAG001", "Unrelated Title", "Unrelated body", []string{"backend", "perf"}),
			mk("TAG002", "Different Title", "Different body", []string{"frontend"}),
			mk("TAG003", "Untagged", "no tag", nil),
		),
	}

	results := store.SearchTikis("backend", nil)
	if len(results) != 1 || results[0].ID != "TAG001" {
		ids := make([]string, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}
		t.Fatalf("tag-only query: got %v, want [TAG001]", ids)
	}

	// Case-insensitive substring match on a tag also surfaces the tiki.
	if results := store.SearchTikis("FRONT", nil); len(results) != 1 || results[0].ID != "TAG002" {
		t.Errorf("case-insensitive tag substring did not match: got %d results", len(results))
	}

	// A query that misses every tag, title, body, and id returns nothing.
	if results := store.SearchTikis("nomatch", nil); len(results) != 0 {
		t.Errorf("non-matching query returned results: %v", results)
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

			tk, err := store.loadTikiFile(testFile, nil, nil)

			if tt.shouldLoad {
				if err != nil {
					t.Fatalf("loadTikiFile() unexpected error = %v", err)
				}
				if tk == nil {
					t.Fatal("loadTikiFile() returned nil tiki")
				}

				dependsOn, present, _ := tk.StringSliceField(tikipkg.FieldDependsOn)
				if !present {
					dependsOn = []string{}
				}
				if !reflect.DeepEqual(dependsOn, tt.expectedDependsOn) {
					t.Errorf("dependsOn = %v, expected %v", dependsOn, tt.expectedDependsOn)
				}
			} else {
				if err == nil {
					t.Error("loadTikiFile() expected error but got none")
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

			// Load the tiki file directly
			tk, err := store.loadTikiFile(testFile, nil, nil)

			if tt.shouldLoad {
				if err != nil {
					t.Fatalf("loadTikiFile() unexpected error = %v", err)
				}
				if tk == nil {
					t.Fatal("loadTikiFile() returned nil tiki")
				}

				// Verify tags
				tags, present, _ := tk.StringSliceField(tikipkg.FieldTags)
				if !present {
					tags = []string{}
				}
				if !reflect.DeepEqual(tags, tt.expectedTags) {
					t.Errorf("tags = %v, expected %v", tags, tt.expectedTags)
				}

				// Verify other fields still work
				if tk.Title != "Test Task" {
					t.Errorf("Title = %q, expected %q", tk.Title, "Test Task")
				}
				typeStr, _, _ := tk.StringField(tikipkg.FieldType)
				if typeStr != string(taskpkg.TypeStory) {
					t.Errorf("type = %q, expected %q", typeStr, taskpkg.TypeStory)
				}
				statusStr, _, _ := tk.StringField(tikipkg.FieldStatus)
				if statusStr != string(taskpkg.StatusBacklog) {
					t.Errorf("status = %q, expected %q", statusStr, taskpkg.StatusBacklog)
				}
			} else {
				if err == nil {
					t.Error("loadTikiFile() expected error but got none")
				}
			}

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

			// Get tiki
			tk := store.GetTiki("TEST01")
			if !tt.shouldLoad {
				if tk != nil {
					t.Error("expected nil tiki, but got one")
				}
				return
			}

			if tk == nil {
				t.Fatal("GetTiki() returned nil")
				return
			}

			due, _, _ := tk.TimeField(tikipkg.FieldDue)
			if tt.expectZero {
				if !due.IsZero() {
					t.Errorf("expected zero due time, got = %v", due)
				}
			} else {
				if due.IsZero() {
					t.Error("expected non-zero due time, got zero")
				}
				got := due.Format(taskpkg.DateFormat)
				if got != tt.expectValue {
					t.Errorf("due date got = %v, expected %v", got, tt.expectValue)
				}
			}

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

			tk := store.GetTiki("REC001")
			if !tt.shouldLoad {
				if tk != nil {
					t.Error("expected nil tiki, but got one")
				}
				return
			}

			if tk == nil {
				t.Fatal("GetTiki() returned nil")
				return
			}

			recStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
			if taskpkg.Recurrence(recStr) != tt.expectValue {
				t.Errorf("recurrence got = %q, expected %q", recStr, tt.expectValue)
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
			tk := tikipkg.New()
			tk.ID = "RECSVR"
			tk.Title = "Test Save Recurrence"
			tk.Set("type", string(taskpkg.TypeStory))
			tk.Set("status", "backlog")
			tk.Set("priority", 3)
			tk.Body = "Test description"
			if tt.recurrence != taskpkg.RecurrenceNone {
				tk.Set(tikipkg.FieldRecurrence, string(tt.recurrence))
			}

			if err := store.CreateTiki(tk); err != nil {
				t.Fatalf("CreateTiki() error = %v", err)
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

			// Verify round-trip via tiki API.
			loaded := store.GetTiki("RECSVR")
			if loaded == nil {
				t.Fatal("GetTiki() returned nil")
				return
			}
			recStr, _, _ := loaded.StringField(tikipkg.FieldRecurrence)
			if taskpkg.Recurrence(recStr) != tt.recurrence {
				t.Errorf("round-trip failed: saved %q, loaded %q", tt.recurrence, recStr)
			}

		})
	}
}

func TestMatchesTikiQuery(t *testing.T) {
	tests := []struct {
		name     string
		tiki     *tikipkg.Tiki
		query    string
		expected bool
	}{
		{
			name:     "nil tiki returns false",
			tiki:     nil,
			query:    "foo",
			expected: false,
		},
		{
			name:     "empty query returns false",
			tiki:     &tikipkg.Tiki{ID: "MQ0001", Title: "Hello"},
			query:    "",
			expected: false,
		},
		{
			name: "match by ID case-insensitive",
			tiki: func() *tikipkg.Tiki {
				tk := tikipkg.New()
				tk.ID = "ABC123"
				return tk
			}(),
			query:    "abc",
			expected: true,
		},
		{
			name: "match by title",
			tiki: func() *tikipkg.Tiki {
				tk := tikipkg.New()
				tk.ID = "MQ0002"
				tk.Title = "Hello World"
				return tk
			}(),
			query:    "hello",
			expected: true,
		},
		{
			name: "match by body (description)",
			tiki: func() *tikipkg.Tiki {
				tk := tikipkg.New()
				tk.ID = "MQ0003"
				tk.Body = "some text here"
				return tk
			}(),
			query:    "some",
			expected: true,
		},
		{
			name: "no match",
			tiki: func() *tikipkg.Tiki {
				tk := tikipkg.New()
				tk.ID = "NOMTCH"
				tk.Title = "foo"
				return tk
			}(),
			query:    "zzz",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesTikiQuery(tt.tiki, strings.ToLower(tt.query))
			if got != tt.expected {
				t.Errorf("matchesTikiQuery() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSearchTikis_WithFilterFunc(t *testing.T) {
	mkTiki := func(id, title string, priority int) *tikipkg.Tiki {
		tk := tikipkg.New()
		tk.ID = id
		tk.Title = title
		tk.Set(tikipkg.FieldPriority, priority)
		return tk
	}
	buildStore := func() *TikiStore {
		return &TikiStore{
			tikis: makeTikiMap(
				mkTiki("F00001", "Alpha", 1),
				mkTiki("F00002", "Beta", 2),
				mkTiki("F00003", "Gamma", 3),
			),
		}
	}

	t.Run("filter excludes all returns empty", func(t *testing.T) {
		s := buildStore()
		results := s.SearchTikis("", func(*tikipkg.Tiki) bool { return false })
		if len(results) != 0 {
			t.Errorf("got %d results, want 0", len(results))
		}
	})

	t.Run("filter includes subset returns subset", func(t *testing.T) {
		s := buildStore()
		results := s.SearchTikis("", func(tk *tikipkg.Tiki) bool { return tk.ID == "F00001" })
		if len(results) != 1 {
			t.Errorf("got %d results, want 1", len(results))
		}
		if results[0].ID != "F00001" {
			t.Errorf("got ID %q, want F00001", results[0].ID)
		}
	})

	t.Run("filter + query intersection", func(t *testing.T) {
		s := buildStore()
		// filter allows F00001 and F00002, query matches only "Beta"
		results := s.SearchTikis("beta", func(tk *tikipkg.Tiki) bool {
			return tk.ID == "F00001" || tk.ID == "F00002"
		})
		if len(results) != 1 {
			t.Errorf("got %d results, want 1", len(results))
		}
		if results[0].ID != "F00002" {
			t.Errorf("got ID %q, want F00002", results[0].ID)
		}
	})

	t.Run("nil filter + empty query returns all tikis", func(t *testing.T) {
		mkTiki2 := func(id, title string) *tikipkg.Tiki {
			tk := tikipkg.New()
			tk.ID = id
			tk.Title = title
			return tk
		}
		s := &TikiStore{
			tikis: makeTikiMap(
				mkTiki2("G00001", "One"),
				mkTiki2("G00002", "Two"),
			),
		}
		results := s.SearchTikis("", nil)
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

			tk := tikipkg.New()
			tk.ID = "SAVE01"
			tk.Title = "Test Save"
			tk.Set("type", string(taskpkg.TypeStory))
			tk.Set("status", "backlog")
			tk.Set("priority", 3)
			tk.Body = "Test description"
			if !dueTime.IsZero() {
				tk.Set(tikipkg.FieldDue, dueTime)
			}

			// Save tiki
			if err := store.CreateTiki(tk); err != nil {
				t.Fatalf("CreateTiki() error = %v", err)
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

			// Verify round-trip via tiki API.
			loadedTk := store.GetTiki("SAVE01")
			if loadedTk == nil {
				t.Fatal("GetTiki() returned nil")
			}
			due, _, _ := loadedTk.TimeField(tikipkg.FieldDue)
			if !due.Equal(dueTime) {
				t.Errorf("round-trip failed: saved %v, loaded %v", dueTime, due)
			}

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

	original := tikipkg.New()
	original.ID = "CUSTOM"
	original.Title = "Custom field test"
	original.Set(tikipkg.FieldStatus, string(taskpkg.StatusReady))
	original.Set(tikipkg.FieldType, "story")
	original.Set(tikipkg.FieldPriority, 2)
	original.Set("severity", "high")
	original.Set("score", 42)
	original.Set("active", true)
	original.Set("notes", "some notes here")

	// save
	if err := store.saveTiki(original); err != nil {
		t.Fatalf("saveTiki: %v", err)
	}

	// reload
	path := store.taskFilePath(original.ID)
	loaded, err := store.loadTikiFile(path, nil, nil)
	if err != nil {
		t.Fatalf("loadTikiFile: %v", err)
	}

	// verify custom fields survived
	severity, hasSeverity, _ := loaded.StringField("severity")
	if !hasSeverity || severity != "high" {
		t.Errorf("severity = %v, want high", loaded.Fields["severity"])
	}
	score, hasScore, _ := loaded.IntField("score")
	if !hasScore || score != 42 {
		t.Errorf("score = %v, want 42", loaded.Fields["score"])
	}
	active, hasActive := loaded.Get("active")
	if !hasActive || active != true {
		t.Errorf("active = %v, want true", loaded.Fields["active"])
	}
	notes, hasNotes, _ := loaded.StringField("notes")
	if !hasNotes || notes != "some notes here" {
		t.Errorf("notes = %v, want 'some notes here'", loaded.Fields["notes"])
	}

	// verify built-in fields are also correct
	if loaded.Title != "Custom field test" {
		t.Errorf("title = %q, want %q", loaded.Title, "Custom field test")
	}
	priority, _, _ := loaded.IntField(tikipkg.FieldPriority)
	if priority != 2 {
		t.Errorf("priority = %d, want 2", priority)
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
		check  func(t *testing.T, fields map[string]interface{})
	}{
		{
			name:   "string that looks like bool",
			fields: map[string]interface{}{"notes": "true"},
			check: func(t *testing.T, fields map[string]interface{}) {
				if v, ok := fields["notes"].(string); !ok || v != "true" {
					t.Errorf("notes = %v (%T), want string \"true\"", fields["notes"], fields["notes"])
				}
			},
		},
		{
			name:   "string that looks like date",
			fields: map[string]interface{}{"notes": "2026-05-15"},
			check: func(t *testing.T, fields map[string]interface{}) {
				if v, ok := fields["notes"].(string); !ok || v != "2026-05-15" {
					t.Errorf("notes = %v (%T), want string \"2026-05-15\"", fields["notes"], fields["notes"])
				}
			},
		},
		{
			name:   "string with colon",
			fields: map[string]interface{}{"notes": "a: b"},
			check: func(t *testing.T, fields map[string]interface{}) {
				if v, ok := fields["notes"].(string); !ok || v != "a: b" {
					t.Errorf("notes = %v (%T), want string \"a: b\"", fields["notes"], fields["notes"])
				}
			},
		},
		{
			name:   "string that looks like int",
			fields: map[string]interface{}{"notes": "42"},
			check: func(t *testing.T, fields map[string]interface{}) {
				if v, ok := fields["notes"].(string); !ok || v != "42" {
					t.Errorf("notes = %v (%T), want string \"42\"", fields["notes"], fields["notes"])
				}
			},
		},
		{
			name:   "list with ambiguous items",
			fields: map[string]interface{}{"labels": []string{"true", "42", "2026-05-15"}},
			check: func(t *testing.T, fields map[string]interface{}) {
				want := []string{"true", "42", "2026-05-15"}
				got, ok := fields["labels"].([]string)
				if !ok {
					t.Fatalf("labels type = %T, want []string", fields["labels"])
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("labels = %v, want %v", got, want)
				}
			},
		},
		{
			name:   "enum value that looks like bool",
			fields: map[string]interface{}{"yesno": "true"},
			check: func(t *testing.T, fields map[string]interface{}) {
				if v, ok := fields["yesno"].(string); !ok || v != "true" {
					t.Errorf("yesno = %v (%T), want string \"true\"", fields["yesno"], fields["yesno"])
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

			original := tikipkg.New()
			original.ID = "AMBIG1"
			original.Title = "Ambiguous round-trip"
			original.Set(tikipkg.FieldStatus, string(taskpkg.StatusReady))
			original.Set(tikipkg.FieldType, "story")
			original.Set(tikipkg.FieldPriority, 2)
			for k, v := range tt.fields {
				original.Set(k, v)
			}

			if err := store.saveTiki(original); err != nil {
				t.Fatalf("saveTiki: %v", err)
			}

			path := store.taskFilePath(original.ID)
			loaded, err := store.loadTikiFile(path, nil, nil)
			if err != nil {
				t.Fatalf("loadTikiFile: %v", err)
			}
			tt.check(t, loaded.Fields)
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

	custom, unknown, err := extractCustomFields(fmMap)
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

	loaded, err := store.loadTikiFile(filePath, nil, nil)
	if err != nil {
		t.Fatalf("loadTikiFile should succeed with stale field, got: %v", err)
	}
	if loaded.ID != "STALE1" {
		t.Errorf("ID = %q, want STALE1", loaded.ID)
	}
	severity, hasSeverity, _ := loaded.StringField("severity")
	if !hasSeverity || severity != "high" {
		t.Errorf("severity = %v, want high", loaded.Fields["severity"])
	}
	staleKeys := loaded.StaleKeys()
	if _, exists := staleKeys["old_field"]; exists {
		// old_field is unknown (not registered custom), so it lives in Fields but not stale
		t.Error("unknown key 'old_field' should not be marked stale")
	}
	// old_field is an unregistered key — it lands in Fields and round-trips as unknown
	oldFieldVal, hasOldField := loaded.Get("old_field")
	if !hasOldField || oldFieldVal != "leftover_value" {
		t.Errorf("Fields[old_field] = %v, want leftover_value", oldFieldVal)
	}
	// severity is a registered custom field with a valid value — must not be stale
	if _, isStaleSeverity := staleKeys["severity"]; isStaleSeverity {
		t.Error("severity should not be stale")
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

	custom, unknown, err := extractCustomFields(fmMap)
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

	loaded, err := store.loadTikiFile(filePath, nil, nil)
	if err != nil {
		t.Fatalf("loadTikiFile should succeed with stale enum value, got: %v", err)
	}
	if loaded.ID != "STALE2" {
		t.Errorf("ID = %q, want STALE2", loaded.ID)
	}
	// stale value should be marked stale (demoted from custom) and survive in Fields
	staleKeys := loaded.StaleKeys()
	if _, isStale := staleKeys["severity"]; !isStale {
		t.Error("stale enum value should be marked stale")
	}
	severityVal, hasSeverity := loaded.Get("severity")
	if !hasSeverity || severityVal != "critical" {
		t.Errorf("Fields[severity] = %v, want 'critical' (preserved for repair)", severityVal)
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

	_, _, err := extractCustomFields(fmMap)
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

	custom, unknown, err := extractCustomFields(fmMap)
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

	custom, unknown, err := extractCustomFields(nil)
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
	loaded, err := store.loadTikiFile(filePath, nil, nil)
	if err != nil {
		t.Fatalf("loadTikiFile: %v", err)
	}
	if err := store.saveTiki(loaded); err != nil {
		t.Fatalf("saveTiki: %v", err)
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

	input := tikipkg.New()
	input.ID = "SET001"
	input.Title = "dedupe built-ins"
	input.Set(tikipkg.FieldType, string(taskpkg.TypeStory))
	input.Set(tikipkg.FieldStatus, string(taskpkg.StatusBacklog))
	input.Set(tikipkg.FieldPriority, 3)
	input.Set(tikipkg.FieldTags, []string{"frontend", "backend", "frontend", " backend "})
	input.Set(tikipkg.FieldDependsOn, []string{"aaa001", "AAA001", " BBB002 "})
	input.Body = "body"

	if err := store.saveTiki(input); err != nil {
		t.Fatalf("saveTiki: %v", err)
	}

	path := store.taskFilePath(input.ID)
	loaded, err := store.loadTikiFile(path, nil, nil)
	if err != nil {
		t.Fatalf("loadTikiFile: %v", err)
	}

	tags, _, _ := loaded.StringSliceField(tikipkg.FieldTags)
	if !reflect.DeepEqual(tags, []string{"backend", "frontend"}) {
		t.Errorf("loaded tags = %v, want [backend frontend]", tags)
	}
	dependsOn, _, _ := loaded.StringSliceField(tikipkg.FieldDependsOn)
	if !reflect.DeepEqual(dependsOn, []string{"AAA001", "BBB002"}) {
		t.Errorf("loaded dependsOn = %v, want [AAA001 BBB002]", dependsOn)
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

	input := tikipkg.New()
	input.ID = "SET002"
	input.Title = "dedupe custom"
	input.Set(tikipkg.FieldType, string(taskpkg.TypeStory))
	input.Set(tikipkg.FieldStatus, string(taskpkg.StatusBacklog))
	input.Set(tikipkg.FieldPriority, 3)
	input.Set("labels", []string{"backend", "backend", " frontend ", ""})
	input.Set("related", []string{"aaa001", "AAA001", "bbb002"})

	if err := store.saveTiki(input); err != nil {
		t.Fatalf("saveTiki: %v", err)
	}

	path := store.taskFilePath(input.ID)
	loaded, err := store.loadTikiFile(path, nil, nil)
	if err != nil {
		t.Fatalf("loadTikiFile: %v", err)
	}

	labels, ok := loaded.Fields["labels"].([]string)
	if !ok {
		t.Fatalf("labels type = %T, want []string", loaded.Fields["labels"])
	}
	if !reflect.DeepEqual(labels, []string{"backend", "frontend"}) {
		t.Errorf("labels = %v, want [backend frontend]", labels)
	}

	related, ok := loaded.Fields["related"].([]string)
	if !ok {
		t.Fatalf("related type = %T, want []string", loaded.Fields["related"])
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

	tk, err := store.loadTikiFile(rel, nil, nil)
	if err != nil {
		t.Fatalf("loadTikiFile: %v", err)
	}
	if !filepath.IsAbs(tk.Path) {
		t.Errorf("Path is not absolute: %q", tk.Path)
	}
	if !strings.HasSuffix(tk.Path, fileName) {
		t.Errorf("Path does not end with expected filename: %q", tk.Path)
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

	tk := store.GetTiki("FP0003")
	if tk == nil {
		t.Fatal("GetTiki returned nil")
	}
	// loaded Path must be the real absolute path, not the stale string
	expectedAbs, _ := filepath.Abs(testFile)
	if tk.Path != expectedAbs {
		t.Errorf("Path = %q, want %q (real path, not stale)", tk.Path, expectedAbs)
	}
	// stale key must not leak into Fields or UnknownFields
	if _, exists := tk.Fields["filepath"]; exists {
		t.Errorf("stale filepath should not survive in Fields, got %v", tk.Fields["filepath"])
	}

	// save and re-read the file: stale key must be gone
	updated := tk.Clone()
	if err := store.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
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

	tk := tikipkg.New()
	tk.ID = "FP0002"
	tk.Title = "Save Filepath Test"
	tk.Set("type", "story")
	tk.Set("status", "backlog")
	tk.Set("priority", 3)
	if err := store.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	if tk.Path == "" {
		t.Fatal("Path not set after CreateTiki")
	}
	if !filepath.IsAbs(tk.Path) {
		t.Errorf("Path is not absolute: %q", tk.Path)
	}
	expectedPath := filepath.Join(tmpDir, "FP0002.md")
	expectedAbs, _ := filepath.Abs(expectedPath)
	if tk.Path != expectedAbs {
		t.Errorf("Path = %q, want %q", tk.Path, expectedAbs)
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
