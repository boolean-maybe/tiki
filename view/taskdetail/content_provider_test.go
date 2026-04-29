package taskdetail

import (
	"strings"
	"testing"

	nav "github.com/boolean-maybe/navidown/navidown"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

func TestExtractTaskID(t *testing.T) {
	tests := []struct {
		input  string
		wantID string
		wantOK bool
	}{
		// bare ids — the only recognized form.
		{"ABC123", "ABC123", true},
		{"abc123", "ABC123", true},
		{"AbC123", "ABC123", true},
		{"ZZZZZZ", "ZZZZZZ", true},
		{"000000", "000000", true},
		// TIKI- prefixed URLs are no longer parsed as task references;
		// they fall through to FileHTTP like any other non-bare URL.
		{"TIKI-ABC123", "", false},
		{"tiki-abc123", "", false},
		{"Tiki-AbC123", "", false},
		// negatives: wrong length, wrong charset, empty.
		{"ABC12", "", false},
		{"ABC1234", "", false},
		{"JIRA-ABC123", "", false},
		{"abc12!", "", false},
		{"", "", false},
		{"not-a-tiki", "", false},
		{"other.md", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := extractTaskID(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("extractTaskID(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.wantID {
				t.Errorf("extractTaskID(%q) id=%q, want %q", tt.input, got, tt.wantID)
			}
		})
	}
}

func TestTaskDescriptionProvider_FetchContent_TikiID(t *testing.T) {
	s := store.NewInMemoryStore()
	_ = s.CreateTask(&task.Task{
		ID:          "ABC123",
		Title:       "Test Task",
		Description: "some description",
		Status:      task.StatusReady,
		Type:        task.TypeStory,
		Priority:    2,
	})

	provider := newTaskDescriptionProvider(s, nil)

	t.Run("bare uppercase ID", func(t *testing.T) {
		content, err := provider.FetchContent(nav.NavElement{URL: "ABC123"})
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

	t.Run("bare lowercase ID", func(t *testing.T) {
		content, err := provider.FetchContent(nav.NavElement{URL: "abc123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(content, "Test Task") {
			t.Errorf("expected title in content, got: %s", content)
		}
	})

	t.Run("TIKI- prefix no longer resolves to a task", func(t *testing.T) {
		// Pre-unification TIKI- URLs are no longer recognized as task
		// references; they fall through to the file loader like any other
		// non-bare URL. With nil search roots the loader fails, but the
		// failure must come from FileHTTP — not from a task lookup.
		_, err := provider.FetchContent(nav.NavElement{URL: "TIKI-ABC123"})
		if err == nil {
			t.Fatal("expected file-not-found error for TIKI- URL")
		}
		// Must NOT produce a task-specific "not found" error — that would
		// mean we still parsed it as a task reference.
		if strings.Contains(err.Error(), "task ABC123") {
			t.Errorf("TIKI- URL should not be parsed as a task reference, got: %v", err)
		}
	})

	t.Run("unknown bare ID falls through to FileHTTP", func(t *testing.T) {
		// A bare 6-char URL is ambiguous: it can be a document id or a
		// filename. When the store doesn't have it, we must fall through
		// to the file loader so a link to `ZZZZZZ.md` on disk still works.
		// Without the fall-through, valid file links whose base name
		// happens to match the bare-id shape would always error.
		_, err := provider.FetchContent(nav.NavElement{URL: "ZZZZZZ"})
		if err == nil {
			t.Fatal("expected error for nonexistent file (nil search roots)")
		}
		// The error must NOT be the "task X not found" one — that would
		// mean we bailed out instead of trying FileHTTP.
		if strings.Contains(err.Error(), "task ZZZZZZ not found") {
			t.Errorf("should have fallen through to FileHTTP, got task error: %v", err)
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
		if strings.Contains(err.Error(), "not found") && strings.Contains(err.Error(), "ABC123") {
			t.Errorf("should not have attempted task lookup for non-tiki URL")
		}
	})
}

func TestFormatTaskAsMarkdown(t *testing.T) {
	t.Run("with all fields", func(t *testing.T) {
		tk := &task.Task{
			ID:          "ABC123",
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
		if !strings.Contains(md, "ABC123") {
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
			ID:     "ABC123",
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
			ID:     "ABC123",
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
