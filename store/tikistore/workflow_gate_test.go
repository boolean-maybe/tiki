package tikistore

import (
	"os"
	"path/filepath"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

func createValidTikiForGateTest() *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = "CREAT1"
	tk.Title = "created"
	tk.Set("type", "story")
	tk.Set("status", "ready")
	tk.Set("priority", 1)
	return tk
}

// TestGetAllTikis_IncludesPlainDocs verifies the Phase 5 contract: GetAllTikis
// returns all tikis, including plain docs. Workflow-only filtering belongs in
// the caller (e.g. ruki `select where has(status)` or hasAnyWorkflowField)
// not at the store boundary.
func TestGetAllTikis_IncludesPlainDocs(t *testing.T) {
	dir := t.TempDir()

	// plain doc: only id + title, no workflow fields.
	plain := filepath.Join(dir, "PLAIN1.md")
	if err := os.WriteFile(plain, []byte("---\nid: PLAIN1\ntitle: just a doc\n---\n\n# A plain markdown doc\n"), 0o644); err != nil {
		t.Fatalf("write plain: %v", err)
	}

	// workflow doc: has explicit status/type/priority.
	workflow := filepath.Join(dir, "WORK01.md")
	if err := os.WriteFile(workflow, []byte("---\nid: WORK01\ntitle: work item\ntype: story\nstatus: ready\npriority: 1\n---\nwork body\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	all := s.GetAllTikis()
	if len(all) != 2 {
		t.Fatalf("GetAllTikis returned %d tikis, want 2 (plain + workflow)", len(all))
	}

	ids := map[string]bool{}
	for _, tk := range all {
		ids[tk.ID] = true
	}
	if !ids["PLAIN1"] {
		t.Error("plain doc should appear in GetAllTikis")
	}
	if !ids["WORK01"] {
		t.Error("workflow doc should appear in GetAllTikis")
	}

	// GetTiki still finds both by id.
	if tk := s.GetTiki("PLAIN1"); tk == nil {
		t.Error("GetTiki should find plain docs by id")
	} else if hasAnyWorkflowField(tk) {
		t.Error("plain doc should have no workflow fields")
	}
	if tk := s.GetTiki("WORK01"); tk == nil || !hasAnyWorkflowField(tk) {
		t.Error("workflow doc should have workflow fields")
	}
}

// TestGetAllTikis_CreatedTikiIsWorkflow verifies that CreateTiki with workflow
// fields marks the tiki as workflow-capable so programmatically-created tikis
// appear on boards.
func TestGetAllTikis_CreatedTikiIsWorkflow(t *testing.T) {
	dir := t.TempDir()
	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	if err := s.CreateTiki(createValidTikiForGateTest()); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	if got := len(s.GetAllTikis()); got != 1 {
		t.Errorf("GetAllTikis after CreateTiki: got %d, want 1", got)
	}
}
