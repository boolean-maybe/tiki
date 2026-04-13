package taskdetail

import (
	"strings"
	"testing"

	nav "github.com/boolean-maybe/navidown/navidown"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

func TestLooksLikeTikiID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"TIKI-ABC123", true},
		{"tiki-abc123", true},
		{"Tiki-AbC123", true},
		{"TIKI-ZZZZZZ", true},
		{"TIKI-000000", true},
		{"TIKI-ABC12", true},
		{"TIKI-ABC1234", true},
		{"JIRA-ABC123", false},
		{"PROJ-FEATURE-1", false},
		{"tiki-abc12!", false},
		{"", false},
		{"not-a-tiki", false},
		{"other.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := looksLikeTikiID(tt.input); got != tt.want {
				t.Errorf("looksLikeTikiID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTaskDescriptionProvider_FetchContent_TikiID(t *testing.T) {
	s := store.NewInMemoryStore()
	_ = s.CreateTask(&task.Task{
		ID:          "TIKI-ABC123",
		Title:       "Test Task",
		Description: "some description",
		Status:      task.StatusReady,
		Type:        task.TypeStory,
		Priority:    2,
	})

	provider := newTaskDescriptionProvider(s, nil)

	t.Run("uppercase tiki ID", func(t *testing.T) {
		content, err := provider.FetchContent(nav.NavElement{URL: "TIKI-ABC123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(content, "Test Task") {
			t.Errorf("expected title in content, got: %s", content)
		}
		if !strings.Contains(content, "some description") {
			t.Errorf("expected description in content, got: %s", content)
		}
		if !strings.Contains(content, "P2") {
			t.Errorf("expected priority in content, got: %s", content)
		}
	})

	t.Run("lowercase tiki ID", func(t *testing.T) {
		content, err := provider.FetchContent(nav.NavElement{URL: "tiki-abc123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(content, "Test Task") {
			t.Errorf("expected title in content, got: %s", content)
		}
	})

	t.Run("not found tiki ID", func(t *testing.T) {
		_, err := provider.FetchContent(nav.NavElement{URL: "TIKI-ZZZZZZ"})
		if err == nil {
			t.Fatal("expected error for missing task")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' in error, got: %v", err)
		}
	})

	t.Run("non-tiki URL falls through", func(t *testing.T) {
		// FileHTTP with nil search roots will fail on a nonexistent file,
		// but the point is it doesn't try the store path
		_, err := provider.FetchContent(nav.NavElement{URL: "other.md"})
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
		// error should be from FileHTTP, not "not found" task error
		if strings.Contains(err.Error(), "not found") && strings.Contains(err.Error(), "TIKI") {
			t.Errorf("should not have attempted task lookup for non-tiki URL")
		}
	})
}

func TestFormatTaskAsMarkdown(t *testing.T) {
	t.Run("with all fields", func(t *testing.T) {
		tk := &task.Task{
			ID:          "TIKI-ABC123",
			Title:       "My Task",
			Description: "detailed desc",
			Status:      task.StatusInProgress,
			Type:        task.TypeBug,
			Priority:    1,
		}
		md := formatTaskAsMarkdown(tk)
		if !strings.HasPrefix(md, "# My Task\n") {
			t.Errorf("expected title as h1, got: %s", md)
		}
		if !strings.Contains(md, "TIKI-ABC123") {
			t.Error("expected task ID in output")
		}
		if !strings.Contains(md, "P1") {
			t.Error("expected priority in output")
		}
		if !strings.Contains(md, "detailed desc") {
			t.Error("expected description in output")
		}
	})

	t.Run("no priority", func(t *testing.T) {
		tk := &task.Task{
			ID:     "TIKI-ABC123",
			Title:  "No Prio",
			Status: task.StatusReady,
			Type:   task.TypeStory,
		}
		md := formatTaskAsMarkdown(tk)
		if strings.Contains(md, "P0") {
			t.Error("should not show P0 for zero priority")
		}
	})

	t.Run("empty description", func(t *testing.T) {
		tk := &task.Task{
			ID:     "TIKI-ABC123",
			Title:  "No Desc",
			Status: task.StatusReady,
			Type:   task.TypeStory,
		}
		md := formatTaskAsMarkdown(tk)
		// should end after the metadata line
		lines := strings.Split(strings.TrimSpace(md), "\n")
		if len(lines) != 3 { // title, blank, metadata
			t.Errorf("expected 3 lines for no-description task, got %d: %q", len(lines), md)
		}
	})
}
