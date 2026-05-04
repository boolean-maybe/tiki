package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestUpdateTiki_ZeroValueWritesOnlyTheFieldSet pins exact-presence
// semantics: setting a single schema-known field on a tiki that previously
// had none must produce a file that contains *only* that field — not the
// full schema-default block.
//
// In the tiki model, exact-presence gives this for free: setting only
// `points` on a cloned tiki means only that key is in Fields, so only
// that key is written to disk.
func TestUpdateTiki_ZeroValueWritesOnlyTheFieldSet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	// Seed a bare tiki on disk: frontmatter has only id and title.
	barePath := filepath.Join(tmp, "BARE01.md")
	src := "---\nid: BARE01\ntitle: just a note\n---\n\nbody\n"
	if err := os.WriteFile(barePath, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Precondition: loads with no schema-known fields.
	stored := s.GetTiki("BARE01")
	if stored == nil {
		t.Fatal("expected BARE01 to load")
	}
	if hasAnySchemaField(stored) {
		t.Fatalf("precondition: BARE01 must load with no schema fields, got presence=true")
	}

	// Set only points=0. Exact-presence: only this key lands on disk —
	// no status, type, priority, tags, etc.
	updated := stored.Clone()
	updated.Set(tikipkg.FieldPoints, 0)
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	// Read the file back: id/title plus a single `points: 0` line —
	// no full-schema leak.
	got, err := os.ReadFile(barePath)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	content := string(got)

	if !strings.Contains(content, "points: 0") {
		t.Errorf("expected `points: 0` in file, got:\n%s", content)
	}
	forbidden := []string{"status:", "priority:", "type:", "tags:", "dependsOn:", "due:", "recurrence:", "assignee:"}
	for _, key := range forbidden {
		if strings.Contains(content, key) {
			t.Errorf("setting points=0 leaked %q into file:\n%s", key, content)
		}
	}

	// And the value survives a reload. `points: 0` is a meaningful
	// "unestimated" sentinel — task.ValidatePoints accepts it — so the
	// load-side clamp must not silently rewrite it to maxPoints/2.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	reloaded := s.GetTiki("BARE01")
	if reloaded == nil {
		t.Fatal("GetTiki post-reload = nil")
	}
	if !reloaded.Has(tikipkg.FieldPoints) {
		t.Fatal("points key disappeared across reload")
	}
	if pts, _, _ := reloaded.IntField(tikipkg.FieldPoints); pts != 0 {
		t.Errorf("points round-trip: got %d, want 0 (the unestimated sentinel)", pts)
	}
}

// TestUpdateTiki_ZeroValueOnExistingFieldsAdds verifies the additive case:
// a tiki loaded with only `status` has Fields={status}. Running
// `set points = 0` (via clone+Set+UpdateTiki) must persist the new key
// alongside the existing status, not silently drop it.
//
// Clone the loaded tiki (Fields={status}), Set("points", 0) to grow Fields
// to {status, points}, then UpdateTiki. Exact-presence writes both keys
// and nothing else.
func TestUpdateTiki_ZeroValueOnExistingFieldsAdds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	// Seed a tiki with `status` only. Use backlog so this test is
	// independent of any test that mutates the active status registry —
	// `backlog` is in the default registry, so StatusToString round-trips
	// it verbatim.
	statusPath := filepath.Join(tmp, "STAT01.md")
	src := "---\nid: STAT01\ntitle: status-only doc\nstatus: backlog\n---\n\nbody\n"
	if err := os.WriteFile(statusPath, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	stored := s.GetTiki("STAT01")
	if stored == nil {
		t.Fatal("expected STAT01 to load")
	}
	if !hasAnySchemaField(stored) {
		t.Fatal("precondition: STAT01 should load with status present")
	}

	// Clone, then add points=0 to the existing Fields set. Both status
	// and points should be present on disk after save.
	updated := stored.Clone()
	updated.Set(tikipkg.FieldPoints, 0)
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	got, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	content := string(got)

	if !strings.Contains(content, "status: backlog") {
		t.Errorf("original status: backlog should survive; got:\n%s", content)
	}
	if !strings.Contains(content, "points: 0") {
		t.Errorf("explicit points: 0 assignment must be written to disk; got:\n%s", content)
	}
	forbidden := []string{"priority:", "type:", "tags:", "dependsOn:", "due:", "recurrence:", "assignee:"}
	for _, key := range forbidden {
		if strings.Contains(content, key) {
			t.Errorf("save leaked %q into file:\n%s", key, content)
		}
	}
}

// TestUpdateTiki_EmptyListWritesOnlyTheFieldSet is the dependsOn=[] twin
// of TestUpdateTiki_ZeroValueWritesOnlyTheFieldSet. Setting only dependsOn
// on a bare tiki should produce a file with exactly that one schema key.
func TestUpdateTiki_EmptyListWritesOnlyTheFieldSet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	barePath := filepath.Join(tmp, "BARE02.md")
	src := "---\nid: BARE02\ntitle: note\n---\n\nbody\n"
	if err := os.WriteFile(barePath, []byte(src), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	stored := s.GetTiki("BARE02")
	if stored == nil {
		t.Fatal("expected BARE02 to load")
	}

	// Set only dependsOn (empty list).
	updated := stored.Clone()
	updated.Set(tikipkg.FieldDependsOn, []string{})
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	got, err := os.ReadFile(barePath)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	content := string(got)

	if !strings.Contains(content, "dependsOn:") {
		t.Errorf("expected `dependsOn:` in file, got:\n%s", content)
	}
	forbidden := []string{"status:", "priority:", "type:", "tags:", "points:", "due:", "recurrence:", "assignee:"}
	for _, key := range forbidden {
		if strings.Contains(content, key) {
			t.Errorf("setting dependsOn=[] leaked %q into file:\n%s", key, content)
		}
	}
}
