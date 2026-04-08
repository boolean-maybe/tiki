package workflow

import (
	"fmt"
	"log/slog"
	"strings"
)

// StatusKey is a named type for workflow status keys.
// All status keys are normalized: lowercase, underscores as separators.
type StatusKey string

// well-known status constants (defaults from workflow.yaml template)
const (
	StatusBacklog    StatusKey = "backlog"
	StatusReady      StatusKey = "ready"
	StatusInProgress StatusKey = "inProgress"
	StatusReview     StatusKey = "review"
	StatusDone       StatusKey = "done"
)

// StatusDef defines a single workflow status.
type StatusDef struct {
	Key     string `yaml:"key"`
	Label   string `yaml:"label"`
	Emoji   string `yaml:"emoji"`
	Active  bool   `yaml:"active"`
	Default bool   `yaml:"default"`
	Done    bool   `yaml:"done"`
}

// StatusRegistry is an ordered collection of valid statuses.
// It is constructed from a list of StatusDef and provides lookup and query methods.
// StatusRegistry holds no global state — the populated singleton lives in config/.
type StatusRegistry struct {
	statuses   []StatusDef
	byKey      map[StatusKey]StatusDef
	defaultKey StatusKey
	doneKey    StatusKey
}

// NormalizeStatusKey trims and converts a status key to camelCase.
// Splits on "_", "-", " ", and camelCase boundaries, then reassembles.
// Examples: "in_progress" → "inProgress", "In Progress" → "inProgress",
// "inProgress" → "inProgress", "IN_PROGRESS" → "inProgress".
func NormalizeStatusKey(key string) StatusKey {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}

	words := splitStatusWords(trimmed)
	if len(words) == 0 {
		return ""
	}

	var b strings.Builder
	for i, w := range words {
		if i == 0 {
			b.WriteString(strings.ToLower(w))
		} else {
			b.WriteString(strings.ToUpper(w[:1]) + strings.ToLower(w[1:]))
		}
	}
	return StatusKey(b.String())
}

// splitStatusWords splits a key string into words on separators ("_", "-", " ")
// and camelCase boundaries (lowercase→uppercase transition).
func splitStatusWords(s string) []string {
	// first split on explicit separators
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})

	// then split each part on camelCase boundaries
	var words []string
	for _, part := range parts {
		words = append(words, splitCamelCase(part)...)
	}
	return words
}

// splitCamelCase splits on lowercase→uppercase transitions.
// "inProgress" → ["in", "Progress"], "ABC" → ["ABC"], "ready" → ["ready"].
func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}

	var words []string
	start := 0
	for i := 1; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' && s[i-1] >= 'a' && s[i-1] <= 'z' {
			words = append(words, s[start:i])
			start = i
		}
	}
	words = append(words, s[start:])
	return words
}

// NewStatusRegistry constructs a StatusRegistry from the given definitions.
// Returns an error if keys are empty, duplicated, or the list is empty.
func NewStatusRegistry(defs []StatusDef) (*StatusRegistry, error) {
	if len(defs) == 0 {
		return nil, fmt.Errorf("statuses list is empty")
	}

	reg := &StatusRegistry{
		statuses: make([]StatusDef, 0, len(defs)),
		byKey:    make(map[StatusKey]StatusDef, len(defs)),
	}

	for i, def := range defs {
		if def.Key == "" {
			return nil, fmt.Errorf("status at index %d has empty key", i)
		}

		normalized := NormalizeStatusKey(def.Key)
		def.Key = string(normalized)

		if _, exists := reg.byKey[normalized]; exists {
			return nil, fmt.Errorf("duplicate status key %q", normalized)
		}

		if def.Default {
			if reg.defaultKey != "" {
				slog.Warn("multiple statuses marked default; using first", "first", reg.defaultKey, "duplicate", normalized)
			} else {
				reg.defaultKey = normalized
			}
		}
		if def.Done {
			if reg.doneKey != "" {
				slog.Warn("multiple statuses marked done; using first", "first", reg.doneKey, "duplicate", normalized)
			} else {
				reg.doneKey = normalized
			}
		}

		reg.byKey[normalized] = def
		reg.statuses = append(reg.statuses, def)
	}

	// if no explicit default, use the first status
	if reg.defaultKey == "" {
		reg.defaultKey = StatusKey(reg.statuses[0].Key)
		slog.Warn("no status marked default; using first status", "key", reg.defaultKey)
	}

	if reg.doneKey == "" {
		slog.Warn("no status marked done; task completion features may not work correctly")
	}

	return reg, nil
}

// All returns the ordered list of status definitions.
// returns a copy to prevent callers from mutating internal state.
func (r *StatusRegistry) All() []StatusDef {
	result := make([]StatusDef, len(r.statuses))
	copy(result, r.statuses)
	return result
}

// Lookup returns the StatusDef for a given key (normalized) and whether it exists.
func (r *StatusRegistry) Lookup(key string) (StatusDef, bool) {
	def, ok := r.byKey[NormalizeStatusKey(key)]
	return def, ok
}

// IsValid reports whether key is a recognized status.
func (r *StatusRegistry) IsValid(key string) bool {
	_, ok := r.byKey[NormalizeStatusKey(key)]
	return ok
}

// IsActive reports whether the status has the active flag set.
func (r *StatusRegistry) IsActive(key string) bool {
	def, ok := r.byKey[NormalizeStatusKey(key)]
	return ok && def.Active
}

// IsDone reports whether the status has the done flag set.
func (r *StatusRegistry) IsDone(key string) bool {
	def, ok := r.byKey[NormalizeStatusKey(key)]
	return ok && def.Done
}

// DefaultKey returns the key of the status with default: true.
func (r *StatusRegistry) DefaultKey() StatusKey {
	return r.defaultKey
}

// DoneKey returns the key of the status with done: true.
func (r *StatusRegistry) DoneKey() StatusKey {
	return r.doneKey
}

// Keys returns all status keys in definition order.
func (r *StatusRegistry) Keys() []StatusKey {
	keys := make([]StatusKey, len(r.statuses))
	for i, s := range r.statuses {
		keys[i] = StatusKey(s.Key)
	}
	return keys
}
