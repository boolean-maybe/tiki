package task

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/workflow"
)

// Type is a type alias for workflow.TaskType.
// This preserves compatibility: task.Type and workflow.TaskType are the same type.
type Type = workflow.TaskType

// well-known built-in type constants.
const (
	TypeStory = workflow.TypeStory
	TypeBug   = workflow.TypeBug
	TypeSpike = workflow.TypeSpike
	TypeEpic  = workflow.TypeEpic
)

// defaultTypeRegistry is built once from the built-in type definitions.
// It serves as a fallback when config has not been initialized yet.
var defaultTypeRegistry = func() *workflow.TypeRegistry {
	reg, err := workflow.NewTypeRegistry(workflow.DefaultTypeDefs())
	if err != nil {
		panic(fmt.Sprintf("task: building default type registry: %v", err))
	}
	return reg
}()

// currentTypeRegistry returns the config-provided type registry when available,
// falling back to the package-level default built from DefaultTypeDefs().
func currentTypeRegistry() *workflow.TypeRegistry {
	if reg, ok := config.MaybeGetTypeRegistry(); ok {
		return reg
	}
	return defaultTypeRegistry
}

// ParseType parses a raw string into a Type with validation.
// Returns the canonical key and true if recognized (including aliases),
// or (TypeStory, false) for unknown types.
func ParseType(t string) (Type, bool) {
	return currentTypeRegistry().ParseType(t)
}

// NormalizeType standardizes a raw type string into a Type.
func NormalizeType(t string) Type {
	return currentTypeRegistry().NormalizeType(t)
}

// TypeLabel returns a human-readable label for a task type.
func TypeLabel(taskType Type) string {
	return currentTypeRegistry().TypeLabel(taskType)
}

// TypeEmoji returns the emoji for a task type.
func TypeEmoji(taskType Type) string {
	return currentTypeRegistry().TypeEmoji(taskType)
}

// TypeDisplay returns a formatted display string with label and emoji.
func TypeDisplay(taskType Type) string {
	return currentTypeRegistry().TypeDisplay(taskType)
}

// ParseDisplay reverses a TypeDisplay() string back to a canonical key.
// Returns (key, true) on match, or (fallback, false) for unrecognized display strings.
func ParseDisplay(display string) (Type, bool) {
	return currentTypeRegistry().ParseDisplay(display)
}

// AllTypes returns the ordered list of all configured type keys.
func AllTypes() []Type {
	return currentTypeRegistry().Keys()
}
