package ruki

import (
	"errors"
	"fmt"

	"github.com/boolean-maybe/tiki/task"
)

// ExecutorRuntimeMode identifies the semantic/runtime environment in which
// a validated AST is intended to execute.
type ExecutorRuntimeMode string

const (
	ExecutorRuntimeCLI          ExecutorRuntimeMode = "cli"
	ExecutorRuntimePlugin       ExecutorRuntimeMode = "plugin"
	ExecutorRuntimeEventTrigger ExecutorRuntimeMode = "eventTrigger"
	ExecutorRuntimeTimeTrigger  ExecutorRuntimeMode = "timeTrigger"
)

// ExecutorRuntime configures executor identity/runtime semantics.
// Per-execution payload (e.g. selected task id, create template) is passed
// via ExecutionInput and is intentionally not part of this struct.
type ExecutorRuntime struct {
	Mode ExecutorRuntimeMode
}

// normalize returns a runtime with defaults applied.
func (r ExecutorRuntime) normalize() ExecutorRuntime {
	if r.Mode == "" {
		r.Mode = ExecutorRuntimeCLI
	}
	return r
}

// ExecutionInput carries per-execution payload that is not part of executor
// runtime identity.
type ExecutionInput struct {
	SelectedTaskID string
	CreateTemplate *task.Task
}

// RuntimeMismatchError reports execution with a wrapper validated for a
// different runtime mode.
type RuntimeMismatchError struct {
	ValidatedFor ExecutorRuntimeMode
	Runtime      ExecutorRuntimeMode
}

func (e *RuntimeMismatchError) Error() string {
	return fmt.Sprintf("validated runtime %q does not match executor runtime %q", e.ValidatedFor, e.Runtime)
}

func (e *RuntimeMismatchError) Unwrap() error { return ErrRuntimeMismatch }

// MissingSelectedTaskIDError reports plugin execution that requires selected id
// (due to syntactic id() usage) but did not receive it.
type MissingSelectedTaskIDError struct{}

func (e *MissingSelectedTaskIDError) Error() string {
	return "selected task id is required for plugin runtime when id() is used"
}

// MissingCreateTemplateError reports CREATE execution without required template.
type MissingCreateTemplateError struct{}

func (e *MissingCreateTemplateError) Error() string {
	return "create template is required for create execution"
}

var (
	// ErrRuntimeMismatch is used with errors.Is for runtime mismatch failures.
	ErrRuntimeMismatch = errors.New("runtime mismatch")
)
