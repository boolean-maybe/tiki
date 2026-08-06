package tikistore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDoesNotRecognizeFormerStatusFieldName(t *testing.T) {
	dir := t.TempDir()
	src := "---\nid: ABC123\ntitle: test\nstatus: in_progress\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "ABC123.md"), []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	tk := s.GetTiki("ABC123")
	if tk == nil {
		t.Fatal("loaded tiki not found")
	}
	got, present := tk.Get("status")
	if !present || got != "in_progress" {
		t.Fatalf("field named status loaded as %v (present=%v), want unchanged", got, present)
	}
}
