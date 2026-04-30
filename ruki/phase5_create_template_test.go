package ruki

import (
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

// These tests lock in that ruki `create` preserves full-schema save
// semantics for create templates. promoteToWorkflow must NOT seed the
// presence map on a template whose IsWorkflow was already true with a
// nil WorkflowFrontmatter — otherwise the save path flips to sparse
// mode and drops every default (type/priority/points/tags/custom
// fields) the caller did not explicitly assign.

// TestPhase5_Create_TemplatePreservesNilPresenceMap is the tight unit
// check: `create title="x" status="ready"` starting from a template
// with IsWorkflow=true and WorkflowFrontmatter=nil must leave the
// presence map nil after the field assignment. The save path reads nil
// as "write the FULL workflow schema with all defaults".
func TestPhase5_Create_TemplatePreservesNilPresenceMap(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	// Mimic what NewTaskTemplate returns: workflow-capable with
	// registry-supplied defaults and a nil presence map.
	template := &task.Task{
		ID:         "NEWDOC",
		Status:     "backlog",
		Type:       "story",
		Priority:   3,
		Points:     1,
		Tags:       []string{"idea"},
		IsWorkflow: true,
		// WorkflowFrontmatter intentionally nil — that's the signal to
		// marshalFrontmatter to write the full schema on save.
	}

	stmt, err := p.ParseStatement(`create title="x" status="ready"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, nil, ExecutionInput{CreateTemplate: template})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Create == nil || result.Create.Task == nil {
		t.Fatal("expected Create result with task")
	}
	created := result.Create.Task

	// The typed fields should carry BOTH the caller's override and the
	// template's defaults.
	if string(created.Status) != "ready" {
		t.Errorf("status = %q, want ready (caller override)", created.Status)
	}
	if string(created.Type) != "story" {
		t.Errorf("type = %q, want story (template default)", created.Type)
	}
	if created.Priority != 3 {
		t.Errorf("priority = %d, want 3 (template default)", created.Priority)
	}
	if created.Points != 1 {
		t.Errorf("points = %d, want 1 (template default)", created.Points)
	}
	if len(created.Tags) != 1 || created.Tags[0] != "idea" {
		t.Errorf("tags = %v, want [idea] (template default)", created.Tags)
	}

	// CRITICAL: the presence map must still be nil so marshalFrontmatter
	// takes the full-schema branch. A non-nil map here is the regression
	// the reviewer caught — seeded by promoteToWorkflow on `set status`,
	// which would flip the save into sparse mode and drop type=story,
	// priority=3, points=1, tags=[idea].
	if created.WorkflowFrontmatter != nil {
		t.Errorf("create template should preserve WorkflowFrontmatter=nil so save writes the full schema; got %v", created.WorkflowFrontmatter)
	}
	if !created.IsWorkflow {
		t.Error("IsWorkflow should remain true")
	}
}

// TestPhase5_Create_PlainPromotionStillSeedsPresence guards the inverse
// case: a caller who explicitly constructs a *plain* doc and then
// "creates" it by setting a workflow field (not a common ruki path but
// exercised to prove the promoteToWorkflow dual-behavior still works)
// must still seed the presence map so sparse save takes effect. This
// ensures the fix scoped to "only seed when promoting plain" doesn't
// break the plain-promotion path Phase 5 originally added.
func TestPhase5_Create_PlainPromotionStillSeedsPresence(t *testing.T) {
	// Call promoteToWorkflow directly to pin the behavior — end-to-end
	// update-path coverage is already in
	// store/tikistore/phase5_promotion_sparse_test.go.
	plain := &task.Task{ID: "P1", IsWorkflow: false}
	promoteToWorkflow(plain, "status")

	if !plain.IsWorkflow {
		t.Fatal("promotion should flip IsWorkflow to true")
	}
	if plain.WorkflowFrontmatter == nil {
		t.Fatal("promotion of a plain doc must seed WorkflowFrontmatter for sparse save")
	}
	if _, ok := plain.WorkflowFrontmatter["status"]; !ok {
		t.Errorf("presence map should record the promoting field; got %v", plain.WorkflowFrontmatter)
	}
}

// TestPhase5_Create_AlreadyWorkflowNilMapStaysNil is the symmetric
// direct-unit complement: calling promoteToWorkflow on a task that is
// already workflow-capable with a nil presence map must leave the map
// nil, regardless of the field being promoted.
func TestPhase5_Create_AlreadyWorkflowNilMapStaysNil(t *testing.T) {
	template := &task.Task{ID: "T1", IsWorkflow: true} // nil WorkflowFrontmatter
	promoteToWorkflow(template, "status")

	if !template.IsWorkflow {
		t.Error("already-workflow task should remain workflow")
	}
	if template.WorkflowFrontmatter != nil {
		t.Errorf("already-workflow nil presence map must stay nil; got %v", template.WorkflowFrontmatter)
	}
}

// TestPhase5_Create_SparseWorkflowAddsNewFieldToPresenceMap covers the
// third case of promoteToWorkflow: an already-workflow task with an
// existing (sparse) presence map. Adding a new workflow field via
// setField must record the new key in the map so the sparse save path
// actually writes it, even when the assigned value is the typed zero
// (e.g. `set points = 0`).
//
// Without this, both the presence map and MergeTypedWorkflowDeltas
// skip zero/empty values, so the user's explicit assignment silently
// vanishes on the next save.
func TestPhase5_Create_SparseWorkflowAddsNewFieldToPresenceMap(t *testing.T) {
	// Simulate a document loaded from disk that wrote only `status:` in
	// its frontmatter — the load path produces WorkflowFrontmatter with
	// just that one key.
	sparse := &task.Task{
		ID:                  "T1",
		Status:              "ready",
		IsWorkflow:          true,
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	// User adds an explicit zero: `set points = 0`.
	promoteToWorkflow(sparse, "points")

	if !sparse.IsWorkflow {
		t.Error("IsWorkflow should remain true")
	}
	// Both keys must be present after the assignment so sparse save
	// writes both `status:` and `points:`.
	if _, ok := sparse.WorkflowFrontmatter["status"]; !ok {
		t.Errorf("status should stay in presence map; got %v", sparse.WorkflowFrontmatter)
	}
	if _, ok := sparse.WorkflowFrontmatter["points"]; !ok {
		t.Errorf("points must be added to presence map; without it, zero-value assignment vanishes on save. Got %v", sparse.WorkflowFrontmatter)
	}
}
