package workflow

import (
	"fmt"
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

// StatusDisplay returns "Label Emoji" for a status.
func StatusDisplay(def StatusDef) string {
	label := def.Label
	if label == "" {
		label = def.Key
	}
	emoji := strings.TrimSpace(def.Emoji)
	if emoji == "" {
		return label
	}
	return label + " " + emoji
}

// NewStatusRegistry constructs a StatusRegistry from the given definitions.
// Configured keys must be canonical (matching NormalizeStatusKey output).
// Labels default to key when omitted; explicitly empty labels are rejected.
// Emoji values are trimmed. Requires exactly one default and one done status.
// Duplicate display strings are rejected.
func NewStatusRegistry(defs []StatusDef) (*StatusRegistry, error) {
	if len(defs) == 0 {
		return nil, fmt.Errorf("statuses list is empty")
	}

	reg := &StatusRegistry{
		statuses: make([]StatusDef, 0, len(defs)),
		byKey:    make(map[StatusKey]StatusDef, len(defs)),
	}
	displaySeen := make(map[string]StatusKey, len(defs))

	for i, def := range defs {
		if def.Key == "" {
			return nil, fmt.Errorf("status at index %d has empty key", i)
		}

		// require canonical key
		canonical := NormalizeStatusKey(def.Key)
		if def.Key != string(canonical) {
			return nil, fmt.Errorf("status key %q is not canonical; use %q", def.Key, canonical)
		}
		def.Key = string(canonical)

		if _, exists := reg.byKey[canonical]; exists {
			return nil, fmt.Errorf("duplicate status key %q", canonical)
		}

		// label: default to key when omitted, reject explicit empty/whitespace
		if def.Label == "" {
			def.Label = def.Key
		} else if strings.TrimSpace(def.Label) == "" {
			return nil, fmt.Errorf("status %q has empty/whitespace label", def.Key)
		}

		// emoji: trim whitespace
		def.Emoji = strings.TrimSpace(def.Emoji)

		// reject duplicate display strings
		display := StatusDisplay(def)
		if existingKey, exists := displaySeen[display]; exists {
			return nil, fmt.Errorf("duplicate status display %q: statuses %q and %q", display, existingKey, canonical)
		}
		displaySeen[display] = canonical

		if def.Default {
			if reg.defaultKey != "" {
				return nil, fmt.Errorf("multiple statuses marked default: %q and %q", reg.defaultKey, canonical)
			}
			reg.defaultKey = canonical
		}
		if def.Done {
			if reg.doneKey != "" {
				return nil, fmt.Errorf("multiple statuses marked done: %q and %q", reg.doneKey, canonical)
			}
			reg.doneKey = canonical
		}

		reg.byKey[canonical] = def
		reg.statuses = append(reg.statuses, def)
	}

	if reg.defaultKey == "" {
		return nil, fmt.Errorf("no status marked default: true; exactly one is required")
	}

	if reg.doneKey == "" {
		return nil, fmt.Errorf("no status marked done: true; exactly one is required")
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
