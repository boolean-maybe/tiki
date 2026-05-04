package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
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

func newWorkflowTiki(id, title string) *tikipkg.Tiki {
	tk := &tikipkg.Tiki{ID: id, Title: title}
	tk.Set(tikipkg.FieldStatus, "backlog")
	tk.Set(tikipkg.FieldType, "story")
	tk.Set(tikipkg.FieldPriority, 3)
	return tk
}

func TestCreateTiki_Success(t *testing.T) {
	gate, s := newGateWithStore()

	tk := newWorkflowTiki("ABC123", "test task")

	if err := gate.CreateTiki(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.GetTiki("ABC123") == nil {
		t.Fatal("tiki not persisted")
		return
	}

	if tk.CreatedAt.IsZero() {
		t.Error("CreatedAt not set")
	}
	if tk.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set")
	}
}

func TestCreateTiki_DoesNotOverwriteCreatedAt(t *testing.T) {
	// verify the gate does not zero an existing CreatedAt before passing to store.
	gate := NewTaskMutationGate()

	var passedCreatedAt time.Time
	past := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	spy := &spyStore{
		Store:    store.NewInMemoryStore(),
		onCreate: func(tk *tikipkg.Tiki) { passedCreatedAt = tk.CreatedAt },
	}
	gate.SetStore(spy)

	tk := newWorkflowTiki("ABC123", "test")
	tk.CreatedAt = past

	if err := gate.CreateTiki(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !passedCreatedAt.Equal(past) {
		t.Errorf("gate changed CreatedAt before passing to store: got %v, want %v", passedCreatedAt, past)
	}
}

// spyStore wraps a Store and calls hooks before delegating.
type spyStore struct {
	store.Store
	onCreate func(*tikipkg.Tiki)
}

func (s *spyStore) CreateTiki(tk *tikipkg.Tiki) error {
	if s.onCreate != nil {
		s.onCreate(tk)
	}
	return s.Store.CreateTiki(tk)
}

func TestCreateTiki_RejectedByValidator(t *testing.T) {
	gate, s := newGateWithStore()
	gate.OnCreate(func(_, _ *tikipkg.Tiki, _ []*tikipkg.Tiki) *Rejection {
		return &Rejection{Reason: "blocked"}
	})

	tk := newWorkflowTiki("ABC123", "test")

	err := gate.CreateTiki(context.Background(), tk)
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
	if s.GetTiki("ABC123") != nil {
		t.Error("tiki should not have been persisted")
	}
}

func TestUpdateTiki_Success(t *testing.T) {
	gate, s := newGateWithStore()

	tk := newWorkflowTiki("ABC123", "original")
	_ = s.CreateTiki(tk)

	tk.Title = "updated"
	if err := gate.UpdateTiki(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored := s.GetTiki("ABC123")
	if stored.Title != "updated" {
		t.Errorf("title not updated: got %q", stored.Title)
	}
	if tk.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set")
	}
}

func TestUpdateTiki_RejectedByValidator(t *testing.T) {
	gate, s := newGateWithStore()
	gate.OnUpdate(func(_, new *tikipkg.Tiki, _ []*tikipkg.Tiki) *Rejection {
		if new.Title == "bad" {
			return &Rejection{Reason: "title cannot be 'bad'"}
		}
		return nil
	})

	original := newWorkflowTiki("ABC123", "good")
	_ = s.CreateTiki(original)

	modified := original.Clone()
	modified.Title = "bad"
	err := gate.UpdateTiki(context.Background(), modified)
	if err == nil {
		t.Fatal("expected rejection")
	}

	stored := s.GetTiki("ABC123")
	if stored.Title != "good" {
		t.Errorf("tiki should not have been updated: got %q", stored.Title)
	}
}

func TestDeleteTiki_Success(t *testing.T) {
	gate, s := newGateWithStore()

	tk := newWorkflowTiki("ABC123", "to delete")
	_ = s.CreateTiki(tk)

	if err := gate.DeleteTiki(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.GetTiki("ABC123") != nil {
		t.Error("tiki should have been deleted")
	}
}

func TestDeleteTiki_RejectedByValidator(t *testing.T) {
	gate, s := newGateWithStore()
	gate.OnDelete(func(_, _ *tikipkg.Tiki, _ []*tikipkg.Tiki) *Rejection {
		return &Rejection{Reason: "cannot delete"}
	})

	tk := newWorkflowTiki("ABC123", "protected")
	_ = s.CreateTiki(tk)

	err := gate.DeleteTiki(context.Background(), tk)
	if err == nil {
		t.Fatal("expected rejection")
		return
	}

	if s.GetTiki("ABC123") == nil {
		t.Error("tiki should not have been deleted")
	}
}

func TestMultipleRejections(t *testing.T) {
	gate, _ := newGateWithStore()

	gate.OnCreate(func(_, _ *tikipkg.Tiki, _ []*tikipkg.Tiki) *Rejection {
		return &Rejection{Reason: "reason one"}
	})
	gate.OnCreate(func(_, _ *tikipkg.Tiki, _ []*tikipkg.Tiki) *Rejection {
		return &Rejection{Reason: "reason two"}
	})

	tk := newWorkflowTiki("ABC123", "test")

	err := gate.CreateTiki(context.Background(), tk)
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

	// create a valid tiki first so UpdateTiki can find it in the store
	valid := newWorkflowTiki("ABC123", "test")
	_ = s.CreateTiki(valid)

	// now try to update with invalid priority — should be rejected
	tk := valid.Clone()
	tk.Set(tikipkg.FieldPriority, 99)

	err := gate.UpdateTiki(context.Background(), tk)
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

	tk := newWorkflowTiki("ABC123", "valid task")

	if err := gate.CreateTiki(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestFieldValidators_AcceptPlainDocument exercises the Phase 7 contract
// through the mutation gate: a plain-doc template (no status, type, priority,
// points) must pass validation and persist successfully.
func TestFieldValidators_AcceptPlainDocument(t *testing.T) {
	gate, s := newGateWithStore()
	RegisterFieldValidators(gate)

	plain := &tikipkg.Tiki{ID: "PLAIN1", Title: "a note"}

	if err := gate.CreateTiki(context.Background(), plain); err != nil {
		t.Fatalf("plain-doc create rejected by gate: %v", err)
	}

	if got := s.GetTiki("PLAIN1"); got == nil {
		t.Error("plain doc was not persisted")
	}
}

// TestFieldValidators_RequireTitleEvenForPlainDocs ensures that document-level
// validators still fire on plain docs — a plain doc with an empty title must
// be rejected.
func TestFieldValidators_RequireTitleEvenForPlainDocs(t *testing.T) {
	gate, _ := newGateWithStore()
	RegisterFieldValidators(gate)

	missingTitle := &tikipkg.Tiki{ID: "PLAIN2"}

	if err := gate.CreateTiki(context.Background(), missingTitle); err == nil {
		t.Fatal("gate accepted plain doc with empty title; document-level validators should still run")
	}
}

// TestFieldValidators_AcceptSparseWorkflowCreate covers the presence-aware
// contract: a tiki that sets only priority but no status or type must pass
// validation because absent fields are not invalid.
func TestFieldValidators_AcceptSparseWorkflowCreate(t *testing.T) {
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

	sparse := &tikipkg.Tiki{ID: "SPARS1", Title: "sparse workflow item"}
	sparse.Set(tikipkg.FieldPriority, 1)

	if err := gate.CreateTiki(context.Background(), sparse); err != nil {
		t.Fatalf("sparse workflow create rejected by gate: %v", err)
	}
	if s.GetTiki("SPARS1") == nil {
		t.Error("sparse workflow doc was not persisted")
	}
}

func TestReadStore(t *testing.T) {
	gate, s := newGateWithStore()

	tk := newWorkflowTiki("ABC123", "test")
	_ = s.CreateTiki(tk)

	rs := gate.ReadStore()
	if rs.GetTiki("ABC123") == nil {
		t.Error("ReadStore should return tiki from underlying store")
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

	_ = gate.CreateTiki(context.Background(), &tikipkg.Tiki{})
}

func TestCreateValidatorDoesNotAffectUpdate(t *testing.T) {
	gate, s := newGateWithStore()

	// register a validator only on create
	gate.OnCreate(func(_, _ *tikipkg.Tiki, _ []*tikipkg.Tiki) *Rejection {
		return &Rejection{Reason: "create blocked"}
	})

	// update should still work
	tk := newWorkflowTiki("ABC123", "test")
	_ = s.CreateTiki(tk)

	tk.Title = "updated"
	if err := gate.UpdateTiki(context.Background(), tk); err != nil {
		t.Fatalf("update should not be affected by create validator: %v", err)
	}
}

func TestBuildGate(t *testing.T) {
	gate := BuildGate()
	s := store.NewInMemoryStore()
	gate.SetStore(s)

	// BuildGate registers field validators, so an invalid tiki should be rejected
	tk := &tikipkg.Tiki{ID: "ABC123"} // empty title → invalid
	tk.Set(tikipkg.FieldStatus, "backlog")
	tk.Set(tikipkg.FieldType, "story")
	tk.Set(tikipkg.FieldPriority, 3)
	if err := gate.CreateTiki(context.Background(), tk); err == nil {
		t.Fatal("expected rejection for empty title")
	}

	// a valid tiki should succeed
	tk.Title = "valid"
	if err := gate.CreateTiki(context.Background(), tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAfterHook_CalledWithCorrectOldNew(t *testing.T) {
	gate, s := newGateWithStore()

	tk := newWorkflowTiki("ABC123", "original")
	_ = s.CreateTiki(tk)

	var hookOld, hookNew *tikipkg.Tiki
	gate.OnAfterUpdate(func(_ context.Context, old, new *tikipkg.Tiki) error {
		hookOld = old
		hookNew = new
		return nil
	})

	updated := tk.Clone()
	updated.Title = "changed"
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
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

	tk := newWorkflowTiki("ABC123", "test")
	_ = s.CreateTiki(tk)

	gate.OnAfterUpdate(func(_ context.Context, _, _ *tikipkg.Tiki) error {
		return fmt.Errorf("hook error")
	})

	updated := tk.Clone()
	updated.Title = "new title"
	// error from after-hook should not propagate
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
		t.Fatalf("after-hook error should not propagate: %v", err)
	}

	// tiki should still be persisted
	stored := s.GetTiki("ABC123")
	if stored.Title != "new title" {
		t.Errorf("tiki should have been updated despite hook error, got %q", stored.Title)
	}
}

func TestAfterHook_CreateAndDelete(t *testing.T) {
	gate, s := newGateWithStore()

	var createCalled, deleteCalled bool
	gate.OnAfterCreate(func(_ context.Context, old, new *tikipkg.Tiki) error {
		createCalled = true
		if old != nil {
			t.Error("create after-hook: old should be nil")
		}
		if new == nil || new.Title != "new task" {
			t.Error("create after-hook: new should have title")
		}
		return nil
	})
	gate.OnAfterDelete(func(_ context.Context, old, new *tikipkg.Tiki) error {
		deleteCalled = true
		if old == nil || old.Title != "new task" {
			t.Error("delete after-hook: old should have title")
		}
		if new != nil {
			t.Error("delete after-hook: new should be nil")
		}
		return nil
	})

	tk := newWorkflowTiki("ABC123", "new task")
	if err := gate.CreateTiki(context.Background(), tk); err != nil {
		t.Fatalf("create error: %v", err)
	}
	if !createCalled {
		t.Error("create after-hook not called")
	}

	if err := gate.DeleteTiki(context.Background(), tk); err != nil {
		t.Fatalf("delete error: %v", err)
	}
	if !deleteCalled {
		t.Error("delete after-hook not called")
	}

	if s.GetTiki("ABC123") != nil {
		t.Error("tiki should have been deleted")
	}
}

func TestAfterHook_Ordering(t *testing.T) {
	gate, s := newGateWithStore()

	tk := newWorkflowTiki("ABC123", "test")
	_ = s.CreateTiki(tk)

	// hook A mutates a second tiki through the gate
	second := newWorkflowTiki("BBB222", "second")
	_ = s.CreateTiki(second)

	gate.OnAfterUpdate(func(ctx context.Context, _, new *tikipkg.Tiki) error {
		// only fire for the original trigger, not for the cascaded mutation
		if new.ID != "ABC123" {
			return nil
		}
		sec := s.GetTiki("BBB222")
		if sec == nil {
			return nil
		}
		upd := sec.Clone()
		upd.Title = "modified by hook A"
		return gate.UpdateTiki(ctx, upd)
	})

	// hook B checks that it sees hook A's mutation
	var hookBSawMutation bool
	gate.OnAfterUpdate(func(_ context.Context, _, _ *tikipkg.Tiki) error {
		sec := s.GetTiki("BBB222")
		if sec != nil && sec.Title == "modified by hook A" {
			hookBSawMutation = true
		}
		return nil
	})

	updated := tk.Clone()
	updated.Title = "trigger"
	if err := gate.UpdateTiki(context.Background(), updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hookBSawMutation {
		t.Error("hook B should see hook A's mutation in the store")
	}
}

func TestCreateTiki_DepthExceeded(t *testing.T) {
	gate, _ := newGateWithStore()

	ctx := withTriggerDepth(context.Background(), maxTriggerDepth+1)
	tk := newWorkflowTiki("DEPTH1", "test")
	err := gate.CreateTiki(ctx, tk)
	if err == nil {
		t.Fatal("expected depth exceeded error")
	}
	if !strings.Contains(err.Error(), "cascade depth exceeded") {
		t.Fatalf("expected cascade depth error, got: %v", err)
	}
}

func TestDeleteTiki_DepthExceeded(t *testing.T) {
	gate, s := newGateWithStore()

	tk := newWorkflowTiki("DEPTH2", "test")
	_ = s.CreateTiki(tk)

	ctx := withTriggerDepth(context.Background(), maxTriggerDepth+1)
	err := gate.DeleteTiki(ctx, tk)
	if err == nil {
		t.Fatal("expected depth exceeded error")
	}
	if !strings.Contains(err.Error(), "cascade depth exceeded") {
		t.Fatalf("expected cascade depth error, got: %v", err)
	}
}

func TestCreateTiki_StoreError(t *testing.T) {
	gate := NewTaskMutationGate()
	fs := &failingCreateStore{Store: store.NewInMemoryStore()}
	gate.SetStore(fs)

	tk := newWorkflowTiki("CRERR1", "test")
	err := gate.CreateTiki(context.Background(), tk)
	if err == nil {
		t.Fatal("expected store error")
	}
}

func TestUpdateTiki_StoreError(t *testing.T) {
	gate := NewTaskMutationGate()
	fs := &failingUpdateStore{Store: store.NewInMemoryStore(), failID: "UPERR1"}
	gate.SetStore(fs)

	tk := newWorkflowTiki("UPERR1", "test")
	_ = fs.CreateTiki(tk)

	updated := tk.Clone()
	updated.Title = "updated"
	err := gate.UpdateTiki(context.Background(), updated)
	if err == nil {
		t.Fatal("expected store error")
	}
}

// failingCreateStore fails CreateTiki
type failingCreateStore struct {
	store.Store
}

func (f *failingCreateStore) CreateTiki(_ *tikipkg.Tiki) error {
	return fmt.Errorf("simulated create failure")
}

// failingUpdateStore fails UpdateTiki for a specific ID
type failingUpdateStore struct {
	store.Store
	failID string
}

func (f *failingUpdateStore) UpdateTiki(tk *tikipkg.Tiki) error {
	if tk.ID == f.failID {
		return fmt.Errorf("simulated update failure")
	}
	return f.Store.UpdateTiki(tk)
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

func TestDeleteTiki_AlreadyDeleted(t *testing.T) {
	gate := NewTaskMutationGate()
	s := store.NewInMemoryStore()
	gate.SetStore(s)

	// delete a tiki that doesn't exist in store — should return nil gracefully
	phantom := &tikipkg.Tiki{ID: "GONE01", Title: "gone"}
	err := gate.DeleteTiki(context.Background(), phantom)
	if err != nil {
		t.Fatalf("expected nil for already-deleted tiki, got: %v", err)
	}
}

func TestUpdateTiki_TikiNotFound(t *testing.T) {
	gate := NewTaskMutationGate()
	s := store.NewInMemoryStore()
	gate.SetStore(s)

	missing := &tikipkg.Tiki{ID: "MISS01", Title: "missing"}
	err := gate.UpdateTiki(context.Background(), missing)
	if err == nil {
		t.Fatal("expected error for missing tiki")
	}
	if !strings.Contains(err.Error(), "task not found") {
		t.Fatalf("expected 'task not found' error, got: %v", err)
	}
}
