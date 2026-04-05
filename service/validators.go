package service

import (
	"github.com/boolean-maybe/tiki/task"
)

// RegisterFieldValidators registers standard field validators with the gate.
// Each validator runs on both create and update operations.
func RegisterFieldValidators(g *TaskMutationGate) {
	for _, fn := range task.AllValidators() {
		wrapped := wrapFieldValidator(fn)
		g.OnCreate(wrapped)
		g.OnUpdate(wrapped)
	}
}

func wrapFieldValidator(fn func(*task.Task) string) MutationValidator {
	return func(t *task.Task) *Rejection {
		if msg := fn(t); msg != "" {
			return &Rejection{Reason: msg}
		}
		return nil
	}
}
