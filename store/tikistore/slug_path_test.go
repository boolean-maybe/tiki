package tikistore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// newTiki builds a test tiki. the constructor is tiki.New() (returns *Tiki with
// an initialized Fields map); there is no NewTiki().
func newTiki(id, title string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.SetID(id)
	tk.SetTitle(title)
	return tk
}

// newSlugTestStore returns a real store rooted at a temp dir. use NewTikiStore
// (the only constructor) rather than a &TikiStore{} literal — the literal leaves
// the mutex, listeners, and other internals zero-valued, which is unsafe for
// CreateTiki and notifyListeners.
func newSlugTestStore(t *testing.T) *TikiStore {
	t.Helper()
	s, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	return s
}

func TestSlugFilePath_BareName(t *testing.T) {
	s := newSlugTestStore(t)
	got, err := s.slugFilePath(newTiki("ABC123", "Fix Login"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(s.dir, "fix-login.md")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSlugFilePath_NumericSuffixOnCollision(t *testing.T) {
	s := newSlugTestStore(t)
	// occupy fix-login.md (e.g. a renamed/external file the index does not know)
	if err := os.WriteFile(filepath.Join(s.dir, "fix-login.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.slugFilePath(newTiki("DEF456", "Fix Login"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(s.dir, "fix-login-2.md")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSlugFilePath_SkipsOccupiedMiddle(t *testing.T) {
	s := newSlugTestStore(t)
	for _, n := range []string{"fix-login.md", "fix-login-2.md"} {
		if err := os.WriteFile(filepath.Join(s.dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.slugFilePath(newTiki("ABC123", "Fix Login"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(s.dir, "fix-login-3.md")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSlugFilePath_EmptyTitleRejected(t *testing.T) {
	s := newSlugTestStore(t)
	_, err := s.slugFilePath(newTiki("ABC123", "!!!"))
	if err == nil {
		t.Fatal("expected error for empty-slug title, got nil")
	}
	if !errors.Is(err, ErrEmptyTitleForSlug) {
		t.Fatalf("want ErrEmptyTitleForSlug, got %v", err)
	}
}

func TestPathForTiki_LoadedPathWins(t *testing.T) {
	s := newSlugTestStore(t)
	tk := newTiki("ABC123", "Fix Login")
	existing := filepath.Join(s.dir, "renamed.md")
	tk.SetPath(existing)
	got, err := s.pathForTiki(tk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != existing {
		t.Fatalf("loaded path should win: got %q, want %q", got, existing)
	}
}

func TestPathForTiki_NewUsesSlug(t *testing.T) {
	s := newSlugTestStore(t)
	got, err := s.pathForTiki(newTiki("GHI789", "Fix Login"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Join(s.dir, "fix-login.md") {
		t.Fatalf("got %q", got)
	}
}

func TestCreateTiki_EmptyTitleRejected(t *testing.T) {
	s := newSlugTestStore(t)
	tk := newTiki("", "!!!") // no id yet, title slugs to empty
	err := s.storeNewDocumentLocked(tk)
	if !errors.Is(err, ErrEmptyTitleForSlug) {
		t.Fatalf("want ErrEmptyTitleForSlug, got %v", err)
	}
	if tk.ID() != "" {
		t.Fatalf("no ID should be assigned on rejection, got %q", tk.ID())
	}
	if len(s.tikis) != 0 {
		t.Fatalf("store should be empty after rejection, has %d", len(s.tikis))
	}
}

func TestCreateTiki_WritesSlugFilename(t *testing.T) {
	s := newSlugTestStore(t)
	tk := newTiki("", "Fix Login Bug")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "fix-login-bug.md")); err != nil {
		t.Fatalf("expected fix-login-bug.md on disk: %v", err)
	}
}
