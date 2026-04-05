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
		{Key: "in_progress", Label: "In Progress", Emoji: "⚙️", Active: true},
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
		ID:       "TIKI-ABC123",
		Title:    "test task",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}

	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.GetTask("TIKI-ABC123") == nil {
		t.Fatal("task not persisted")
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
		ID:        "TIKI-ABC123",
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
		ID:       "TIKI-ABC123",
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
	}
	if re.Rejections[0].Reason != "blocked" {
		t.Errorf("unexpected reason: %s", re.Rejections[0].Reason)
	}
	if s.GetTask("TIKI-ABC123") != nil {
		t.Error("task should not have been persisted")
	}
}

func TestUpdateTask_Success(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "TIKI-ABC123",
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

	stored := s.GetTask("TIKI-ABC123")
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
		ID:       "TIKI-ABC123",
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

	stored := s.GetTask("TIKI-ABC123")
	if stored.Title != "good" {
		t.Errorf("task should not have been updated: got %q", stored.Title)
	}
}

func TestDeleteTask_Success(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "TIKI-ABC123",
		Title:    "to delete",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	if err := gate.DeleteTask(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.GetTask("TIKI-ABC123") != nil {
		t.Error("task should have been deleted")
	}
}

func TestDeleteTask_RejectedByValidator(t *testing.T) {
	gate, s := newGateWithStore()
	gate.OnDelete(func(_, _ *task.Task, _ []*task.Task) *Rejection {
		return &Rejection{Reason: "cannot delete"}
	})

	tk := &task.Task{
		ID:       "TIKI-ABC123",
		Title:    "protected",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	err := gate.DeleteTask(context.Background(), tk)
	if err == nil {
		t.Fatal("expected rejection")
	}

	if s.GetTask("TIKI-ABC123") == nil {
		t.Error("task should not have been deleted")
	}
}

func TestAddComment_Success(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "TIKI-ABC123",
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
	if err := gate.AddComment("TIKI-ABC123", comment); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored := s.GetTask("TIKI-ABC123")
	if len(stored.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(stored.Comments))
	}
}

func TestAddComment_TaskNotFound(t *testing.T) {
	gate, _ := newGateWithStore()

	comment := task.Comment{ID: "c1", Author: "user", Text: "hello"}
	err := gate.AddComment("TIKI-NONEXIST", comment)
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
		ID:       "TIKI-ABC123",
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
		ID:       "TIKI-ABC123",
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
		ID:       "TIKI-ABC123",
		Title:    "valid task",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}

	if err := gate.CreateTask(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadStore(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "TIKI-ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	rs := gate.ReadStore()
	if rs.GetTask("TIKI-ABC123") == nil {
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
		ID:       "TIKI-ABC123",
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
		ID:       "TIKI-ABC123",
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
		ID:       "TIKI-ABC123",
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
		ID:       "TIKI-ABC123",
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
	stored := s.GetTask("TIKI-ABC123")
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
		ID:       "TIKI-ABC123",
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

	if s.GetTask("TIKI-ABC123") != nil {
		t.Error("task should have been deleted")
	}
}

func TestAfterHook_Ordering(t *testing.T) {
	gate, s := newGateWithStore()

	tk := &task.Task{
		ID:       "TIKI-ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(tk)

	// hook A mutates a second task through the gate
	second := &task.Task{
		ID:       "TIKI-BBB222",
		Title:    "second",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}
	_ = s.CreateTask(second)

	gate.OnAfterUpdate(func(ctx context.Context, _, new *task.Task) error {
		// only fire for the original trigger, not for the cascaded mutation
		if new.ID != "TIKI-ABC123" {
			return nil
		}
		sec := s.GetTask("TIKI-BBB222")
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
		sec := s.GetTask("TIKI-BBB222")
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
	phantom := &task.Task{ID: "TIKI-GONE01", Title: "gone"}
	err := gate.DeleteTask(context.Background(), phantom)
	if err != nil {
		t.Fatalf("expected nil for already-deleted task, got: %v", err)
	}
}

func TestUpdateTask_TaskNotFound(t *testing.T) {
	gate := NewTaskMutationGate()
	s := store.NewInMemoryStore()
	gate.SetStore(s)

	missing := &task.Task{ID: "TIKI-MISS01", Title: "missing"}
	err := gate.UpdateTask(context.Background(), missing)
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if !strings.Contains(err.Error(), "task not found") {
		t.Fatalf("expected 'task not found' error, got: %v", err)
	}
}
