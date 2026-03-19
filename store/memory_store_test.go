package store

import (
	"testing"
	"time"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

func TestInMemoryStore_CreateTask(t *testing.T) {
	tests := []struct {
		name     string
		inputID  string
		expected string
	}{
		{"normalizes ID to uppercase", "tiki-abc123", "TIKI-ABC123"},
		{"trims whitespace from ID", "  TIKI-XYZ  ", "TIKI-XYZ"},
		{"already uppercase passthrough", "TIKI-DEF456", "TIKI-DEF456"},
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

func TestInMemoryStore_UpdateTask(t *testing.T) {
	t.Run("error when task not found", func(t *testing.T) {
		s := NewInMemoryStore()
		task := &taskpkg.Task{ID: "TIKI-MISSING", Title: "Ghost"}
		err := s.UpdateTask(task)
		if err == nil {
			t.Error("expected error for non-existent task, got nil")
		}
	})

	t.Run("normalizes ID before update", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-ABC123", Title: "Original"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		updated := &taskpkg.Task{ID: "tiki-abc123", Title: "Updated"}
		if err := s.UpdateTask(updated); err != nil {
			t.Fatalf("UpdateTask() error = %v", err)
		}
		got := s.GetTask("TIKI-ABC123")
		if got == nil || got.Title != "Updated" {
			t.Errorf("expected title %q, got %v", "Updated", got)
		}
	})

	t.Run("sets UpdatedAt after update", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-UPD001", Title: "Before"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		task := s.GetTask("TIKI-UPD001")
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
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-RT0001", Title: "Old Title"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		got := s.GetTask("TIKI-RT0001")
		got.Title = "New Title"
		if err := s.UpdateTask(got); err != nil {
			t.Fatalf("UpdateTask() error = %v", err)
		}

		reloaded := s.GetTask("TIKI-RT0001")
		if reloaded.Title != "New Title" {
			t.Errorf("title = %q, want %q", reloaded.Title, "New Title")
		}
	})
}

func TestInMemoryStore_DeleteTask(t *testing.T) {
	t.Run("removes existing task", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-DEL001", Title: "To Delete"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		s.DeleteTask("TIKI-DEL001")
		if s.GetTask("TIKI-DEL001") != nil {
			t.Error("expected nil after delete, got task")
		}
	})

	t.Run("no panic for non-existent ID", func(t *testing.T) {
		s := NewInMemoryStore()
		s.DeleteTask("TIKI-GHOST1") // should not panic
	})

	t.Run("normalizes ID for delete", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-LOWER1", Title: "Task"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		s.DeleteTask("tiki-lower1") // lowercase
		if s.GetTask("TIKI-LOWER1") != nil {
			t.Error("expected nil after delete with lowercase ID")
		}
	})
}

func TestInMemoryStore_AddComment(t *testing.T) {
	t.Run("returns false for unknown task", func(t *testing.T) {
		s := NewInMemoryStore()
		ok := s.AddComment("TIKI-NOEXST", taskpkg.Comment{Text: "hello"})
		if ok {
			t.Error("expected false for unknown task, got true")
		}
	})

	t.Run("returns true for known task", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-CMT001", Title: "Task"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		ok := s.AddComment("TIKI-CMT001", taskpkg.Comment{Text: "first comment"})
		if !ok {
			t.Error("expected true for known task")
		}
	})

	t.Run("sets comment CreatedAt", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-CMT002", Title: "Task"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		comment := taskpkg.Comment{Text: "check timestamp"}
		s.AddComment("TIKI-CMT002", comment)

		got := s.GetTask("TIKI-CMT002")
		if got.Comments[0].CreatedAt.IsZero() {
			t.Error("expected non-zero CreatedAt on comment")
		}
	})

	t.Run("appends to existing comments", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-CMT003", Title: "Task"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		s.AddComment("TIKI-CMT003", taskpkg.Comment{Text: "one"})
		s.AddComment("TIKI-CMT003", taskpkg.Comment{Text: "two"})

		got := s.GetTask("TIKI-CMT003")
		if len(got.Comments) != 2 {
			t.Errorf("expected 2 comments, got %d", len(got.Comments))
		}
	})

	t.Run("normalizes task ID", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-CMT004", Title: "Task"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		ok := s.AddComment("tiki-cmt004", taskpkg.Comment{Text: "lowercase key"})
		if !ok {
			t.Error("expected true with lowercase task ID")
		}
	})
}

func TestInMemoryStore_Search(t *testing.T) {
	buildStore := func(tb testing.TB) *InMemoryStore {
		tb.Helper()
		s := NewInMemoryStore()
		for _, task := range []*taskpkg.Task{
			{ID: "TIKI-S00001", Title: "Alpha feature", Description: "desc alpha", Tags: []string{"ui", "frontend"}},
			{ID: "TIKI-S00002", Title: "Beta Bug", Description: "beta description", Tags: []string{"backend"}},
			{ID: "TIKI-S00003", Title: "Gamma chore", Description: "third task"},
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
		{"matches ID case-insensitive", "tiki-s00001", nil, 1, 1},
		{"matches title case-insensitive", "alpha", nil, 1, 1},
		{"matches description", "beta description", nil, 1, 1},
		{"matches first tag", "ui", nil, 1, 1},
		{"matches second tag", "backend", nil, 1, 1},
		{"non-matching query returns empty", "zzz-no-match", nil, 0, 0},
		{"filterFunc excludes tasks", "", func(t *taskpkg.Task) bool { return t.ID == "TIKI-S00001" }, 1, 1},
		{"filterFunc + query intersection", "beta", func(t *taskpkg.Task) bool { return t.ID == "TIKI-S00002" }, 1, 1},
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
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-LIS001"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("called after UpdateTask", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-LIS002", Title: "orig"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		called := 0
		s.AddListener(func() { called++ })
		task := s.GetTask("TIKI-LIS002")
		if err := s.UpdateTask(task); err != nil {
			t.Fatalf("UpdateTask() error = %v", err)
		}
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("called after DeleteTask", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-LIS003"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		called := 0
		s.AddListener(func() { called++ })
		s.DeleteTask("TIKI-LIS003")
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("called after AddComment success", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-LIS004"}); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		called := 0
		s.AddListener(func() { called++ })
		s.AddComment("TIKI-LIS004", taskpkg.Comment{Text: "hi"})
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("not called after RemoveListener", func(t *testing.T) {
		s := NewInMemoryStore()
		called := 0
		id := s.AddListener(func() { called++ })
		s.RemoveListener(id)
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-LIS005"}); err != nil {
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
		if err := s.CreateTask(&taskpkg.Task{ID: "TIKI-LIS006"}); err != nil {
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
	if tmpl.Priority != 7 {
		t.Errorf("Priority = %d, want 7", tmpl.Priority)
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
	if tmpl.Type != taskpkg.TypeStory {
		t.Errorf("Type = %q, want %q", tmpl.Type, taskpkg.TypeStory)
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
		for _, id := range []string{"TIKI-ALL001", "TIKI-ALL002", "TIKI-ALL003"} {
			if err := s.CreateTask(&taskpkg.Task{ID: id}); err != nil {
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
		original := &taskpkg.Task{ID: "TIKI-PTR001", Title: "Pointer Task"}
		if err := s.CreateTask(original); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		tasks := s.GetAllTasks()
		if len(tasks) != 1 {
			t.Fatalf("got %d tasks, want 1", len(tasks))
		}
		// mutate via pointer returned from GetAllTasks
		tasks[0].Title = "Mutated"
		reloaded := s.GetTask("TIKI-PTR001")
		if reloaded.Title != "Mutated" {
			t.Errorf("title = %q, want %q — GetAllTasks should return pointers to stored tasks", reloaded.Title, "Mutated")
		}
	})
}
