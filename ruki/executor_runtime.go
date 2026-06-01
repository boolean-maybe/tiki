package ruki

import (
	"errors"
	"fmt"
	"strings"
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
// Per-execution payload (e.g. selected tiki id, create template) is passed
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
// SelectedTikiIDs is the full set of currently selected tiki IDs. id()
// succeeds only when exactly one id is selected, ids() returns the whole set,
// and selected_count() returns len().
type ExecutionInput struct {
	SelectedTikiIDs []string
	CreateTemplate  Document
	InputValue      interface{} // value returned by input() builtin
	HasInput        bool        // distinguishes nil from unset
	ChooseValue     string      // tiki ID returned by choose() builtin
	HasChoose       bool        // distinguishes empty from unset
}

// NewSingleSelectionInput returns an ExecutionInput carrying exactly one
// selected tiki id. Convenience for the common single-selection case.
func NewSingleSelectionInput(tikiID string) ExecutionInput {
	trimmed := strings.TrimSpace(tikiID)
	if trimmed == "" {
		return ExecutionInput{}
	}
	return ExecutionInput{SelectedTikiIDs: []string{trimmed}}
}

// SelectionCount returns the number of non-empty selected tiki IDs.
func (in ExecutionInput) SelectionCount() int {
	n := 0
	for _, id := range in.SelectedTikiIDs {
		if strings.TrimSpace(id) != "" {
			n++
		}
	}
	return n
}

// HasSelection returns true when at least one tiki id is selected.
func (in ExecutionInput) HasSelection() bool {
	return in.SelectionCount() > 0
}

// SingleSelectedTikiID returns the sole selected id (and true) when the
// selection cardinality is exactly one. Empty strings in the slice are
// ignored. Returns ("", false) for zero or many selections.
func (in ExecutionInput) SingleSelectedTikiID() (string, bool) {
	var only string
	count := 0
	for _, id := range in.SelectedTikiIDs {
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

// SelectedTikiIDList returns a copy of the trimmed, non-empty selected tiki
// IDs suitable for use as a ruki list value returned by ids().
func (in ExecutionInput) SelectedTikiIDList() []string {
	if len(in.SelectedTikiIDs) == 0 {
		return nil
	}
	out := make([]string, 0, len(in.SelectedTikiIDs))
	for _, id := range in.SelectedTikiIDs {
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

// MissingSelectedTikiIDError reports plugin execution that uses a scalar
// selection builtin (id(), filepath()) but was invoked with no selected
// tiki id. BuiltinName names the calling builtin and defaults to "id" so
// existing call sites produce byte-identical messages.
type MissingSelectedTikiIDError struct {
	BuiltinName string
}

func (e *MissingSelectedTikiIDError) Error() string {
	name := e.BuiltinName
	if name == "" {
		name = "id"
	}
	return fmt.Sprintf("selected tiki id is required for plugin runtime when %s() is used", name)
}

// AmbiguousSelectedTikiIDError reports plugin execution that uses a scalar
// selection builtin but was invoked with more than one selected tiki id.
// BuiltinName names the scalar builtin (defaults to "id"); PluralName names
// the multi-selection counterpart suggested in the message (defaults to
// "ids").
type AmbiguousSelectedTikiIDError struct {
	BuiltinName string
	PluralName  string
	Count       int
}

func (e *AmbiguousSelectedTikiIDError) Error() string {
	scalar := e.BuiltinName
	if scalar == "" {
		scalar = "id"
	}
	plural := e.PluralName
	if plural == "" {
		plural = "ids"
	}
	return fmt.Sprintf("%s() requires exactly one selected tiki, got %d — use %s() for multi-selection", scalar, e.Count, plural)
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
