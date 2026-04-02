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

// TypeDef defines a single task type with metadata and aliases.
type TypeDef struct {
	Key     TaskType
	Label   string
	Emoji   string
	Aliases []string // e.g. "feature" and "task" → story
}

// DefaultTypeDefs returns the built-in type definitions.
func DefaultTypeDefs() []TypeDef {
	return []TypeDef{
		{Key: TypeStory, Label: "Story", Emoji: "🌀", Aliases: []string{"feature", "task"}},
		{Key: TypeBug, Label: "Bug", Emoji: "💥"},
		{Key: TypeSpike, Label: "Spike", Emoji: "🔍"},
		{Key: TypeEpic, Label: "Epic", Emoji: "🗂️"},
	}
}

// TypeRegistry is an ordered collection of valid task types.
// It is constructed from a list of TypeDef and provides lookup and normalization.
type TypeRegistry struct {
	types    []TypeDef
	byKey    map[TaskType]TypeDef
	byAlias  map[string]TaskType // normalized alias → canonical key
	fallback TaskType            // returned for unknown types
}

// NormalizeTypeKey lowercases, trims, and strips all separators ("-", "_", " ").
// Built-in type keys are single words, so stripping is lossless.
func NormalizeTypeKey(s string) TaskType {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return TaskType(s)
}

// NewTypeRegistry constructs a TypeRegistry from the given definitions.
// The first definition's key is used as the fallback for unknown types.
func NewTypeRegistry(defs []TypeDef) (*TypeRegistry, error) {
	if len(defs) == 0 {
		return nil, fmt.Errorf("type definitions list is empty")
	}

	reg := &TypeRegistry{
		types:    make([]TypeDef, 0, len(defs)),
		byKey:    make(map[TaskType]TypeDef, len(defs)),
		byAlias:  make(map[string]TaskType),
		fallback: NormalizeTypeKey(string(defs[0].Key)),
	}

	// first pass: register all primary keys
	for i, def := range defs {
		if def.Key == "" {
			return nil, fmt.Errorf("type at index %d has empty key", i)
		}

		normalized := NormalizeTypeKey(string(def.Key))
		def.Key = normalized
		defs[i] = def

		if _, exists := reg.byKey[normalized]; exists {
			return nil, fmt.Errorf("duplicate type key %q", normalized)
		}

		reg.byKey[normalized] = def
		reg.types = append(reg.types, def)
	}

	// second pass: register aliases against the complete key set
	for _, def := range defs {
		for _, alias := range def.Aliases {
			normAlias := string(NormalizeTypeKey(alias))
			if existing, ok := reg.byAlias[normAlias]; ok {
				return nil, fmt.Errorf("duplicate alias %q (already maps to %q)", alias, existing)
			}
			if _, ok := reg.byKey[TaskType(normAlias)]; ok {
				return nil, fmt.Errorf("alias %q collides with primary key", alias)
			}
			reg.byAlias[normAlias] = def.Key
		}
	}

	return reg, nil
}

// Lookup returns the TypeDef for a given key (normalized) and whether it exists.
func (r *TypeRegistry) Lookup(key TaskType) (TypeDef, bool) {
	def, ok := r.byKey[NormalizeTypeKey(string(key))]
	return def, ok
}

// ParseType parses a raw string into a TaskType with validation.
// Returns the canonical key and true if recognized (including aliases),
// or (fallback, false) for unknown types.
func (r *TypeRegistry) ParseType(s string) (TaskType, bool) {
	normalized := NormalizeTypeKey(s)

	// check primary keys
	if _, ok := r.byKey[normalized]; ok {
		return normalized, true
	}

	// check aliases
	if canonical, ok := r.byAlias[string(normalized)]; ok {
		return canonical, true
	}

	return r.fallback, false
}

// NormalizeType normalizes a raw string into a TaskType.
// Unknown types default to the fallback (first registered type).
func (r *TypeRegistry) NormalizeType(s string) TaskType {
	t, _ := r.ParseType(s)
	return t
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
	if emoji == "" {
		return label
	}
	return label + " " + emoji
}

// ParseDisplay reverses a TypeDisplay() string (e.g. "Bug 💥") back to
// its canonical key. Returns (key, true) on match, or (fallback, false).
func (r *TypeRegistry) ParseDisplay(display string) (TaskType, bool) {
	for _, def := range r.types {
		if r.TypeDisplay(def.Key) == display {
			return def.Key, true
		}
	}
	return r.fallback, false
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

// IsValid reports whether key is a recognized type (primary key only, not alias).
func (r *TypeRegistry) IsValid(key TaskType) bool {
	_, ok := r.byKey[NormalizeTypeKey(string(key))]
	return ok
}
