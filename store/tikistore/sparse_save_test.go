package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestUpdateTiki_BodyEditPreservesExactFieldPresence pins the exact-
// presence rule against body edits: a tiki loaded with only `status:`
// among the schema-known keys must stay shaped that way on disk after a
// body-only UpdateTiki. Pre-Phase-3 saveTask always ran the full schema
// and added type/priority/points/...
func TestUpdateTiki_BodyEditPreservesExactFieldPresence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	sparsePath := filepath.Join(tmp, "SPARSE.md")
	// Only `status` among schema-known keys; no type/priority/points/tags/etc.
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

	// And a reload must keep the field presence set unchanged — only
	// status should be present, not any defaulted keys.
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
// verifies the tiki-native path: setting a single typed field on a sparsely-
// populated tiki grows the presence set for only that field. Other absent
// schema-known keys stay absent.
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
			t.Errorf("untouched schema-known key %q leaked after typed edit; contents:\n%s",
				forbidden, contents)
		}
	}
}

// TestCreateTiki_TemplateDerivedTikiSerializesWithDefaultedFields verifies
// the fallback path: a brand-new tiki built via NewTikiTemplate carries
// the full set of workflow-declared defaults in Fields, so its first save
// emits status/type/priority. Defaults flow from the template, not from
// any save-time synthesis.
func TestCreateTiki_TemplateDerivedTikiSerializesWithDefaultedFields(t *testing.T) {
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
			t.Errorf("template-derived tiki missing required %q; contents:\n%s", required, contents)
		}
	}
}
