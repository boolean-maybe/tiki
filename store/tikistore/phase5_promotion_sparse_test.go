package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestPhase5_PromotionWithZeroValueWritesSparseYAML proves the second review
// finding is closed: a ruki update that promotes a plain document by setting
// a workflow field to its zero/empty value (`points = 0`, `dependsOn = []`)
// must NOT fall through to the full-workflow-schema serializer — the file
// should contain only the field the caller explicitly set.
//
// In the tiki model, exact-presence semantics give us this for free: setting
// only `points` on a cloned plain tiki means only that key is in Fields, so
// only that key is written to disk.
func TestPhase5_PromotionWithZeroValueWritesSparseYAML(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	// Seed a plain document on disk: frontmatter has id and title only,
	// no workflow keys at all.
	plainPath := filepath.Join(tmp, "PLAINX.md")
	plain := "---\nid: PLAINX\ntitle: just a note\n---\n\nbody\n"
	if err := os.WriteFile(plainPath, []byte(plain), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Verify precondition: loads as plain (no workflow fields).
	stored := s.GetTiki("PLAINX")
	if stored == nil {
		t.Fatal("expected PLAINX to load")
	}
	if hasAnyWorkflowField(stored) {
		t.Fatalf("precondition: PLAINX must load as plain, got workflow=true")
	}

	// Promote by setting only points=0. Exact-presence: only this key lands
	// on disk — no status, type, priority, tags, etc.
	updated := stored.Clone()
	updated.Set(tikipkg.FieldPoints, 0)
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	// Read the file back. The sparse write path should produce a header
	// with id/title plus a single `points: 0` line — NOT status, type,
	// priority, tags, etc.
	got, err := os.ReadFile(plainPath)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	content := string(got)

	if !strings.Contains(content, "points: 0") {
		t.Errorf("expected `points: 0` in file, got:\n%s", content)
	}
	// These would appear only if buildWorkflowFrontmatter (full-schema) ran.
	forbidden := []string{"status:", "priority:", "type:", "tags:", "dependsOn:", "due:", "recurrence:", "assignee:"}
	for _, key := range forbidden {
		if strings.Contains(content, key) {
			t.Errorf("promotion with points=0 leaked %q into file:\n%s", key, content)
		}
	}
}

// TestPhase5_SparseWorkflowZeroAssignmentPersists closes a sparse-workflow
// edge case: a document loaded with only `status:` has Fields={status}. Running
// `set points = 0` must persist the new `points:` key, not silently drop it.
//
// In the tiki model: clone the loaded tiki (Fields={status}), Set("points", 0)
// to grow Fields to {status, points}, then UpdateTiki. Exact-presence writes
// both keys and nothing else.
func TestPhase5_SparseWorkflowZeroAssignmentPersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	// Seed a sparse workflow document on disk: frontmatter has `status` only.
	sparsePath := filepath.Join(tmp, "SPARSE.md")
	sparse := "---\nid: SPARSE\ntitle: sparse doc\nstatus: ready\n---\n\nbody\n"
	if err := os.WriteFile(sparsePath, []byte(sparse), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	stored := s.GetTiki("SPARSE")
	if stored == nil {
		t.Fatal("expected SPARSE to load")
	}
	if !hasAnyWorkflowField(stored) {
		t.Fatal("precondition: SPARSE should load as workflow (has status)")
	}

	// Clone, then add points=0 to the existing Fields set. Both status and
	// points should be present on disk after save.
	updated := stored.Clone()
	updated.Set(tikipkg.FieldPoints, 0)
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	// Read the file back. Both `status:` (original) and `points: 0`
	// (just assigned) must be present; nothing else should leak.
	got, err := os.ReadFile(sparsePath)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	content := string(got)

	if !strings.Contains(content, "status: ready") {
		t.Errorf("original status: ready should survive; got:\n%s", content)
	}
	if !strings.Contains(content, "points: 0") {
		t.Errorf("explicit points: 0 assignment must be written to disk; got:\n%s", content)
	}
	forbidden := []string{"priority:", "type:", "tags:", "dependsOn:", "due:", "recurrence:", "assignee:"}
	for _, key := range forbidden {
		if strings.Contains(content, key) {
			t.Errorf("sparse save leaked %q into file:\n%s", key, content)
		}
	}
}

// TestPhase5_PromotionWithEmptyListWritesSparseYAML is the dependsOn=[] twin
// of the points=0 test. Setting only dependsOn on a plain doc should produce
// a sparse file with just that one workflow key.
func TestPhase5_PromotionWithEmptyListWritesSparseYAML(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	plainPath := filepath.Join(tmp, "PLAINY.md")
	plain := "---\nid: PLAINY\ntitle: note\n---\n\nbody\n"
	if err := os.WriteFile(plainPath, []byte(plain), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	stored := s.GetTiki("PLAINY")
	if stored == nil {
		t.Fatal("expected PLAINY to load")
	}

	// Promote by setting only dependsOn (empty list).
	updated := stored.Clone()
	updated.Set(tikipkg.FieldDependsOn, []string{})
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	got, err := os.ReadFile(plainPath)
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
			t.Errorf("promotion with dependsOn=[] leaked %q into file:\n%s", key, content)
		}
	}
}
