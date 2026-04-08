package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// triggerDepthKey is the context key for tracking trigger cascade depth.
type triggerDepthKey struct{}

// triggerDepth returns the current trigger cascade depth from the context.
// Returns 0 if no depth has been set (root mutation) or if ctx is nil.
func triggerDepth(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	if v, ok := ctx.Value(triggerDepthKey{}).(int); ok {
		return v
	}
	return 0
}

// withTriggerDepth returns a derived context with the given trigger cascade depth.
// Falls back to context.Background() if ctx is nil.
func withTriggerDepth(ctx context.Context, depth int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, triggerDepthKey{}, depth)
}

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

// MutationValidator inspects a mutation and optionally rejects it.
// For create: old=nil, new=proposed task.
// For update: old=current persisted version (cloned), new=proposed version.
// For delete: old=task being deleted, new=nil.
type MutationValidator func(old, new *task.Task, allTasks []*task.Task) *Rejection

// AfterHook runs after a successful mutation for side effects (e.g. trigger cascades).
// Hooks receive the context (with trigger depth), old and new task snapshots.
// Errors are logged but do not propagate — the original mutation is not affected.
type AfterHook func(ctx context.Context, old, new *task.Task) error

// TaskMutationGate is the single gateway for all task mutations.
// All Create/Update/Delete/AddComment operations must go through this gate.
// Validators are registered per operation type and run before persistence.
// After-hooks run post-persist for side effects; their errors are logged, not propagated.
type TaskMutationGate struct {
	store            store.Store
	createValidators []MutationValidator
	updateValidators []MutationValidator
	deleteValidators []MutationValidator
	afterCreateHooks []AfterHook
	afterUpdateHooks []AfterHook
	afterDeleteHooks []AfterHook
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

// OnAfterCreate registers a hook that runs after a successful CreateTask.
func (g *TaskMutationGate) OnAfterCreate(h AfterHook) {
	g.afterCreateHooks = append(g.afterCreateHooks, h)
}

// OnAfterUpdate registers a hook that runs after a successful UpdateTask.
func (g *TaskMutationGate) OnAfterUpdate(h AfterHook) {
	g.afterUpdateHooks = append(g.afterUpdateHooks, h)
}

// OnAfterDelete registers a hook that runs after a successful DeleteTask.
func (g *TaskMutationGate) OnAfterDelete(h AfterHook) {
	g.afterDeleteHooks = append(g.afterDeleteHooks, h)
}

// CreateTask validates the task, sets timestamps, persists it, and runs after-hooks.
func (g *TaskMutationGate) CreateTask(ctx context.Context, t *task.Task) error {
	if err := checkTriggerDepth(ctx); err != nil {
		return err
	}
	g.ensureStore()
	allTasks := append(g.store.GetAllTasks(), t)
	if err := g.runValidators(g.createValidators, nil, t, allTasks); err != nil {
		return err
	}
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if err := g.store.CreateTask(t); err != nil {
		return err
	}
	g.runAfterHooks(ctx, g.afterCreateHooks, nil, t.Clone())
	return nil
}

// UpdateTask validates the task, sets UpdatedAt, persists changes, and runs after-hooks.
func (g *TaskMutationGate) UpdateTask(ctx context.Context, t *task.Task) error {
	if err := checkTriggerDepth(ctx); err != nil {
		return err
	}
	g.ensureStore()
	raw := g.store.GetTask(t.ID)
	if raw == nil {
		return fmt.Errorf("task not found: %s", t.ID)
	}
	old := raw.Clone()
	allTasks := g.candidateAllTasks(t)
	if err := g.runValidators(g.updateValidators, old, t, allTasks); err != nil {
		return err
	}
	t.UpdatedAt = time.Now()
	if err := g.store.UpdateTask(t); err != nil {
		return err
	}
	g.runAfterHooks(ctx, g.afterUpdateHooks, old, t.Clone())
	return nil
}

// DeleteTask validates, removes a task, and runs after-hooks.
// Receives the full task so delete validators can inspect it.
func (g *TaskMutationGate) DeleteTask(ctx context.Context, t *task.Task) error {
	if err := checkTriggerDepth(ctx); err != nil {
		return err
	}
	g.ensureStore()
	raw := g.store.GetTask(t.ID)
	if raw == nil {
		// task already gone — skip
		return nil
	}
	old := raw.Clone()
	allTasks := g.store.GetAllTasks()
	if err := g.runValidators(g.deleteValidators, old, nil, allTasks); err != nil {
		return err
	}
	g.store.DeleteTask(t.ID)
	g.runAfterHooks(ctx, g.afterDeleteHooks, old, nil)
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

// candidateAllTasks returns a snapshot of all tasks with the proposed update
// applied. This lets before-update validators evaluate aggregate predicates
// (e.g. WIP limits via count(select ...)) against the candidate world state
// rather than the stale pre-mutation snapshot.
func (g *TaskMutationGate) candidateAllTasks(proposed *task.Task) []*task.Task {
	stored := g.store.GetAllTasks()
	result := make([]*task.Task, len(stored))
	for i, t := range stored {
		if t.ID == proposed.ID {
			result[i] = proposed
		} else {
			result[i] = t
		}
	}
	return result
}

func (g *TaskMutationGate) runValidators(validators []MutationValidator, old, new *task.Task, allTasks []*task.Task) error {
	var rejections []Rejection
	for _, v := range validators {
		if r := v(old, new, allTasks); r != nil {
			rejections = append(rejections, *r)
		}
	}
	if len(rejections) > 0 {
		return &RejectionError{Rejections: rejections}
	}
	return nil
}

func (g *TaskMutationGate) runAfterHooks(ctx context.Context, hooks []AfterHook, old, new *task.Task) {
	for _, h := range hooks {
		if err := h(ctx, old, new); err != nil {
			slog.Error("after-hook failed", "error", err)
		}
	}
}

// checkTriggerDepth returns an error if the trigger cascade depth exceeds the limit.
func checkTriggerDepth(ctx context.Context) error {
	if triggerDepth(ctx) > maxTriggerDepth {
		return fmt.Errorf("trigger cascade depth exceeded (max %d)", maxTriggerDepth)
	}
	return nil
}

func (g *TaskMutationGate) ensureStore() {
	if g.store == nil {
		panic("TaskMutationGate: store not set — call SetStore before using mutations or ReadStore")
	}
}
