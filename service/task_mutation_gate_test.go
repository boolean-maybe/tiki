package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

func init() {
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "inProgress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	})
}

func newGateWithStore() (*TaskMutationGate, store.Store) {
	gate := NewTaskMutationGate()
	s := store.NewInMemoryStore()
	gate.SetStore(s)
	return gate, s
}

func TestCreateTask_Success(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "test task",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}

	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.GetTask("ABC123") == nil {
		t.Fatal("task not persisted")
		return
	}

	if tk.CreatedAt.IsZero() {
		t.Error("CreatedAt not set")
	}
	if tk.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set")
	}
}

func TestCreateTask_DoesNotOverwriteCreatedAt(t *testing.T) {
	// verify the gate does not zero an existing CreatedAt before passing to store.
	// note: the in-memory store unconditionally sets CreatedAt, so we test
	// the gate's behavior by checking the task state *before* store.CreateTask.
	gate := NewTaskMutationGate()

	var passedCreatedAt time.Time
	past := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	spy := &spyStore{
		Store:    store.NewInMemoryStore(),
		onCreate: func(tk *task.Task) { passedCreatedAt = tk.CreatedAt },
	}
	gate.SetStore(spy)

	tk := &task.Task{
		ID:        "ABC123",
		Title:     "test",
		Status:    task.StatusBacklog,
		Type:      task.TypeStory,
		Priority:  3,
		CreatedAt: past,
	}

	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !passedCreatedAt.Equal(past) {
		t.Errorf("gate changed CreatedAt before passing to store: got %v, want %v", passedCreatedAt, past)
	}
}

// spyStore wraps a Store and calls hooks before delegating.
type spyStore struct {
	store.Store
	onCreate func(*task.Task)
}

func (s *spyStore) CreateTask(tk *task.Task) error {
	if s.onCreate != nil {
		s.onCreate(tk)
	}
	return s.Store.CreateTask(tk)
}

func TestCreateTask_RejectedByValidator(t *testing.T) {
	gate, s := newGateWithStore()
	gate.OnCreate(func(_, _ *task.Task, _ []*task.Task) *Rejection {
		return &Rejection{Reason: "blocked"}
	})

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}

	err := gate.CreateTask(context.Background(), tk)
	if err == nil {
		t.Fatal("expected rejection error")
	}

	re, ok := err.(*RejectionError)
	if !ok {
		t.Fatalf("expected *RejectionError, got %T", err)
		return
	}
	if re.Rejections[0].Reason != "blocked" {
		t.Errorf("unexpected reason: %s", re.Rejections[0].Reason)
	}
	if s.GetTask("ABC123") != nil {
		t.Error("task should not have been persisted")
	}
}

func TestUpdateTask_Success(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "original",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	tk.Title = "updated"
	if err := gate.UpdateTask(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored := s.GetTask("ABC123")
	if stored.Title != "updated" {
		t.Errorf("title not updated: got %q", stored.Title)
	}
	if tk.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set")
	}
}

func TestUpdateTask_RejectedByValidator(t *testing.T) {
	gate, s := newGateWithStore()
	gate.OnUpdate(func(_, new *task.Task, _ []*task.Task) *Rejection {
		if new.Title == "bad" {
			return &Rejection{Reason: "title cannot be 'bad'"}
		}
		return nil
	})

	original := &task.Task{
		ID:       "ABC123",
		Title:    "good",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(original)

	// clone to avoid mutating the store's pointer
	modified := original.Clone()
	modified.Title = "bad"
	err := gate.UpdateTask(context.Background(), modified)
	if err == nil {
		t.Fatal("expected rejection")
	}

	stored := s.GetTask("ABC123")
	if stored.Title != "good" {
		t.Errorf("task should not have been updated: got %q", stored.Title)
	}
}

func TestDeleteTask_Success(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "to delete",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	if err := gate.DeleteTask(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.GetTask("ABC123") != nil {
		t.Error("task should have been deleted")
	}
}

func TestDeleteTask_RejectedByValidator(t *testing.T) {
	gate, s := newGateWithStore()
	gate.OnDelete(func(_, _ *task.Task, _ []*task.Task) *Rejection {
		return &Rejection{Reason: "cannot delete"}
	})

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "protected",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	err := gate.DeleteTask(context.Background(), tk)
	if err == nil {
		t.Fatal("expected rejection")
		return
	}

	if s.GetTask("ABC123") == nil {
		t.Error("task should not have been deleted")
	}
}

func TestAddComment_Success(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	comment := task.Comment{
		ID:     "c1",
		Author: "user",
		Text:   "hello",
	}
	if err := gate.AddComment("ABC123", comment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored := s.GetTask("ABC123")
	if len(stored.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(stored.Comments))
	}
}

func TestAddComment_TaskNotFound(t *testing.T) {
	gate, _ := newGateWithStore()

	comment := task.Comment{ID: "c1", Author: "user", Text: "hello"}
	err := gate.AddComment("NONEXI", comment)
	if err == nil {
		t.Fatal("expected error for missing task")
	}
}

func TestMultipleRejections(t *testing.T) {
	gate, _ := newGateWithStore()

	gate.OnCreate(func(_, _ *task.Task, _ []*task.Task) *Rejection {
		return &Rejection{Reason: "reason one"}
	})
	gate.OnCreate(func(_, _ *task.Task, _ []*task.Task) *Rejection {
		return &Rejection{Reason: "reason two"}
	})

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}

	err := gate.CreateTask(context.Background(), tk)
	if err == nil {
		t.Fatal("expected rejection error")
	}

	re, ok := err.(*RejectionError)
	if !ok {
		t.Fatalf("expected *RejectionError, got %T", err)
		return
	}
	if len(re.Rejections) != 2 {
		t.Fatalf("expected 2 rejections, got %d", len(re.Rejections))
	}

	errStr := re.Error()
	if !strings.Contains(errStr, "reason one") || !strings.Contains(errStr, "reason two") {
		t.Errorf("error should contain both reasons: %s", errStr)
	}
}

func TestSingleRejection_ErrorFormat(t *testing.T) {
	re := &RejectionError{
		Rejections: []Rejection{{Reason: "single reason"}},
	}
	if re.Error() != "single reason" {
		t.Errorf("expected plain reason, got %q", re.Error())
	}
}

func TestFieldValidators_RejectInvalidTask(t *testing.T) {
	gate, s := newGateWithStore()
	RegisterFieldValidators(gate)

	// create a valid task first so UpdateTask can find it in the store
	valid := &task.Task{
		ID:       "ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(valid)

	// now try to update with invalid priority — should be rejected
	tk := valid.Clone()
	tk.Priority = 99

	err := gate.UpdateTask(context.Background(), tk)
	if err == nil {
		t.Fatal("expected rejection for invalid priority")
	}

	re, ok := err.(*RejectionError)
	if !ok {
		t.Fatalf("expected *RejectionError, got %T", err)
		return
	}

	found := false
	for _, r := range re.Rejections {
		if strings.Contains(r.Reason, "priority") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected priority rejection, got: %v", re.Rejections)
	}
}

func TestFieldValidators_AcceptValidTask(t *testing.T) {
	gate, _ := newGateWithStore()
	RegisterFieldValidators(gate)

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "valid task",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}

	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestFieldValidators_AcceptPlainDocument exercises the Phase 7 contract
// through the mutation gate: a plain-doc template (no status, type, priority,
// points) must pass validation and persist successfully. This is the path
// piped input and ruki `create` take when the active workflow has no
// `default: true` status, so any regression here silently breaks capture
// for notes-only workflows.
func TestFieldValidators_AcceptPlainDocument(t *testing.T) {
	gate, s := newGateWithStore()
	RegisterFieldValidators(gate)

	plain := &task.Task{
		ID:         "PLAIN1",
		Title:      "a note",
		IsWorkflow: false,
	}

	if err := gate.CreateTask(context.Background(), plain); err != nil {
		t.Fatalf("plain-doc create rejected by gate: %v", err)
	}

	// The store was called (no in-mem filter prevented persistence). A plain
	// doc does not appear in GetAllTasks (which filters to workflow items)
	// but GetTask returns it by id.
	if got := s.GetTask("PLAIN1"); got == nil {
		t.Error("plain doc was not persisted")
	}
}

// TestFieldValidators_RequireTitleEvenForPlainDocs ensures that document-level
// validators still fire on plain docs — a plain doc with an empty title must
// be rejected. Otherwise the workflow-only skip would over-reach and let bad
// input through.
func TestFieldValidators_RequireTitleEvenForPlainDocs(t *testing.T) {
	gate, _ := newGateWithStore()
	RegisterFieldValidators(gate)

	missingTitle := &task.Task{
		ID:         "PLAIN2",
		IsWorkflow: false,
	}

	if err := gate.CreateTask(context.Background(), missingTitle); err == nil {
		t.Fatal("gate accepted plain doc with empty title; document-level validators should still run")
	}
}

// TestFieldValidators_AcceptSparseWorkflowCreate covers the Phase 1/5
// presence-aware contract under a no-default workflow: a ruki create that
// sets only `priority` promotes the template to workflow-capable but leaves
// `status` and `type` absent. The gate must validate the fields that are
// present (priority must be in range) and accept empty `status`/`type` as
// absent rather than invalid. Without this, `create title="x" priority=1`
// is rejected end-to-end in notes-only workflows, breaking the Phase 7 CLI
// contract.
func TestFieldValidators_AcceptSparseWorkflowCreate(t *testing.T) {
	// Swap to a workflow with no default status so the workflow can legally
	// produce workflow docs without a status set.
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "alpha", Label: "Alpha"},
		{Key: "done", Label: "Done", Done: true},
	})
	t.Cleanup(func() {
		config.ResetStatusRegistry([]workflow.StatusDef{
			{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
			{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
			{Key: "inProgress", Label: "In Progress", Emoji: "⚙️", Active: true},
			{Key: "done", Label: "Done", Emoji: "✅", Done: true},
		})
	})

	gate, s := newGateWithStore()
	RegisterFieldValidators(gate)

	sparse := &task.Task{
		ID:         "SPARS1",
		Title:      "sparse workflow item",
		Priority:   1,
		IsWorkflow: true,
	}

	if err := gate.CreateTask(context.Background(), sparse); err != nil {
		t.Fatalf("sparse workflow create rejected by gate: %v", err)
	}
	if s.GetTask("SPARS1") == nil {
		t.Error("sparse workflow doc was not persisted")
	}
}

// TestFieldValidators_RejectInvalidFieldOnPlainUpdate closes a bypass hole:
// before this test's fix, a caller could update an existing workflow task
// with `IsWorkflow=false, Priority=99` and the gate would skip ValidatePriority
// because the new task claimed to be plain. The store then carry-forwarded
// IsWorkflow=true and persisted the out-of-range priority. The validator
// must inspect present-and-invalid fields regardless of the caller's
// IsWorkflow flag.
func TestFieldValidators_RejectInvalidFieldOnPlainUpdate(t *testing.T) {
	gate, s := newGateWithStore()
	RegisterFieldValidators(gate)

	// seed an existing workflow task via the in-memory store (force-flips
	// IsWorkflow=true) so the carry-forward path is active on update.
	if err := s.CreateTask(&task.Task{
		ID:       "BYPASS",
		Title:    "seed",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// caller pretends to be a plain doc but supplies an out-of-range priority.
	bad := &task.Task{
		ID:         "BYPASS",
		Title:      "seed",
		Priority:   99,
		IsWorkflow: false,
	}
	if err := gate.UpdateTask(context.Background(), bad); err == nil {
		t.Fatal("gate accepted out-of-range priority on an IsWorkflow=false update against a workflow-flagged stored task")
	}
}

func TestReadStore(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	rs := gate.ReadStore()
	if rs.GetTask("ABC123") == nil {
		t.Error("ReadStore should return task from underlying store")
	}
}

func TestEnsureStore_Panics(t *testing.T) {
	gate := NewTaskMutationGate()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
	}()

	_ = gate.CreateTask(context.Background(), &task.Task{})
}

func TestCreateValidatorDoesNotAffectUpdate(t *testing.T) {
	gate, s := newGateWithStore()

	// register a validator only on create
	gate.OnCreate(func(_, _ *task.Task, _ []*task.Task) *Rejection {
		return &Rejection{Reason: "create blocked"}
	})

	// update should still work
	tk := &task.Task{
		ID:       "ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	tk.Title = "updated"
	if err := gate.UpdateTask(context.Background(), tk); err != nil {
		t.Fatalf("update should not be affected by create validator: %v", err)
	}
}

func TestBuildGate(t *testing.T) {
	gate := BuildGate()
	s := store.NewInMemoryStore()
	gate.SetStore(s)

	// BuildGate registers field validators, so an invalid task should be rejected
	tk := &task.Task{
		ID:       "ABC123",
		Title:    "", // invalid
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	if err := gate.CreateTask(context.Background(), tk); err == nil {
		t.Fatal("expected rejection for empty title")
	}

	// a valid task should succeed
	tk.Title = "valid"
	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAfterHook_CalledWithCorrectOldNew(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "original",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	var hookOld, hookNew *task.Task
	gate.OnAfterUpdate(func(_ context.Context, old, new *task.Task) error {
		hookOld = old
		hookNew = new
		return nil
	})

	updated := tk.Clone()
	updated.Title = "changed"
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hookOld == nil || hookOld.Title != "original" {
		t.Errorf("after-hook old should have original title, got %v", hookOld)
	}
	if hookNew == nil || hookNew.Title != "changed" {
		t.Errorf("after-hook new should have changed title, got %v", hookNew)
	}
}

func TestAfterHook_ErrorSwallowed(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	gate.OnAfterUpdate(func(_ context.Context, _, _ *task.Task) error {
		return fmt.Errorf("hook error")
	})

	updated := tk.Clone()
	updated.Title = "new title"
	// error from after-hook should not propagate
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("after-hook error should not propagate: %v", err)
	}

	// task should still be persisted
	stored := s.GetTask("ABC123")
	if stored.Title != "new title" {
		t.Errorf("task should have been updated despite hook error, got %q", stored.Title)
	}
}

func TestAfterHook_CreateAndDelete(t *testing.T) {
	gate, s := newGateWithStore()

	var createCalled, deleteCalled bool
	gate.OnAfterCreate(func(_ context.Context, old, new *task.Task) error {
		createCalled = true
		if old != nil {
			t.Error("create after-hook: old should be nil")
		}
		if new == nil || new.Title != "new task" {
			t.Error("create after-hook: new should have title")
		}
		return nil
	})
	gate.OnAfterDelete(func(_ context.Context, old, new *task.Task) error {
		deleteCalled = true
		if old == nil || old.Title != "new task" {
			t.Error("delete after-hook: old should have title")
		}
		if new != nil {
			t.Error("delete after-hook: new should be nil")
		}
		return nil
	})

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "new task",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatalf("create error: %v", err)
	}
	if !createCalled {
		t.Error("create after-hook not called")
	}

	if err := gate.DeleteTask(context.Background(), tk); err != nil {
		t.Fatalf("delete error: %v", err)
	}
	if !deleteCalled {
		t.Error("delete after-hook not called")
	}

	if s.GetTask("ABC123") != nil {
		t.Error("task should have been deleted")
	}
}

func TestAfterHook_Ordering(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	// hook A mutates a second task through the gate
	second := &task.Task{
		ID:       "BBB222",
		Title:    "second",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(second)

	gate.OnAfterUpdate(func(ctx context.Context, _, new *task.Task) error {
		// only fire for the original trigger, not for the cascaded mutation
		if new.ID != "ABC123" {
			return nil
		}
		sec := s.GetTask("BBB222")
		if sec == nil {
			return nil
		}
		upd := sec.Clone()
		upd.Title = "modified by hook A"
		return gate.UpdateTask(ctx, upd)
	})

	// hook B checks that it sees hook A's mutation
	var hookBSawMutation bool
	gate.OnAfterUpdate(func(_ context.Context, _, _ *task.Task) error {
		sec := s.GetTask("BBB222")
		if sec != nil && sec.Title == "modified by hook A" {
			hookBSawMutation = true
		}
		return nil
	})

	updated := tk.Clone()
	updated.Title = "trigger"
	if err := gate.UpdateTask(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hookBSawMutation {
		t.Error("hook B should see hook A's mutation in the store")
	}
}

func TestCreateTask_DepthExceeded(t *testing.T) {
	gate, _ := newGateWithStore()

	ctx := withTriggerDepth(context.Background(), maxTriggerDepth+1)
	tk := &task.Task{
		ID: "DEPTH1", Title: "test", Status: task.StatusBacklog,
		Type: task.TypeStory, Priority: 3,
	}
	err := gate.CreateTask(ctx, tk)
	if err == nil {
		t.Fatal("expected depth exceeded error")
	}
	if !strings.Contains(err.Error(), "cascade depth exceeded") {
		t.Fatalf("expected cascade depth error, got: %v", err)
	}
}

func TestDeleteTask_DepthExceeded(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID: "DEPTH2", Title: "test", Status: task.StatusBacklog,
		Type: task.TypeStory, Priority: 3,
	}
	_ = s.CreateTask(tk)

	ctx := withTriggerDepth(context.Background(), maxTriggerDepth+1)
	err := gate.DeleteTask(ctx, tk)
	if err == nil {
		t.Fatal("expected depth exceeded error")
	}
	if !strings.Contains(err.Error(), "cascade depth exceeded") {
		t.Fatalf("expected cascade depth error, got: %v", err)
	}
}

func TestCreateTask_StoreError(t *testing.T) {
	gate := NewTaskMutationGate()
	fs := &failingCreateStore{Store: store.NewInMemoryStore()}
	gate.SetStore(fs)

	tk := &task.Task{
		ID: "CRERR1", Title: "test", Status: task.StatusBacklog,
		Type: task.TypeStory, Priority: 3,
	}
	err := gate.CreateTask(context.Background(), tk)
	if err == nil {
		t.Fatal("expected store error")
	}
}

func TestUpdateTask_StoreError(t *testing.T) {
	gate := NewTaskMutationGate()
	fs := &failingUpdateStore{Store: store.NewInMemoryStore(), failID: "UPERR1"}
	gate.SetStore(fs)

	tk := &task.Task{
		ID: "UPERR1", Title: "test", Status: task.StatusBacklog,
		Type: task.TypeStory, Priority: 3,
	}
	_ = fs.CreateTask(tk)

	updated := tk.Clone()
	updated.Title = "updated"
	err := gate.UpdateTask(context.Background(), updated)
	if err == nil {
		t.Fatal("expected store error")
	}
}

// failingCreateStore fails CreateTask
type failingCreateStore struct {
	store.Store
}

func (f *failingCreateStore) CreateTask(_ *task.Task) error {
	return fmt.Errorf("simulated create failure")
}

// failingUpdateStore fails UpdateTask for a specific ID
type failingUpdateStore struct {
	store.Store
	failID string
}

func (f *failingUpdateStore) UpdateTask(t *task.Task) error {
	if t.ID == f.failID {
		return fmt.Errorf("simulated update failure")
	}
	return f.Store.UpdateTask(t)
}

func TestTriggerDepth_NilContext(t *testing.T) {
	// triggerDepth must not panic on nil context
	depth := triggerDepth(nil) //nolint:staticcheck // SA1012: intentionally testing nil-context safety
	if depth != 0 {
		t.Fatalf("expected 0, got %d", depth)
	}
}

func TestWithTriggerDepth_NilContext(t *testing.T) {
	// withTriggerDepth must not panic on nil context
	ctx := withTriggerDepth(nil, 3) //nolint:staticcheck // SA1012: intentionally testing nil-context safety
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if got := triggerDepth(ctx); got != 3 {
		t.Fatalf("expected depth 3, got %d", got)
	}
}

func TestDeleteTask_AlreadyDeleted(t *testing.T) {
	gate := NewTaskMutationGate()
	s := store.NewInMemoryStore()
	gate.SetStore(s)

	// delete a task that doesn't exist in store — should return nil gracefully
	phantom := &task.Task{ID: "GONE01", Title: "gone"}
	err := gate.DeleteTask(context.Background(), phantom)
	if err != nil {
		t.Fatalf("expected nil for already-deleted task, got: %v", err)
	}
}

func TestUpdateTask_TaskNotFound(t *testing.T) {
	gate := NewTaskMutationGate()
	s := store.NewInMemoryStore()
	gate.SetStore(s)

	missing := &task.Task{ID: "MISS01", Title: "missing"}
	err := gate.UpdateTask(context.Background(), missing)
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if !strings.Contains(err.Error(), "task not found") {
		t.Fatalf("expected 'task not found' error, got: %v", err)
	}
}
