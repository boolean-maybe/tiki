package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestUpdateTiki_SparseWorkflowDocPreservesExistingKeys verifies that
// UpdateTiki with a full-presence tiki preserves all keys from the original
// sparse workflow file when they are explicitly set on the incoming tiki.
// This is the tiki-native equivalent of the old UpdateTask carry-forward test.
func TestUpdateTiki_SparseWorkflowDocPreservesExistingKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	sparsePath := filepath.Join(tmp, "SPARSE.md")
	sparse := "---\nid: SPARSE\ntitle: v1\nstatus: ready\n---\n\nbody\n"
	if err := os.WriteFile(sparsePath, []byte(sparse), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// tiki-native update: load the stored tiki and update only the title,
	// leaving all other fields (including status) intact.
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

	if !strings.Contains(contents, "status: ready") {
		t.Errorf("status key erased by UpdateTiki; contents:\n%s", contents)
	}
	if !strings.Contains(contents, "title: v2") {
		t.Errorf("title edit did not persist; contents:\n%s", contents)
	}

	// Reload must still classify as workflow.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if tk := s.GetTiki("SPARSE"); tk == nil {
		t.Fatal("GetTiki after reload = nil")
	} else if !hasAnyWorkflowField(tk) {
		t.Error("sparse workflow doc demoted to plain across UpdateTiki + reload")
	}
}

// TestUpdateTiki_FullWorkflowDocPreservesEveryKey extends the sparse case
// to a fuller workflow doc: a file with status, type, and priority present.
// A tiki-native update that only changes the title must preserve all three
// when the caller loads the stored tiki first and modifies in-place.
func TestUpdateTiki_FullWorkflowDocPreservesEveryKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	filePath := filepath.Join(tmp, "FULL01.md")
	src := "---\nid: FULL01\ntitle: v1\nstatus: ready\ntype: story\npriority: 2\n---\n\nbody\n"
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
	for _, required := range []string{"title: v2", "status: ready", "type: story", "priority: 2"} {
		if !strings.Contains(contents, required) {
			t.Errorf("expected %q in file after UpdateTiki; contents:\n%s", required, contents)
		}
	}
}

// TestUpdateDocument_ExplicitKeyRemovalStillWorks verifies the tiki API can
// remove a workflow key by omitting it from the tiki's Fields map. This uses
// UpdateTiki with exact-presence semantics: a field absent from Fields is
// treated as explicitly removed.
func TestUpdateDocument_ExplicitKeyRemovalStillWorks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	filePath := filepath.Join(tmp, "TRIM01.md")
	src := "---\nid: TRIM01\ntitle: v1\nstatus: ready\npriority: 2\n---\n\nbody\n"
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
	// Remove priority explicitly — tiki exact-presence semantics honor this.
	updated := tk.Clone()
	updated.Delete(tikipkg.FieldPriority)
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	contents := string(data)
	if !strings.Contains(contents, "status: ready") {
		t.Errorf("status should be preserved; contents:\n%s", contents)
	}
	if strings.Contains(contents, "priority:") {
		t.Errorf("priority should be removed via tiki API; contents:\n%s", contents)
	}
}

// TestUpdateTiki_ExplicitDemotionWorksWithUpdateTiki verifies that
// UpdateTiki (exact-presence semantics) can demote a workflow doc to plain
// by omitting all workflow fields from the incoming tiki.
func TestUpdateTiki_ExplicitDemotionWorksWithUpdateTiki(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	filePath := filepath.Join(tmp, "DEMOT1.md")
	src := "---\nid: DEMOT1\ntitle: workflow\nstatus: ready\n---\n\nbody\n"
	if err := os.WriteFile(filePath, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Build a plain tiki (no workflow fields) and call UpdateTiki.
	plain := tikipkg.New()
	plain.ID = "DEMOT1"
	plain.Title = "now plain"
	if err := s.UpdateTiki(plain); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	data, _ := os.ReadFile(filePath)
	contents := string(data)
	if strings.Contains(contents, "status:") {
		t.Errorf("status should be removed via UpdateTiki; contents:\n%s", contents)
	}

	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if tk := s.GetTiki("DEMOT1"); tk == nil {
		t.Fatal("GetTiki after reload = nil")
	} else if hasAnyWorkflowField(tk) {
		t.Error("tiki should be plain after demotion via UpdateTiki")
	}
}
