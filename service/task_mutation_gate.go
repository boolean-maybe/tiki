package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// Rejection is returned by a validator to deny a mutation.
type Rejection struct {
	Reason string
}

// RejectionError holds one or more rejections from validators.
type RejectionError struct {
	Rejections []Rejection
}

func (e *RejectionError) Error() string {
	if len(e.Rejections) == 1 {
		return e.Rejections[0].Reason
	}
	msgs := make([]string, len(e.Rejections))
	for i, r := range e.Rejections {
		msgs[i] = r.Reason
	}
	return "validation failed: " + strings.Join(msgs, "; ")
}

// MutationValidator inspects a task and optionally rejects the mutation.
type MutationValidator func(t *task.Task) *Rejection

// TaskMutationGate is the single gateway for all task mutations.
// All Create/Update/Delete/AddComment operations must go through this gate.
// Validators are registered per operation type and run before persistence.
type TaskMutationGate struct {
	store            store.Store
	createValidators []MutationValidator
	updateValidators []MutationValidator
	deleteValidators []MutationValidator
}

// NewTaskMutationGate creates a gate without a store.
// Call SetStore after store initialization. Validator registration
// is safe before SetStore — mutations are not.
func NewTaskMutationGate() *TaskMutationGate {
	return &TaskMutationGate{}
}

// SetStore wires the persistence layer into the gate.
func (g *TaskMutationGate) SetStore(s store.Store) {
	g.store = s
}

// ReadStore returns the underlying store as a read-only interface.
func (g *TaskMutationGate) ReadStore() store.ReadStore {
	g.ensureStore()
	return g.store
}

// OnCreate registers a validator that runs before CreateTask.
func (g *TaskMutationGate) OnCreate(v MutationValidator) {
	g.createValidators = append(g.createValidators, v)
}

// OnUpdate registers a validator that runs before UpdateTask.
func (g *TaskMutationGate) OnUpdate(v MutationValidator) {
	g.updateValidators = append(g.updateValidators, v)
}

// OnDelete registers a validator that runs before DeleteTask.
func (g *TaskMutationGate) OnDelete(v MutationValidator) {
	g.deleteValidators = append(g.deleteValidators, v)
}

// CreateTask validates the task, sets timestamps, and persists it.
func (g *TaskMutationGate) CreateTask(t *task.Task) error {
	g.ensureStore()
	if err := g.runValidators(g.createValidators, t); err != nil {
		return err
	}
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	return g.store.CreateTask(t)
}

// UpdateTask validates the task, sets UpdatedAt, and persists changes.
func (g *TaskMutationGate) UpdateTask(t *task.Task) error {
	g.ensureStore()
	if err := g.runValidators(g.updateValidators, t); err != nil {
		return err
	}
	t.UpdatedAt = time.Now()
	return g.store.UpdateTask(t)
}

// DeleteTask validates and removes a task.
// Receives the full task so delete validators can inspect it.
func (g *TaskMutationGate) DeleteTask(t *task.Task) error {
	g.ensureStore()
	if err := g.runValidators(g.deleteValidators, t); err != nil {
		return err
	}
	g.store.DeleteTask(t.ID)
	return nil
}

// AddComment adds a comment to a task.
// Returns an error if the task does not exist.
func (g *TaskMutationGate) AddComment(taskID string, comment task.Comment) error {
	g.ensureStore()
	if !g.store.AddComment(taskID, comment) {
		return fmt.Errorf("task not found: %s", taskID)
	}
	return nil
}

func (g *TaskMutationGate) runValidators(validators []MutationValidator, t *task.Task) error {
	var rejections []Rejection
	for _, v := range validators {
		if r := v(t); r != nil {
			rejections = append(rejections, *r)
		}
	}
	if len(rejections) > 0 {
		return &RejectionError{Rejections: rejections}
	}
	return nil
}

func (g *TaskMutationGate) ensureStore() {
	if g.store == nil {
		panic("TaskMutationGate: store not set — call SetStore before using mutations or ReadStore")
	}
}
