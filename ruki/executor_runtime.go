package ruki

import (
	"errors"
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/tiki"
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
//
// SelectedTaskIDs is the full set of currently selected task IDs. id()
// succeeds only when exactly one id is selected, ids() returns the whole set,
// and selected_count() returns len().
type ExecutionInput struct {
	SelectedTaskIDs []string
	CreateTemplate  *tiki.Tiki
	InputValue      interface{} // value returned by input() builtin
	HasInput        bool        // distinguishes nil from unset
	ChooseValue     string      // task ID returned by choose() builtin
	HasChoose       bool        // distinguishes empty from unset
}

// NewSingleSelectionInput returns an ExecutionInput carrying exactly one
// selected task id. Convenience for the common single-selection case.
func NewSingleSelectionInput(taskID string) ExecutionInput {
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		return ExecutionInput{}
	}
	return ExecutionInput{SelectedTaskIDs: []string{trimmed}}
}

// SelectionCount returns the number of non-empty selected task IDs.
func (in ExecutionInput) SelectionCount() int {
	n := 0
	for _, id := range in.SelectedTaskIDs {
		if strings.TrimSpace(id) != "" {
			n++
		}
	}
	return n
}

// HasSelection returns true when at least one task id is selected.
func (in ExecutionInput) HasSelection() bool {
	return in.SelectionCount() > 0
}

// SingleSelectedTaskID returns the sole selected id (and true) when the
// selection cardinality is exactly one. Empty strings in the slice are
// ignored. Returns ("", false) for zero or many selections.
func (in ExecutionInput) SingleSelectedTaskID() (string, bool) {
	var only string
	count := 0
	for _, id := range in.SelectedTaskIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		count++
		if count > 1 {
			return "", false
		}
		only = trimmed
	}
	if count != 1 {
		return "", false
	}
	return only, true
}

// SelectedTaskIDList returns a copy of the trimmed, non-empty selected task
// IDs suitable for use as a ruki list value returned by ids().
func (in ExecutionInput) SelectedTaskIDList() []string {
	if len(in.SelectedTaskIDs) == 0 {
		return nil
	}
	out := make([]string, 0, len(in.SelectedTaskIDs))
	for _, id := range in.SelectedTaskIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
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

// MissingSelectedTaskIDError reports plugin execution that uses id() but was
// invoked with no selected task id.
type MissingSelectedTaskIDError struct{}

func (e *MissingSelectedTaskIDError) Error() string {
	return "selected task id is required for plugin runtime when id() is used"
}

// AmbiguousSelectedTaskIDError reports plugin execution that uses scalar id()
// but was invoked with more than one selected task id. ids() should be used
// instead for multi-selection.
type AmbiguousSelectedTaskIDError struct {
	Count int
}

func (e *AmbiguousSelectedTaskIDError) Error() string {
	return fmt.Sprintf("id() requires exactly one selected task, got %d — use ids() for multi-selection", e.Count)
}

// MissingCreateTemplateError reports CREATE execution without required template.
type MissingCreateTemplateError struct{}

func (e *MissingCreateTemplateError) Error() string {
	return "create template is required for create execution"
}

// MissingInputValueError reports execution of input() without a provided value.
type MissingInputValueError struct{}

func (e *MissingInputValueError) Error() string {
	return "input value is required when input() is used"
}

// MissingChooseValueError reports execution of choose() without a provided value.
type MissingChooseValueError struct{}

func (e *MissingChooseValueError) Error() string {
	return "choose value is required when choose() is used"
}

var (
	// ErrRuntimeMismatch is used with errors.Is for runtime mismatch failures.
	ErrRuntimeMismatch = errors.New("runtime mismatch")
)
