package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestUpdateTiki_SparseDocPreservesExistingKeys verifies that UpdateTiki
// against a clone of a sparsely-populated stored tiki preserves every
// frontmatter key. Exact-presence: the cloned Fields map carries `status`
// forward, so a title-only edit doesn't drop it.
func TestUpdateTiki_SparseDocPreservesExistingKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	sparsePath := filepath.Join(tmp, "SPARSE.md")
	src := "---\nid: SPARSE\ntitle: v1\nstatus: backlog\n---\n\nbody\n"
	if err := os.WriteFile(sparsePath, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Tiki-native update: load the stored tiki and mutate the title only,
	// leaving every other field intact.
	stored := s.GetTiki("SPARSE")
	if stored == nil {
		t.Fatal("GetTiki = nil")
	}
	updated := stored.Clone()
	updated.Title = "v2"
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	data, err := os.ReadFile(sparsePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	contents := string(data)

	if !strings.Contains(contents, "status: backlog") {
		t.Errorf("status key erased by UpdateTiki; contents:\n%s", contents)
	}
	if !strings.Contains(contents, "title: v2") {
		t.Errorf("title edit did not persist; contents:\n%s", contents)
	}

	// Reload must show the schema-known field still present.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if tk := s.GetTiki("SPARSE"); tk == nil {
		t.Fatal("GetTiki after reload = nil")
	} else if !hasAnySchemaField(tk) {
		t.Error("sparse tiki lost its schema-known field across UpdateTiki + reload")
	}
}

// TestUpdateTiki_FullDocPreservesEveryKey extends the sparse case to a
// fuller tiki: a file with status, type, and priority. A clone-and-edit
// title update must preserve all three.
func TestUpdateTiki_FullDocPreservesEveryKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	filePath := filepath.Join(tmp, "FULL01.md")
	src := "---\nid: FULL01\ntitle: v1\nstatus: backlog\ntype: story\npriority: 2\n---\n\nbody\n"
	if err := os.WriteFile(filePath, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	stored := s.GetTiki("FULL01")
	if stored == nil {
		t.Fatal("GetTiki = nil")
	}
	updated := stored.Clone()
	updated.Title = "v2"
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	contents := string(data)
	for _, required := range []string{"title: v2", "status: backlog", "type: story", "priority: 2"} {
		if !strings.Contains(contents, required) {
			t.Errorf("expected %q in file after UpdateTiki; contents:\n%s", required, contents)
		}
	}
}

// TestUpdateTiki_ExplicitDeleteRemovesField verifies the tiki API can
// remove a field by Delete-ing it from Fields before UpdateTiki. Other
// keys in the cloned tiki survive untouched.
func TestUpdateTiki_ExplicitDeleteRemovesField(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	filePath := filepath.Join(tmp, "TRIM01.md")
	src := "---\nid: TRIM01\ntitle: v1\nstatus: backlog\npriority: 2\n---\n\nbody\n"
	if err := os.WriteFile(filePath, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk := s.GetTiki("TRIM01")
	if tk == nil {
		t.Fatal("GetTiki: nil")
	}
	updated := tk.Clone()
	updated.Delete(tikipkg.FieldPriority)
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	contents := string(data)
	if !strings.Contains(contents, "status: backlog") {
		t.Errorf("status should be preserved; contents:\n%s", contents)
	}
	if strings.Contains(contents, "priority:") {
		t.Errorf("priority should be removed by Delete; contents:\n%s", contents)
	}
}

// TestUpdateTiki_OmittingAllSchemaFieldsRemovesThem verifies the all-or-
// nothing case of exact presence: a fresh tiki with no Fields entries
// removes every schema-known key from the file. The id+title still pin
// the file in place; the rest is gone.
func TestUpdateTiki_OmittingAllSchemaFieldsRemovesThem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	filePath := filepath.Join(tmp, "SHRINK.md")
	src := "---\nid: SHRINK\ntitle: starts with status\nstatus: backlog\n---\n\nbody\n"
	if err := os.WriteFile(filePath, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	bare := tikipkg.New()
	bare.ID = "SHRINK"
	bare.Title = "now bare"
	if err := s.UpdateTiki(bare); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	contents := string(data)
	if strings.Contains(contents, "status:") {
		t.Errorf("status should be removed by exact-presence UpdateTiki; contents:\n%s", contents)
	}

	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if tk := s.GetTiki("SHRINK"); tk == nil {
		t.Fatal("GetTiki after reload = nil")
	} else if hasAnySchemaField(tk) {
		t.Error("schema-known fields survived reload — they must have been written back to disk")
	}
}
