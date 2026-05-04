package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestGetDocument_PlainDocRemainsPlainAfterReload proves review finding #1
// is closed: a plain tiki loaded from disk must have an empty Fields map
// (no workflow keys), so a caller that does UpdateTiki(GetTiki(id)) does NOT
// promote it.
//
// Before the fix, loadTaskFile assigned default Priority/Points/Status to
// every task (including plain docs), and ToDocument then synthesized
// workflow frontmatter from those non-zero typed values — re-promoting the
// doc on the next round.
func TestGetDocument_PlainDocRemainsPlainAfterReload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	// Write a plain doc file directly; bypass the create API to match what a
	// user would author by hand.
	plainPath := filepath.Join(tmp, "PLAIN1.md")
	plain := "---\nid: PLAIN1\ntitle: plain doc\n---\n\njust markdown\n"
	if err := os.WriteFile(plainPath, []byte(plain), 0644); err != nil {
		t.Fatalf("write plain file: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk := s.GetTiki("PLAIN1")
	if tk == nil {
		t.Fatal("GetTiki returned nil")
	}
	if hasAnyWorkflowField(tk) {
		t.Errorf("plain tiki leaked workflow fields: %+v", tk.Fields)
	}

	// And the round-trip update must keep it plain and keep the disk YAML
	// free of workflow keys.
	updated := tk.Clone()
	updated.Body = "edited body\n"
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	data, err := os.ReadFile(plainPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	for _, forbidden := range []string{"status:", "type:", "priority:", "points:"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("plain doc gained %q workflow key after body edit; contents:\n%s",
				forbidden, data)
		}
	}

	// And after a reload the memory-resident tiki stays plain.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if reloaded := s.GetTiki("PLAIN1"); reloaded != nil && hasAnyWorkflowField(reloaded) {
		t.Error("plain doc re-promoted across reload — disk state still leaks workflow keys")
	}
}

// TestGetDocument_PreservesAbsentWorkflowKeys proves review finding #2 is
// closed: a workflow tiki whose source YAML carries only `status` must not
// acquire synthetic `type`, `priority`, or `points` keys. The preserved
// source-frontmatter subset is the authoritative presence set.
func TestGetDocument_PreservesAbsentWorkflowKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	sparse := "---\nid: SPARSE\ntitle: status only\nstatus: ready\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(tmp, "SPARSE.md"), []byte(sparse), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk := s.GetTiki("SPARSE")
	if tk == nil {
		t.Fatal("GetTiki returned nil")
	}
	if _, has := tk.Fields[tikipkg.FieldStatus]; !has {
		t.Error("status should be present in tiki Fields")
	}
	for _, k := range []string{"type", "priority", "points", "tags", "dependsOn", "due", "recurrence", "assignee"} {
		if _, has := tk.Fields[k]; has {
			t.Errorf("absent workflow key %q leaked into tiki Fields: %v",
				k, tk.Fields[k])
		}
	}
}

// TestNewDocumentTemplate_EmitsDefaultsFromSynthesis proves the fallback
// path: an in-memory workflow tiki built via NewTikiTemplate has no
// preserved frontmatter (it came from code, not disk), so projection falls
// back to typed-value synthesis. This is the "workflow creation path
// applies defaults" rule from the plan and must keep working.
func TestNewDocumentTemplate_EmitsDefaultsFromSynthesis(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk, err := s.NewTikiTemplate()
	if err != nil {
		t.Fatalf("NewTikiTemplate: %v", err)
	}
	// Template must be workflow-classified and carry synthesized defaults.
	if !hasAnyWorkflowField(tk) {
		t.Fatal("template tiki has no workflow fields — defaults should be synthesized")
	}
	if _, has := tk.Fields[tikipkg.FieldStatus]; !has {
		t.Error("template is missing status default")
	}
	if _, has := tk.Fields[tikipkg.FieldPriority]; !has {
		t.Error("template is missing priority default")
	}
}
