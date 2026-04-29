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
		input     string
		wantID    string
		wantShape urlShape
		wantOK    bool
	}{
		// bare Phase-2 ids — the canonical form.
		{"ABC123", "ABC123", urlShapeBareID, true},
		{"abc123", "ABC123", urlShapeBareID, true},
		{"AbC123", "ABC123", urlShapeBareID, true},
		{"ZZZZZZ", "ZZZZZZ", urlShapeBareID, true},
		{"000000", "000000", urlShapeBareID, true},
		// legacy TIKI-* still accepted during the migration window.
		{"TIKI-ABC123", "ABC123", urlShapeLegacyTiki, true},
		{"tiki-abc123", "ABC123", urlShapeLegacyTiki, true},
		{"Tiki-AbC123", "ABC123", urlShapeLegacyTiki, true},
		// negatives: wrong length, wrong prefix, wrong charset.
		{"ABC12", "", urlShapeNone, false},
		{"ABC1234", "", urlShapeNone, false},
		{"TIKI-ABC12", "", urlShapeNone, false},
		{"TIKI-ABC1234", "", urlShapeNone, false},
		{"JIRA-ABC123", "", urlShapeNone, false},
		{"abc12!", "", urlShapeNone, false},
		{"", "", urlShapeNone, false},
		{"not-a-tiki", "", urlShapeNone, false},
		{"other.md", "", urlShapeNone, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, shape, ok := extractTaskID(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("extractTaskID(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.wantID {
				t.Errorf("extractTaskID(%q) id=%q, want %q", tt.input, got, tt.wantID)
			}
			if shape != tt.wantShape {
				t.Errorf("extractTaskID(%q) shape=%v, want %v", tt.input, shape, tt.wantShape)
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

	t.Run("legacy TIKI- prefix still resolves to bare id", func(t *testing.T) {
		content, err := provider.FetchContent(nav.NavElement{URL: "TIKI-ABC123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(content, "Test Task") {
			t.Errorf("expected title in content, got: %s", content)
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

	t.Run("unknown legacy TIKI- URL reports task-not-found", func(t *testing.T) {
		// Legacy `TIKI-*` is an unambiguous task reference — nothing on
		// disk uses that prefix — so the clearer "task not found" error
		// is preserved. This catches callers using old-format links for
		// ids that were since removed.
		_, err := provider.FetchContent(nav.NavElement{URL: "TIKI-ZZZZZZ"})
		if err == nil {
			t.Fatal("expected task-not-found error for legacy URL")
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
		if strings.Contains(err.Error(), "not found") && strings.Contains(err.Error(), "ABC123") {
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
