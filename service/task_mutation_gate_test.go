package service

import (
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

	if err := gate.CreateTask(tk); err != nil {
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

	if err := gate.CreateTask(tk); err != nil {
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
	gate.OnCreate(func(tk *task.Task) *Rejection {
		return &Rejection{Reason: "blocked"}
	})

	tk := &task.Task{
		ID:       "TIKI-ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}

	err := gate.CreateTask(tk)
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
	if err := gate.UpdateTask(tk); err != nil {
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
	gate.OnUpdate(func(tk *task.Task) *Rejection {
		if tk.Title == "bad" {
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
	err := gate.UpdateTask(modified)
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

	if err := gate.DeleteTask(tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.GetTask("TIKI-ABC123") != nil {
		t.Error("task should have been deleted")
	}
}

func TestDeleteTask_RejectedByValidator(t *testing.T) {
	gate, s := newGateWithStore()
	gate.OnDelete(func(tk *task.Task) *Rejection {
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

	err := gate.DeleteTask(tk)
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

	gate.OnCreate(func(tk *task.Task) *Rejection {
		return &Rejection{Reason: "reason one"}
	})
	gate.OnCreate(func(tk *task.Task) *Rejection {
		return &Rejection{Reason: "reason two"}
	})

	tk := &task.Task{
		ID:       "TIKI-ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 3,
	}

	err := gate.CreateTask(tk)
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
	gate, _ := newGateWithStore()
	RegisterFieldValidators(gate)

	// task with invalid priority — should be rejected
	tk := &task.Task{
		ID:       "TIKI-ABC123",
		Title:    "test",
		Status:   task.StatusBacklog,
		Type:     task.TypeStory,
		Priority: 99,
	}

	err := gate.UpdateTask(tk)
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

	if err := gate.CreateTask(tk); err != nil {
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

	_ = gate.CreateTask(&task.Task{})
}

func TestCreateValidatorDoesNotAffectUpdate(t *testing.T) {
	gate, s := newGateWithStore()

	// register a validator only on create
	gate.OnCreate(func(tk *task.Task) *Rejection {
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
	if err := gate.UpdateTask(tk); err != nil {
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
	if err := gate.CreateTask(tk); err == nil {
		t.Fatal("expected rejection for empty title")
	}

	// a valid task should succeed
	tk.Title = "valid"
	if err := gate.CreateTask(tk); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
