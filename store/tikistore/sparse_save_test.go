package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestUpdateDocument_BodyEditDoesNotExpandSparseFrontmatter proves the
// current review finding is closed: a workflow tiki loaded with only
// `status:` present must stay sparse on disk after a body-only UpdateTiki.
// Before the fix, saveTask always ran the full workflow schema and added
// `type:`, `priority:`, `points:` etc.
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

	// Body-only edit through the tiki API.
	tk := s.GetTiki("SPARSE")
	if tk == nil {
		t.Fatal("GetTiki: nil")
	}
	updated := tk.Clone()
	updated.Body = "body v2\n"
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
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

	// And a reload must keep the tiki classified as workflow with only
	// status present — not gain any defaulted keys the next time around.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	tk2 := s.GetTiki("SPARSE")
	if tk2 == nil {
		t.Fatal("GetTiki after reload: nil")
	}
	if _, has := tk2.Fields[tikipkg.FieldStatus]; !has {
		t.Error("status missing after reload")
	}
	for _, k := range []string{"type", "priority", "points", "tags", "dependsOn", "assignee", "recurrence", "due"} {
		if _, has := tk2.Fields[k]; has {
			t.Errorf("key %q reappeared after reload: %v", k, tk2.Fields[k])
		}
	}
}

// TestUpdateTiki_TypedEditOnSparseDocGrowsPresenceSetOnlyForEditedKey
// verifies the tiki-native path: setting a single typed field on a sparse
// workflow tiki grows the presence set for only that field. Other absent
// workflow keys stay absent.
func TestUpdateTiki_TypedEditOnSparseDocGrowsPresenceSetOnlyForEditedKey(t *testing.T) {
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

	// Tiki-native edit: change priority (was absent from file). Clone the
	// loaded tiki and set only priority — exact-presence writes only the
	// fields present in Fields.
	loaded := s.GetTiki("SPARSE")
	if loaded == nil {
		t.Fatal("GetTiki: nil")
	}
	updated := loaded.Clone()
	updated.Set(tikipkg.FieldPriority, 1)
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
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
// path: a brand-new workflow tiki (created in code, no preserved
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

	tk, err := s.NewTikiTemplate()
	if err != nil {
		t.Fatalf("NewTikiTemplate: %v", err)
	}
	tk.Title = "new one"
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, tk.ID+".md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	contents := string(data)
	for _, required := range []string{"status:", "type:", "priority:"} {
		if !strings.Contains(contents, required) {
			t.Errorf("new workflow tiki missing required %q; contents:\n%s", required, contents)
		}
	}
}
