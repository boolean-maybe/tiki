package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUpdateDocument_BodyEditDoesNotExpandSparseFrontmatter proves the
// current review finding is closed: a workflow doc loaded with only
// `status:` present must stay sparse on disk after a body-only
// UpdateDocument. Before the fix, saveTask always ran the full workflow
// schema and added `type:`, `priority:`, `points:` etc.
func TestUpdateDocument_BodyEditDoesNotExpandSparseFrontmatter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	sparsePath := filepath.Join(tmp, "SPARSE.md")
	// Only `status` among workflow keys; no type/priority/points/tags/etc.
	sparse := "---\nid: SPARSE\ntitle: sparse\nstatus: ready\n---\n\nbody v1\n"
	if err := os.WriteFile(sparsePath, []byte(sparse), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Body-only edit through the document API.
	doc := s.GetDocument("SPARSE")
	if doc == nil {
		t.Fatal("GetDocument: nil")
	}
	doc.Body = "body v2\n"
	if err := s.UpdateDocument(doc); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}

	data, err := os.ReadFile(sparsePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	contents := string(data)

	// Must keep status, must not acquire anything else.
	if !strings.Contains(contents, "status: ready") {
		t.Errorf("status dropped; contents:\n%s", contents)
	}
	for _, forbidden := range []string{"type:", "priority:", "points:", "tags:", "dependsOn:", "assignee:", "recurrence:", "due:"} {
		if strings.Contains(contents, forbidden) {
			t.Errorf("sparse doc gained %q workflow key after body edit; contents:\n%s",
				forbidden, contents)
		}
	}

	// And a reload must keep the doc classified as workflow with only
	// status present — not gain any defaulted keys the next time around.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	doc2 := s.GetDocument("SPARSE")
	if doc2 == nil {
		t.Fatal("GetDocument after reload: nil")
	}
	if _, has := doc2.Frontmatter["status"]; !has {
		t.Error("status missing after reload")
	}
	for _, k := range []string{"type", "priority", "points", "tags", "dependsOn", "assignee", "recurrence", "due"} {
		if _, has := doc2.Frontmatter[k]; has {
			t.Errorf("key %q reappeared after reload: %v", k, doc2.Frontmatter[k])
		}
	}
}

// TestUpdateTask_TypedEditOnSparseDocGrowsPresenceSetOnlyForEditedKey
// verifies the task-API path: ruki/UI callers set a typed field, and that
// field (and only that field) grows the presence set. Other absent workflow
// keys stay absent.
func TestUpdateTask_TypedEditOnSparseDocGrowsPresenceSetOnlyForEditedKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	sparsePath := filepath.Join(tmp, "SPARSE.md")
	sparse := "---\nid: SPARSE\ntitle: sparse\nstatus: ready\n---\n\nbody\n"
	if err := os.WriteFile(sparsePath, []byte(sparse), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Task-API edit: change priority (was absent from file, defaulted to 3
	// on load per the current load-path behavior). Simulate a ruki-style
	// caller that sets the typed field explicitly.
	loaded := s.GetTask("SPARSE").Clone()
	loaded.Priority = 1 // genuine user edit — was defaulted 3
	if err := s.UpdateTask(loaded); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	data, _ := os.ReadFile(sparsePath)
	contents := string(data)

	if !strings.Contains(contents, "status: ready") {
		t.Errorf("status dropped; contents:\n%s", contents)
	}
	if !strings.Contains(contents, "priority: 1") {
		t.Errorf("typed priority edit did not persist; contents:\n%s", contents)
	}
	for _, forbidden := range []string{"type:", "points:", "tags:", "dependsOn:", "assignee:", "recurrence:", "due:"} {
		if strings.Contains(contents, forbidden) {
			t.Errorf("untouched workflow key %q leaked after typed edit; contents:\n%s",
				forbidden, contents)
		}
	}
}

// TestSaveTask_NewInMemoryWorkflowTaskUsesFullSchema verifies the fallback
// path: a brand-new workflow task (created in code, no preserved
// frontmatter) serializes with the full workflow schema so boards and lists
// continue to see complete metadata. The sparse path only kicks in when
// there is a preserved presence set to honor.
func TestSaveTask_NewInMemoryWorkflowTaskUsesFullSchema(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk, err := s.NewTaskTemplate()
	if err != nil {
		t.Fatalf("NewTaskTemplate: %v", err)
	}
	tk.Title = "new one"
	if err := s.CreateTask(tk); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, tk.ID+".md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	contents := string(data)
	for _, required := range []string{"status:", "type:", "priority:"} {
		if !strings.Contains(contents, required) {
			t.Errorf("new workflow task missing required %q; contents:\n%s", required, contents)
		}
	}

}
