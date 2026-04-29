package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/store"
)

// TestGetDocument_PlainDocRemainsPainAfterReload proves review finding #1
// is closed: a plain doc loaded from disk must project through GetDocument
// with an empty Frontmatter, so a caller that does
// UpdateDocument(GetDocument(id)) does NOT promote it.
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

	doc := s.GetDocument("PLAIN1")
	if doc == nil {
		t.Fatal("GetDocument returned nil")
	}
	if doc.Frontmatter != nil {
		t.Errorf("plain doc leaked workflow frontmatter: %+v", doc.Frontmatter)
	}
	if document.IsWorkflowFrontmatter(doc.Frontmatter) {
		t.Error("projected Document classifies as workflow — should stay plain")
	}

	// And the round-trip update must keep it plain and keep the disk YAML
	// free of workflow keys.
	doc.Body = "edited body\n"
	if err := s.UpdateDocument(doc); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
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

	// And after a reload the memory-resident task stays plain.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if tk := s.GetTask("PLAIN1"); tk != nil && tk.IsWorkflow {
		t.Error("plain doc re-promoted across reload — disk state still leaks workflow keys")
	}
}

// TestGetDocument_PreservesAbsentWorkflowKeys proves review finding #2 is
// closed: a workflow doc whose source YAML carries only `status` must not
// acquire synthetic `type`, `priority`, or `points` keys when projected
// through GetDocument. The preserved source-frontmatter subset is the
// authoritative presence set.
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

	doc := s.GetDocument("SPARSE")
	if doc == nil {
		t.Fatal("GetDocument returned nil")
	}
	if _, has := doc.Frontmatter["status"]; !has {
		t.Error("status should be present in projected frontmatter")
	}
	for _, k := range []string{"type", "priority", "points", "tags", "dependsOn", "due", "recurrence", "assignee"} {
		if _, has := doc.Frontmatter[k]; has {
			t.Errorf("absent workflow key %q leaked into projected frontmatter: %v",
				k, doc.Frontmatter[k])
		}
	}
}

// TestNewDocumentTemplate_EmitsDefaultsFromSynthesis proves the fallback
// path: an in-memory workflow task built via NewDocumentTemplate has no
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

	doc, err := s.NewDocumentTemplate()
	if err != nil {
		t.Fatalf("NewDocumentTemplate: %v", err)
	}
	// Template must be workflow-classified and carry synthesized defaults.
	if doc.Frontmatter == nil {
		t.Fatal("template Document has nil Frontmatter — defaults should be synthesized")
	}
	if _, has := doc.Frontmatter["status"]; !has {
		t.Error("template is missing status default")
	}
	if _, has := doc.Frontmatter["priority"]; !has {
		t.Error("template is missing priority default")
	}

	// Ensure TikiStore still satisfies DocumentStore after all this wiring.
	var _ store.DocumentStore = s
}
