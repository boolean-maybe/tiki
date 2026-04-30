package tikistore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/document"
	taskpkg "github.com/boolean-maybe/tiki/task"
)

// TestPhase5_PromotionWithZeroValueWritesSparseYAML proves the second review
// finding is closed: a ruki update that promotes a plain document by setting
// a workflow field to its zero/empty value (`points = 0`, `dependsOn = []`)
// must NOT fall through to the full-workflow-schema serializer — the file
// should contain only the field the caller explicitly set.
//
// Pre-fix failure mode: promoteToWorkflow only flipped IsWorkflow, leaving
// WorkflowFrontmatter nil. MergeTypedWorkflowDeltas skipped zero values, so
// the presence map never grew. marshalFrontmatter then took the
// nil-WorkflowFrontmatter branch and called buildWorkflowFrontmatter, which
// wrote every workflow key with registry defaults.
//
// The fix makes promoteToWorkflow also seed WorkflowFrontmatter with the
// exact promoting key so marshalSparseWorkflowFrontmatter runs and emits
// only that key.
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

	// Start from the loaded task so FilePath/LoadedMtime are correct.
	loaded := s.GetDocument("PLAINX")
	if loaded == nil {
		t.Fatal("expected PLAINX to load")
	}
	if got := document.IsWorkflowFrontmatter(loaded.Frontmatter); got {
		t.Fatalf("precondition: PLAINX must load as plain, got workflow=%v", got)
	}

	// Simulate exactly what ruki setField does after `set points = 0` on a
	// plain doc: Points=0, IsWorkflow=true, WorkflowFrontmatter seeded with
	// the promoting key so marshal takes the sparse path.
	promoted := taskpkg.FromDocument(loaded)
	promoted.Points = 0
	promoted.IsWorkflow = true
	promoted.WorkflowFrontmatter = map[string]interface{}{"points": ""}

	if err := s.UpdateTask(promoted); err != nil {
		t.Fatalf("UpdateTask: %v", err)
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
// edge case: a document loaded with only `status:` has
// WorkflowFrontmatter={status}. Running `set points = 0` must persist
// the new `points:` key, not silently drop it on save.
//
// Pre-fix failure mode: after an earlier iteration scoped presence
// seeding to only plain-document promotion, already-workflow sparse
// tasks stopped seeding new keys. MergeTypedWorkflowDeltas already
// skips zero/empty values, so `points = 0` left both the presence map
// unchanged and the typed delta suppressed — the sparse save path then
// wrote only the original `status:` and the user's explicit `points: 0`
// assignment disappeared.
//
// The fix extends the seeding rule: if the presence map is already
// non-nil (sparse workflow doc), always record the assigned field so
// zero/empty values land on disk.
func TestPhase5_SparseWorkflowZeroAssignmentPersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	// Seed a sparse workflow document on disk: frontmatter has `status`
	// only. This is what the store produces after loading a file whose
	// frontmatter wrote a single workflow key.
	sparsePath := filepath.Join(tmp, "SPARSE.md")
	sparse := "---\nid: SPARSE\ntitle: sparse doc\nstatus: ready\n---\n\nbody\n"
	if err := os.WriteFile(sparsePath, []byte(sparse), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	loaded := s.GetDocument("SPARSE")
	if loaded == nil {
		t.Fatal("expected SPARSE to load")
	}
	if !document.IsWorkflowFrontmatter(loaded.Frontmatter) {
		t.Fatal("precondition: SPARSE should load as workflow (has status)")
	}

	// Simulate what ruki setField does after `set points = 0` on this
	// already-workflow-sparse doc: IsWorkflow already true, presence
	// map already has {status}, and the fix adds the `points` key.
	updated := taskpkg.FromDocument(loaded)
	updated.Points = 0
	if updated.WorkflowFrontmatter == nil {
		updated.WorkflowFrontmatter = map[string]interface{}{}
	}
	updated.WorkflowFrontmatter["points"] = ""

	if err := s.UpdateTask(updated); err != nil {
		t.Fatalf("UpdateTask: %v", err)
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
// of the points=0 test. Both values are "zero" from MergeTypedWorkflowDeltas'
// point of view, so both must seed the presence map at promotion time.
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

	loaded := s.GetDocument("PLAINY")
	if loaded == nil {
		t.Fatal("expected PLAINY to load")
	}

	promoted := taskpkg.FromDocument(loaded)
	promoted.DependsOn = nil // empty list
	promoted.IsWorkflow = true
	promoted.WorkflowFrontmatter = map[string]interface{}{"dependsOn": ""}

	if err := s.UpdateTask(promoted); err != nil {
		t.Fatalf("UpdateTask: %v", err)
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
