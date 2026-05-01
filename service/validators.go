package service

import (
	"github.com/boolean-maybe/tiki/task"
)

// RegisterFieldValidators registers standard field validators with the gate.
// Every validator runs on every create and update. Gating lives inside each
// validator: workflow-field validators (ValidateStatus, ValidateType,
// ValidatePriority, ValidatePoints, ValidateDue, ValidateRecurrence) treat a
// zero value as absent and return success, matching the Phase 1
// presence-aware contract. This ensures sparse workflow creates succeed in
// no-default workflows and closes the IsWorkflow=false update-bypass hole
// where a caller's flag could skip validation of a present-and-invalid
// field — the store's workflow carry-forward would then persist the bad
// value.
func RegisterFieldValidators(g *TaskMutationGate) {
	for _, fn := range task.AllValidators() {
		wrapped := wrapFieldValidator(fn)
		g.OnCreate(wrapped)
		g.OnUpdate(wrapped)
	}
}

func wrapFieldValidator(fn func(*task.Task) string) MutationValidator {
	return func(old, new *task.Task, allTasks []*task.Task) *Rejection {
		// field validators only inspect the proposed task
		t := new
		if t == nil {
			t = old // delete case
		}
		if msg := fn(t); msg != "" {
			return &Rejection{Reason: msg}
		}
		return nil
	}
}
