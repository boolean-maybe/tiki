package task

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/workflow"
)

// well-known built-in type constants.
const (
	TypeStory = workflow.TypeStory
	TypeBug   = workflow.TypeBug
	TypeSpike = workflow.TypeSpike
	TypeEpic  = workflow.TypeEpic
)

// requireTypeRegistry returns the loaded type registry.
// Panics if workflow registries have not been loaded — this is a programmer
// error, not a user-facing path.
func requireTypeRegistry() *workflow.TypeRegistry {
	return config.GetTypeRegistry()
}

// ParseType parses a raw string into a canonical type key with validation.
// Returns the canonical key and true if recognized,
// or ("", false) for unknown types.
// Panics if registries are not loaded.
func ParseType(t string) (string, bool) {
	return requireTypeRegistry().ParseType(t)
}

// TypeLabel returns a human-readable label for a task type.
// Panics if registries are not loaded.
func TypeLabel(taskType string) string {
	return requireTypeRegistry().TypeLabel(taskType)
}

// TypeEmoji returns the emoji for a task type.
// Panics if registries are not loaded.
func TypeEmoji(taskType string) string {
	return requireTypeRegistry().TypeEmoji(taskType)
}

// TypeDisplay returns a formatted display string with label and emoji.
// Panics if registries are not loaded.
func TypeDisplay(taskType string) string {
	return requireTypeRegistry().TypeDisplay(taskType)
}

// ParseDisplay reverses a TypeDisplay() string back to a canonical key.
// Returns (key, true) on match, or ("", false) for unrecognized display strings.
// Panics if registries are not loaded.
func ParseDisplay(display string) (string, bool) {
	return requireTypeRegistry().ParseDisplay(display)
}

// AllTypes returns the ordered list of all configured type keys.
// Panics if registries are not loaded.
func AllTypes() []string {
	return requireTypeRegistry().Keys()
}

// DefaultType returns the creation-default type (explicit default or first type).
// Panics if registries are not loaded.
func DefaultType() string {
	return requireTypeRegistry().DefaultType()
}

// IsValidType reports whether t is a recognized type key.
// Panics if registries are not loaded.
func IsValidType(t string) bool {
	return requireTypeRegistry().IsValid(t)
}
