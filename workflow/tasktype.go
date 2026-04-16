package workflow

import (
	"fmt"
	"strings"
)

// TaskType is a named type for workflow task type keys.
type TaskType string

// well-known built-in type constants.
const (
	TypeStory TaskType = "story"
	TypeBug   TaskType = "bug"
	TypeSpike TaskType = "spike"
	TypeEpic  TaskType = "epic"
)

// TypeDef defines a single task type with display metadata.
// Keys must be canonical (matching NormalizeTypeKey output).
type TypeDef struct {
	Key   TaskType `yaml:"key"`
	Label string   `yaml:"label,omitempty"`
	Emoji string   `yaml:"emoji,omitempty"`
}

// DefaultTypeDefs returns the built-in type definitions.
func DefaultTypeDefs() []TypeDef {
	return []TypeDef{
		{Key: TypeStory, Label: "Story", Emoji: "🌀"},
		{Key: TypeBug, Label: "Bug", Emoji: "💥"},
		{Key: TypeSpike, Label: "Spike", Emoji: "🔍"},
		{Key: TypeEpic, Label: "Epic", Emoji: "🗂️"},
	}
}

// TypeRegistry is an ordered collection of valid task types.
// Unknown input is never silently coerced — ParseType returns ("", false).
type TypeRegistry struct {
	types     []TypeDef
	byKey     map[TaskType]TypeDef
	byDisplay map[string]TaskType // display string → canonical key
}

// NormalizeTypeKey lowercases, trims, and strips all separators ("-", "_", " ").
// Used to compute the canonical form of a type key for validation.
func NormalizeTypeKey(s string) TaskType {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return TaskType(s)
}

// NewTypeRegistry constructs a TypeRegistry from the given definitions.
// Configured keys must already be canonical (matching NormalizeTypeKey output).
// Labels default to the key when omitted; explicitly empty labels are rejected.
// Emoji values are trimmed; duplicate display strings are rejected.
func NewTypeRegistry(defs []TypeDef) (*TypeRegistry, error) {
	if len(defs) == 0 {
		return nil, fmt.Errorf("type definitions list is empty")
	}

	reg := &TypeRegistry{
		types:     make([]TypeDef, 0, len(defs)),
		byKey:     make(map[TaskType]TypeDef, len(defs)),
		byDisplay: make(map[string]TaskType, len(defs)),
	}

	for i, def := range defs {
		if def.Key == "" {
			return nil, fmt.Errorf("type at index %d has empty key", i)
		}

		// require canonical key
		canonical := NormalizeTypeKey(string(def.Key))
		if def.Key != canonical {
			return nil, fmt.Errorf("type key %q is not canonical; use %q", def.Key, canonical)
		}
		def.Key = canonical

		if _, exists := reg.byKey[canonical]; exists {
			return nil, fmt.Errorf("duplicate type key %q", canonical)
		}

		// label: default to key when omitted, reject explicit empty/whitespace
		if def.Label == "" {
			def.Label = string(def.Key)
		} else if strings.TrimSpace(def.Label) == "" {
			return nil, fmt.Errorf("type %q has empty/whitespace label", def.Key)
		}

		// emoji: trim whitespace
		def.Emoji = strings.TrimSpace(def.Emoji)

		// compute display and reject duplicates
		display := typeDisplay(def.Label, def.Emoji)
		if existingKey, exists := reg.byDisplay[display]; exists {
			return nil, fmt.Errorf("duplicate type display %q: types %q and %q", display, existingKey, def.Key)
		}
		reg.byDisplay[display] = def.Key

		reg.byKey[canonical] = def
		reg.types = append(reg.types, def)
	}

	return reg, nil
}

// typeDisplay computes "Label Emoji" from parts (shared by constructor and method).
func typeDisplay(label, emoji string) string {
	if emoji == "" {
		return label
	}
	return label + " " + emoji
}

// Lookup returns the TypeDef for a given key (normalized) and whether it exists.
func (r *TypeRegistry) Lookup(key TaskType) (TypeDef, bool) {
	def, ok := r.byKey[NormalizeTypeKey(string(key))]
	return def, ok
}

// ParseType parses a raw string into a TaskType with validation.
// Returns the canonical key and true if recognized,
// or ("", false) for unknown types. No fallback, no coercion.
func (r *TypeRegistry) ParseType(s string) (TaskType, bool) {
	normalized := NormalizeTypeKey(s)
	if _, ok := r.byKey[normalized]; ok {
		return normalized, true
	}
	return "", false
}

// TypeLabel returns the human-readable label for a task type.
func (r *TypeRegistry) TypeLabel(t TaskType) string {
	if def, ok := r.Lookup(t); ok {
		return def.Label
	}
	return string(t)
}

// TypeEmoji returns the emoji for a task type.
func (r *TypeRegistry) TypeEmoji(t TaskType) string {
	if def, ok := r.Lookup(t); ok {
		return def.Emoji
	}
	return ""
}

// TypeDisplay returns "Label Emoji" for a task type.
func (r *TypeRegistry) TypeDisplay(t TaskType) string {
	label := r.TypeLabel(t)
	emoji := r.TypeEmoji(t)
	return typeDisplay(label, emoji)
}

// ParseDisplay reverses a TypeDisplay() string (e.g. "Bug 💥") back to
// its canonical key. Returns (key, true) on match, or ("", false).
func (r *TypeRegistry) ParseDisplay(display string) (TaskType, bool) {
	if key, ok := r.byDisplay[display]; ok {
		return key, true
	}
	return "", false
}

// DefaultType returns the first configured type key — used as the creation
// default when no type is specified. Requires at least one registered type.
func (r *TypeRegistry) DefaultType() TaskType {
	return r.types[0].Key
}

// Keys returns all type keys in definition order.
func (r *TypeRegistry) Keys() []TaskType {
	keys := make([]TaskType, len(r.types))
	for i, td := range r.types {
		keys[i] = td.Key
	}
	return keys
}

// All returns the ordered list of type definitions.
// returns a copy to prevent callers from mutating internal state.
func (r *TypeRegistry) All() []TypeDef {
	result := make([]TypeDef, len(r.types))
	copy(result, r.types)
	return result
}

// IsValid reports whether key is a recognized type.
func (r *TypeRegistry) IsValid(key TaskType) bool {
	_, ok := r.byKey[NormalizeTypeKey(string(key))]
	return ok
}
