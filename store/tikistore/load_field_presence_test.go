package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestLoadTiki_BareFileLoadsWithEmptyFields pins the load-side of exact-
// presence: a tiki authored with only id and title in the frontmatter must
// load with an empty Fields map. A round-trip update of body content must
// keep it that way; the load path must not synthesize defaults.
func TestLoadTiki_BareFileLoadsWithEmptyFields(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	// Author the file directly to mirror what a user would write by hand.
	barePath := filepath.Join(tmp, "BARE01.md")
	src := "---\nid: BARE01\ntitle: bare doc\n---\n\njust markdown\n"
	if err := os.WriteFile(barePath, []byte(src), 0644); err != nil {
		t.Fatalf("write bare file: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk := s.GetTiki("BARE01")
	if tk == nil {
		t.Fatal("GetTiki returned nil")
	}
	if hasAnySchemaField(tk) {
		t.Errorf("bare tiki gained synthetic schema-known fields on load: %+v", tk.Fields)
	}

	// A round-trip body edit must keep the file shape identical: no
	// schema-known keys appear, since none were ever set.
	updated := tk.Clone()
	updated.Body = "edited body\n"
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	data, err := os.ReadFile(barePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	for _, forbidden := range []string{"status:", "type:", "priority:", "points:"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("bare doc gained %q schema key after body edit; contents:\n%s",
				forbidden, data)
		}
	}

	// And after a reload the in-memory tiki stays bare.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if reloaded := s.GetTiki("BARE01"); reloaded != nil && hasAnySchemaField(reloaded) {
		t.Error("bare doc gained schema-known fields across reload — disk must have leaked keys")
	}
}

// TestLoadTiki_PreservesAbsentSchemaKeys verifies that loading a sparse
// tiki (only `status`) does not promote-fill the absent schema keys. The
// preserved source-frontmatter subset is the authoritative presence set.
func TestLoadTiki_PreservesAbsentSchemaKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	sparse := "---\nid: SPARSE\ntitle: status only\nstatus: backlog\n---\n\nbody\n"
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
		t.Error("status should be present in tiki Fields after load")
	}
	for _, k := range []string{"type", "priority", "points", "tags", "dependsOn", "due", "recurrence", "assignee"} {
		if _, has := tk.Fields[k]; has {
			t.Errorf("absent schema-known key %q gained presence on load: %v",
				k, tk.Fields[k])
		}
	}
}

// TestNewTikiTemplate_AppliesDefaultsWhenWorkflowConfigured verifies the
// fallback path: an in-memory tiki built via NewTikiTemplate, when the
// active workflow has a default status, carries the workflow's declared
// defaults so that programmatic creation produces a usable item without
// every caller having to set fields by hand.
func TestNewTikiTemplate_AppliesDefaultsWhenWorkflowConfigured(t *testing.T) {
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
	// Template carries schema-known fields when a workflow default is set.
	if !hasAnySchemaField(tk) {
		t.Fatal("template tiki has no schema-known fields — workflow defaults should be applied")
	}
	if _, has := tk.Fields[tikipkg.FieldStatus]; !has {
		t.Error("template is missing status default")
	}
	if _, has := tk.Fields[tikipkg.FieldPriority]; !has {
		t.Error("template is missing priority default")
	}
}
