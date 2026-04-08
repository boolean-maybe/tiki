package ruki

import (
	"errors"
	"testing"
)

func TestExecutorRuntimeNormalize(t *testing.T) {
	tests := []struct {
		name     string
		mode     ExecutorRuntimeMode
		expected ExecutorRuntimeMode
	}{
		{"empty mode defaults to cli", "", ExecutorRuntimeCLI},
		{"cli preserved", ExecutorRuntimeCLI, ExecutorRuntimeCLI},
		{"plugin preserved", ExecutorRuntimePlugin, ExecutorRuntimePlugin},
		{"event trigger preserved", ExecutorRuntimeEventTrigger, ExecutorRuntimeEventTrigger},
		{"time trigger preserved", ExecutorRuntimeTimeTrigger, ExecutorRuntimeTimeTrigger},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ExecutorRuntime{Mode: tt.mode}
			got := r.normalize()
			if got.Mode != tt.expected {
				t.Errorf("normalize().Mode = %q, want %q", got.Mode, tt.expected)
			}
		})
	}
}

// TestExecutorRuntimeNormalizeDoesNotMutateReceiver verifies normalize returns
// a copy and leaves the original unchanged.
func TestExecutorRuntimeNormalizeDoesNotMutateReceiver(t *testing.T) {
	r := ExecutorRuntime{Mode: ""}
	normalized := r.normalize()
	if r.Mode != "" {
		t.Errorf("original mutated: Mode = %q, want %q", r.Mode, "")
	}
	if normalized.Mode != ExecutorRuntimeCLI {
		t.Errorf("normalized.Mode = %q, want %q", normalized.Mode, ExecutorRuntimeCLI)
	}
}

func TestRuntimeMismatchErrorMessage(t *testing.T) {
	tests := []struct {
		name         string
		validatedFor ExecutorRuntimeMode
		runtime      ExecutorRuntimeMode
		want         string
	}{
		{
			"plugin vs cli",
			ExecutorRuntimePlugin,
			ExecutorRuntimeCLI,
			`validated runtime "plugin" does not match executor runtime "cli"`,
		},
		{
			"event trigger vs time trigger",
			ExecutorRuntimeEventTrigger,
			ExecutorRuntimeTimeTrigger,
			`validated runtime "eventTrigger" does not match executor runtime "timeTrigger"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RuntimeMismatchError{
				ValidatedFor: tt.validatedFor,
				Runtime:      tt.runtime,
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimeMismatchErrorUnwrap(t *testing.T) {
	err := &RuntimeMismatchError{
		ValidatedFor: ExecutorRuntimePlugin,
		Runtime:      ExecutorRuntimeCLI,
	}

	if unwrapped := err.Unwrap(); unwrapped != ErrRuntimeMismatch {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, ErrRuntimeMismatch)
	}

	if !errors.Is(err, ErrRuntimeMismatch) {
		t.Error("errors.Is(err, ErrRuntimeMismatch) = false, want true")
	}
}

func TestMissingSelectedTaskIDErrorMessage(t *testing.T) {
	err := &MissingSelectedTaskIDError{}
	want := "selected task id is required for plugin runtime when id() is used"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestMissingCreateTemplateErrorMessage(t *testing.T) {
	err := &MissingCreateTemplateError{}
	want := "create template is required for create execution"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrorTypesWithErrorsAs(t *testing.T) {
	tests := []struct {
		name string
		err  error
		// check runs errors.As against the appropriate target type
		check func(error) bool
	}{
		{
			"RuntimeMismatchError",
			&RuntimeMismatchError{ValidatedFor: ExecutorRuntimeCLI, Runtime: ExecutorRuntimePlugin},
			func(err error) bool {
				var target *RuntimeMismatchError
				return errors.As(err, &target)
			},
		},
		{
			"MissingSelectedTaskIDError",
			&MissingSelectedTaskIDError{},
			func(err error) bool {
				var target *MissingSelectedTaskIDError
				return errors.As(err, &target)
			},
		},
		{
			"MissingCreateTemplateError",
			&MissingCreateTemplateError{},
			func(err error) bool {
				var target *MissingCreateTemplateError
				return errors.As(err, &target)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(tt.err) {
				t.Errorf("errors.As failed for %T", tt.err)
			}
		})
	}
}
