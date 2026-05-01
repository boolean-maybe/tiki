package store

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestInMemoryStore_CreateTask(t *testing.T) {
	tests := []struct {
		name     string
		inputID  string
		expected string
	}{
		{"normalizes ID to uppercase", "abc123", "ABC123"},
		{"trims whitespace from ID", "  DEF456  ", "DEF456"},
		{"already uppercase passthrough", "GHI789", "GHI789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewInMemoryStore()
			task := &taskpkg.Task{ID: tt.inputID, Title: "Test", Type: taskpkg.TypeStory, Status: taskpkg.DefaultStatus()}

			err := s.CreateTask(task)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			if task.ID != tt.expected {
				t.Errorf("task.ID = %q, want %q", task.ID, tt.expected)
			}
			if task.CreatedAt.IsZero() {
				t.Error("expected non-zero CreatedAt")
			}
			if task.UpdatedAt.IsZero() {
				t.Error("expected non-zero UpdatedAt")
			}
			got := s.GetTask(tt.expected)
			if got == nil {
				t.Errorf("GetTask(%q) returned nil after CreateTask", tt.expected)
			}
		})
	}
}

func TestInMemoryStore_CreateTaskNormalizesCollections(t *testing.T) {
	s := NewInMemoryStore()
	// need existing tasks that dependsOn references so validation passes.
	if err := s.CreateTask(&taskpkg.Task{ID: "AAA001", Title: "dep1"}); err != nil {
		t.Fatalf("setup dep1: %v", err)
	}
	if err := s.CreateTask(&taskpkg.Task{ID: "BBB002", Title: "dep2"}); err != nil {
		t.Fatalf("setup dep2: %v", err)
	}
	input := &taskpkg.Task{
		ID:        "NORM01",
		Title:     "normalize",
		Tags:      []string{"frontend", "frontend", " backend ", ""},
		DependsOn: []string{"aaa001", "AAA001", "bbb002"},
	}

	if err := s.CreateTask(input); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if !reflect.DeepEqual(input.Tags, []string{"frontend", "backend"}) {
		t.Errorf("tags = %v, want [frontend backend]", input.Tags)
	}
	if !reflect.DeepEqual(input.DependsOn, []string{"AAA001", "BBB002"}) {
		t.Errorf("dependsOn = %v, want [AAA001 BBB002]", input.DependsOn)
	}
}

func TestInMemoryStore_UpdateTask(t *testing.T) {
	t.Run("error when task not found", func(t *testing.T) {
		s := NewInMemoryStore()
		task := &taskpkg.Task{ID: "MISSING", Title: "Ghost"}
		err := s.UpdateTask(task)
		if err == nil {
			t.Error("expected error for non-existent task, got nil")
		}
	})

	t.Run("normalizes ID before update", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "ABC123", Title: "Original"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		updated := &taskpkg.Task{ID: "abc123", Title: "Updated"}
		if err := s.UpdateTask(updated); err != nil {
			t.Fatalf("UpdateTask() error = %v", err)
		}
		got := s.GetTask("ABC123")
		if got == nil || got.Title != "Updated" {
			t.Errorf("expected title %q, got %v", "Updated", got)
		}
	})

	t.Run("sets UpdatedAt after update", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "UPD001", Title: "Before"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		task := s.GetTask("UPD001")
		before := task.UpdatedAt

		time.Sleep(time.Millisecond)
		task.Title = "After"
		if err := s.UpdateTask(task); err != nil {
			t.Fatalf("UpdateTask() error = %v", err)
		}
		if !task.UpdatedAt.After(before) {
			t.Errorf("expected UpdatedAt to advance after update")
		}
	})

	t.Run("updates task fields roundtrip", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "RT0001", Title: "Old Title"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		got := s.GetTask("RT0001")
		got.Title = "New Title"
		if err := s.UpdateTask(got); err != nil {
			t.Fatalf("UpdateTask() error = %v", err)
		}

		reloaded := s.GetTask("RT0001")
		if reloaded.Title != "New Title" {
			t.Errorf("title = %q, want %q", reloaded.Title, "New Title")
		}
	})

	// Mirrors the TikiStore test of the same shape: a caller that rebuilds
	// a Task from only the fields it cares about (leaving IsWorkflow at
	// its zero value) must not silently demote a workflow task to a plain
	// doc. InMemoryStore is used by ruki tests and other paths; this guard
	// keeps the two backends behaviorally consistent.
	t.Run("preserves IsWorkflow when caller passes a fresh value", func(t *testing.T) {
		s := NewInMemoryStore()
		// Post-Phase-7: CreateTask honors the caller's IsWorkflow instead of
		// forcing it true, so explicit opt-in is required for a workflow item.
		if err := s.CreateTask(&taskpkg.Task{ID: "WFMEM1", Title: "workflow", IsWorkflow: true}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		if !s.GetTask("WFMEM1").IsWorkflow {
			t.Fatal("precondition: CreateTask should persist IsWorkflow=true")
		}

		// Fresh Task — no IsWorkflow set, zero-value false. UpdateTask must
		// still carry it forward from the stored workflow task.
		fresh := &taskpkg.Task{ID: "WFMEM1", Title: "updated"}
		if err := s.UpdateTask(fresh); err != nil {
			t.Fatalf("UpdateTask() error = %v", err)
		}
		if !s.GetTask("WFMEM1").IsWorkflow {
			t.Error("IsWorkflow demoted to false after UpdateTask — task would vanish from workflow-filtered views")
		}
	})
}

func TestInMemoryStore_DeleteTask(t *testing.T) {
	t.Run("removes existing task", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "DEL001", Title: "To Delete"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		s.DeleteTask("DEL001")
		if s.GetTask("DEL001") != nil {
			t.Error("expected nil after delete, got task")
		}
	})

	t.Run("no panic for non-existent ID", func(t *testing.T) {
		s := NewInMemoryStore()
		s.DeleteTask("GHOST1") // should not panic
	})

	t.Run("normalizes ID for delete", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "LOWER1", Title: "Task"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		s.DeleteTask("lower1") // lowercase bare id
		if s.GetTask("LOWER1") != nil {
			t.Error("expected nil after delete with lowercase ID")
		}
	})
}

func TestInMemoryStore_AddComment(t *testing.T) {
	t.Run("returns false for unknown task", func(t *testing.T) {
		s := NewInMemoryStore()
		ok := s.AddComment("NOEXST", taskpkg.Comment{Text: "hello"})
		if ok {
			t.Error("expected false for unknown task, got true")
		}
	})

	t.Run("returns true for known task", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "CMT001", Title: "Task"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		ok := s.AddComment("CMT001", taskpkg.Comment{Text: "first comment"})
		if !ok {
			t.Error("expected true for known task")
		}
	})

	t.Run("sets comment CreatedAt", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "CMT002", Title: "Task"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		comment := taskpkg.Comment{Text: "check timestamp"}
		s.AddComment("CMT002", comment)

		got := s.GetTask("CMT002")
		if got.Comments[0].CreatedAt.IsZero() {
			t.Error("expected non-zero CreatedAt on comment")
		}
	})

	t.Run("appends to existing comments", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "CMT003", Title: "Task"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		s.AddComment("CMT003", taskpkg.Comment{Text: "one"})
		s.AddComment("CMT003", taskpkg.Comment{Text: "two"})

		got := s.GetTask("CMT003")
		if len(got.Comments) != 2 {
			t.Errorf("expected 2 comments, got %d", len(got.Comments))
		}
	})

	t.Run("normalizes task ID", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "CMT004", Title: "Task"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		ok := s.AddComment("cmt004", taskpkg.Comment{Text: "lowercase key"})
		if !ok {
			t.Error("expected true with lowercase task ID")
		}
	})
}

// TestInMemoryStore_Search_NilFilterExcludesPlainDocs verifies the L4
// contract for the in-memory store: nil filterFunc means "workflow tasks
// only", matching TikiStore.Search and GetAllTasks.
func TestInMemoryStore_Search_NilFilterExcludesPlainDocs(t *testing.T) {
	s := NewInMemoryStore()
	// direct map poke since CreateTask would set IsWorkflow=true.
	s.tasks["WORK01"] = &taskpkg.Task{ID: "WORK01", Title: "workflow", IsWorkflow: true}
	s.tasks["PLAIN1"] = &taskpkg.Task{ID: "PLAIN1", Title: "plain", IsWorkflow: false}

	results := s.Search("", nil)
	if len(results) != 1 || results[0].Task.ID != "WORK01" {
		t.Errorf("nil filter: got %v, want [WORK01]", results)
	}

	// explicit caller filter bypasses the workflow gate.
	all := s.Search("", func(*taskpkg.Task) bool { return true })
	if len(all) != 2 {
		t.Errorf("explicit filter: got %d, want 2", len(all))
	}
}

func TestInMemoryStore_Search(t *testing.T) {
	buildStore := func(tb testing.TB) *InMemoryStore {
		tb.Helper()
		s := NewInMemoryStore()
		for _, task := range []*taskpkg.Task{
			{ID: "S00001", Title: "Alpha feature", Description: "desc alpha", Tags: []string{"ui", "frontend"}, IsWorkflow: true},
			{ID: "S00002", Title: "Beta Bug", Description: "beta description", Tags: []string{"backend"}, IsWorkflow: true},
			{ID: "S00003", Title: "Gamma chore", Description: "third task", IsWorkflow: true},
		} {
			if err := s.CreateTask(task); err != nil {
				tb.Fatalf("CreateTask() error = %v", err)
			}
		}
		return s
	}

	tests := []struct {
		name       string
		query      string
		filterFunc func(*taskpkg.Task) bool
		minResults int
		maxResults int
	}{
		{"empty query + nil filter returns all", "", nil, 3, 3},
		{"matches ID case-insensitive", "s00001", nil, 1, 1},
		{"matches title case-insensitive", "alpha", nil, 1, 1},
		{"matches description", "beta description", nil, 1, 1},
		{"matches first tag", "ui", nil, 1, 1},
		{"matches second tag", "backend", nil, 1, 1},
		{"non-matching query returns empty", "zzz-no-match", nil, 0, 0},
		{"filterFunc excludes tasks", "", func(t *taskpkg.Task) bool { return t.ID == "S00001" }, 1, 1},
		{"filterFunc + query intersection", "beta", func(t *taskpkg.Task) bool { return t.ID == "S00002" }, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := buildStore(t)
			results := s.Search(tt.query, tt.filterFunc)
			if len(results) < tt.minResults || len(results) > tt.maxResults {
				t.Errorf("Search(%q) returned %d results, want [%d, %d]", tt.query, len(results), tt.minResults, tt.maxResults)
			}
		})
	}
}

func TestInMemoryStore_Listeners(t *testing.T) {
	t.Run("called after CreateTask", func(t *testing.T) {
		s := NewInMemoryStore()
		called := 0
		s.AddListener(func() { called++ })
		if err := s.CreateTask(&taskpkg.Task{ID: "LIS001"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("called after UpdateTask", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "LIS002", Title: "orig"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		called := 0
		s.AddListener(func() { called++ })
		task := s.GetTask("LIS002")
		if err := s.UpdateTask(task); err != nil {
			t.Fatalf("UpdateTask() error = %v", err)
		}
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("called after DeleteTask", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "LIS003"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		called := 0
		s.AddListener(func() { called++ })
		s.DeleteTask("LIS003")
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("called after AddComment success", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "LIS004"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		called := 0
		s.AddListener(func() { called++ })
		s.AddComment("LIS004", taskpkg.Comment{Text: "hi"})
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("not called after RemoveListener", func(t *testing.T) {
		s := NewInMemoryStore()
		called := 0
		id := s.AddListener(func() { called++ })
		s.RemoveListener(id)
		if err := s.CreateTask(&taskpkg.Task{ID: "LIS005"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		if called != 0 {
			t.Errorf("removed listener called %d times, want 0", called)
		}
	})

	t.Run("multiple listeners all notified", func(t *testing.T) {
		s := NewInMemoryStore()
		a, b := 0, 0
		s.AddListener(func() { a++ })
		s.AddListener(func() { b++ })
		if err := s.CreateTask(&taskpkg.Task{ID: "LIS006"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		if a != 1 || b != 1 {
			t.Errorf("listeners called a=%d b=%d, want both 1", a, b)
		}
	})
}

func TestInMemoryStore_NewTaskTemplate(t *testing.T) {
	s := NewInMemoryStore()
	tmpl, err := s.NewTaskTemplate()
	if err != nil {
		t.Fatalf("NewTaskTemplate() error = %v", err)
	}
	// Post-Phase-1: IDs are bare uppercase (no TIKI- prefix), 6 chars.
	if len(tmpl.ID) != 6 {
		t.Errorf("ID = %q, want 6-character bare ID", tmpl.ID)
	}
	if tmpl.ID != strings.ToUpper(tmpl.ID) {
		t.Errorf("ID = %q, should be uppercased", tmpl.ID)
	}
	if tmpl.Priority != 3 {
		t.Errorf("Priority = %d, want 3", tmpl.Priority)
	}
	if tmpl.Points != 1 {
		t.Errorf("Points = %d, want 1", tmpl.Points)
	}
	if len(tmpl.Tags) != 1 || tmpl.Tags[0] != "idea" {
		t.Errorf("Tags = %v, want [idea]", tmpl.Tags)
	}
	if tmpl.Status != taskpkg.DefaultStatus() {
		t.Errorf("Status = %q, want %q", tmpl.Status, taskpkg.DefaultStatus())
	}
	if tmpl.Type != taskpkg.DefaultType() {
		t.Errorf("Type = %q, want %q", tmpl.Type, taskpkg.DefaultType())
	}
}

func TestBuildMemoryFieldDefaults_DedupesCollectionDefaults(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "labels", Type: workflow.TypeListString, DefaultValue: []string{"backend", " backend ", "backend"}},
		{Name: "related", Type: workflow.TypeListRef, DefaultValue: []string{"aaa001", "AAA001", "bbb002"}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	defaults := buildMemoryFieldDefaults()
	labels, ok := defaults["labels"].([]string)
	if !ok {
		t.Fatalf("labels type = %T, want []string", defaults["labels"])
	}
	if !reflect.DeepEqual(labels, []string{"backend"}) {
		t.Errorf("labels = %v, want [backend]", labels)
	}

	related, ok := defaults["related"].([]string)
	if !ok {
		t.Fatalf("related type = %T, want []string", defaults["related"])
	}
	if !reflect.DeepEqual(related, []string{"AAA001", "BBB002"}) {
		t.Errorf("related = %v, want [AAA001 BBB002]", related)
	}
}

func TestInMemoryStore_NewTaskTemplateCollision(t *testing.T) {
	s := NewInMemoryStore()

	// pre-populate store with a task that will collide on bare id
	_ = s.CreateTask(&taskpkg.Task{ID: "AAAAAA", Title: "existing"})

	callCount := 0
	s.idGenerator = func() string {
		callCount++
		if callCount == 1 {
			return "AAAAAA" // will collide with existing bare id
		}
		return "BBBBBB" // will succeed
	}

	tmpl, err := s.NewTaskTemplate()
	if err != nil {
		t.Fatalf("NewTaskTemplate() error = %v", err)
	}
	if tmpl.ID != "BBBBBB" {
		t.Errorf("ID = %q, want BBBBBB (should skip collision)", tmpl.ID)
	}
	if callCount != 2 {
		t.Errorf("idGenerator called %d times, want 2 (one collision + one success)", callCount)
	}
}

func TestInMemoryStore_NewTaskTemplateExhaustion(t *testing.T) {
	s := NewInMemoryStore()

	// pre-populate with the only ID the generator will ever produce
	_ = s.CreateTask(&taskpkg.Task{ID: "AAAAAA", Title: "existing"})

	s.idGenerator = func() string { return "AAAAAA" }

	_, err := s.NewTaskTemplate()
	if err == nil {
		t.Fatal("expected error for ID exhaustion, got nil")
	}
	if !strings.Contains(err.Error(), "failed to generate unique task ID") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInMemoryStore_GetAllTasks(t *testing.T) {
	t.Run("empty store returns empty slice", func(t *testing.T) {
		s := NewInMemoryStore()
		tasks := s.GetAllTasks()
		if len(tasks) != 0 {
			t.Errorf("got %d tasks, want 0", len(tasks))
		}
	})

	t.Run("3 tasks returns len 3", func(t *testing.T) {
		s := NewInMemoryStore()
		for _, id := range []string{"ALL001", "ALL002", "ALL003"} {
			if err := s.CreateTask(&taskpkg.Task{ID: id, IsWorkflow: true}); err != nil {
				t.Fatalf("CreateTask(%s) error = %v", id, err)
			}
		}
		tasks := s.GetAllTasks()
		if len(tasks) != 3 {
			t.Errorf("got %d tasks, want 3", len(tasks))
		}
	})

	t.Run("returns same pointers, not copies", func(t *testing.T) {
		s := NewInMemoryStore()
		original := &taskpkg.Task{ID: "PTR001", Title: "Pointer Task", IsWorkflow: true}
		if err := s.CreateTask(original); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		tasks := s.GetAllTasks()
		if len(tasks) != 1 {
			t.Fatalf("got %d tasks, want 1", len(tasks))
		}
		// mutate via pointer returned from GetAllTasks
		tasks[0].Title = "Mutated"
		reloaded := s.GetTask("PTR001")
		if reloaded.Title != "Mutated" {
			t.Errorf("title = %q, want %q — GetAllTasks should return pointers to stored tasks", reloaded.Title, "Mutated")
		}
	})
}

func TestSearch_WithQueryAndFilter(t *testing.T) {
	s := NewInMemoryStore()
	if err := s.CreateTask(&taskpkg.Task{ID: "SRC001", Title: "Bug in parser", Tags: []string{"backend"}}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := s.CreateTask(&taskpkg.Task{ID: "SRC002", Title: "Bug in UI", Tags: []string{"frontend"}}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	if err := s.CreateTask(&taskpkg.Task{ID: "SRC003", Title: "Feature request", Tags: []string{"backend"}}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// query "Bug" + filter for backend tag
	results := s.Search("Bug", func(t *taskpkg.Task) bool {
		for _, tag := range t.Tags {
			if tag == "backend" {
				return true
			}
		}
		return false
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result (Bug + backend), got %d", len(results))
	}
	if results[0].Task.ID != "SRC001" {
		t.Errorf("expected SRC001, got %s", results[0].Task.ID)
	}
}

func TestSearch_FilterRejectsAll(t *testing.T) {
	s := NewInMemoryStore()
	if err := s.CreateTask(&taskpkg.Task{ID: "REJ001", Title: "Task"}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	results := s.Search("", func(t *taskpkg.Task) bool {
		return false // reject all
	})
	if len(results) != 0 {
		t.Fatalf("expected 0 results when filter rejects all, got %d", len(results))
	}
}

func TestSearch_MatchesTags(t *testing.T) {
	s := NewInMemoryStore()
	if err := s.CreateTask(&taskpkg.Task{ID: "TAG001", Title: "No match in title", Tags: []string{"backend"}, IsWorkflow: true}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	results := s.Search("backend", nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (tag match), got %d", len(results))
	}
}

func TestNewTaskTemplate_IDCollision(t *testing.T) {
	s := NewInMemoryStore()
	// pre-populate so the generated bare ID always collides
	if err := s.CreateTask(&taskpkg.Task{ID: "FIXED1", Title: "blocker"}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// set idGenerator to always return the same ID
	s.idGenerator = func() string { return "FIXED1" }

	_, err := s.NewTaskTemplate()
	if err == nil {
		t.Fatal("expected error for ID exhaustion")
	}
}
