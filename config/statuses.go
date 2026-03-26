package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// StatusDef defines a single workflow status loaded from workflow.yaml.
type StatusDef struct {
	Key     string `yaml:"key"`
	Label   string `yaml:"label"`
	Emoji   string `yaml:"emoji"`
	Active  bool   `yaml:"active"`
	Default bool   `yaml:"default"`
	Done    bool   `yaml:"done"`
}

// StatusRegistry is the central, ordered collection of valid statuses.
// It is loaded once from workflow.yaml during bootstrap and accessed globally.
type StatusRegistry struct {
	statuses   []StatusDef
	byKey      map[string]StatusDef
	defaultKey string
	doneKey    string
}

var (
	globalRegistry *StatusRegistry
	registryMu     sync.RWMutex
)

// LoadStatusRegistry reads the statuses: section from workflow.yaml files.
// The last file from FindWorkflowFiles() that contains a non-empty statuses list wins
// (most specific location takes precedence, matching plugin merge behavior).
// Returns an error if no statuses are defined anywhere (no Go fallback).
func LoadStatusRegistry() error {
	files := FindWorkflowFiles()
	if len(files) == 0 {
		return fmt.Errorf("no workflow.yaml found; statuses must be defined in workflow.yaml")
	}

	reg, path, err := loadStatusRegistryFromFiles(files)
	if err != nil {
		return err
	}
	if reg == nil {
		return fmt.Errorf("no statuses defined in workflow.yaml; add a statuses: section")
	}

	registryMu.Lock()
	globalRegistry = reg
	registryMu.Unlock()
	slog.Debug("loaded status registry", "file", path, "count", len(reg.statuses))
	return nil
}

// loadStatusRegistryFromFiles iterates workflow files and returns the registry
// from the last file that contains a non-empty statuses section.
// Returns a parse error immediately if any file is malformed.
func loadStatusRegistryFromFiles(files []string) (*StatusRegistry, string, error) {
	var lastReg *StatusRegistry
	var lastFile string

	for _, path := range files {
		reg, err := loadStatusesFromFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("loading statuses from %s: %w", path, err)
		}
		if reg != nil {
			lastReg = reg
			lastFile = path
		}
	}

	return lastReg, lastFile, nil
}

// GetStatusRegistry returns the global StatusRegistry.
// Panics if LoadStatusRegistry() was never called — this is a programming error,
// not a user-facing path.
func GetStatusRegistry() *StatusRegistry {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if globalRegistry == nil {
		panic("config: GetStatusRegistry called before LoadStatusRegistry")
	}
	return globalRegistry
}

// ResetStatusRegistry replaces the global registry with one built from the given defs.
// Intended for tests only.
func ResetStatusRegistry(defs []StatusDef) {
	reg, err := buildRegistry(defs)
	if err != nil {
		panic(fmt.Sprintf("ResetStatusRegistry: %v", err))
	}
	registryMu.Lock()
	globalRegistry = reg
	registryMu.Unlock()
}

// ClearStatusRegistry removes the global registry. Intended for test teardown.
func ClearStatusRegistry() {
	registryMu.Lock()
	globalRegistry = nil
	registryMu.Unlock()
}

// --- Registry methods ---

// All returns the ordered list of status definitions.
func (r *StatusRegistry) All() []StatusDef {
	return r.statuses
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
func (r *StatusRegistry) DefaultKey() string {
	return r.defaultKey
}

// DoneKey returns the key of the status with done: true.
func (r *StatusRegistry) DoneKey() string {
	return r.doneKey
}

// Keys returns all status keys in definition order.
func (r *StatusRegistry) Keys() []string {
	keys := make([]string, len(r.statuses))
	for i, s := range r.statuses {
		keys[i] = s.Key
	}
	return keys
}

// NormalizeStatusKey lowercases, trims, and normalizes separators in a status key.
func NormalizeStatusKey(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

// --- internal ---

// workflowStatusData is the YAML shape we unmarshal to extract just the statuses key.
type workflowStatusData struct {
	Statuses []StatusDef `yaml:"statuses"`
}

func loadStatusesFromFile(path string) (*StatusRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var ws workflowStatusData
	if err := yaml.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if len(ws.Statuses) == 0 {
		return nil, nil // no statuses in this file, try next
	}

	return buildRegistry(ws.Statuses)
}

func buildRegistry(defs []StatusDef) (*StatusRegistry, error) {
	if len(defs) == 0 {
		return nil, fmt.Errorf("statuses list is empty")
	}

	reg := &StatusRegistry{
		statuses: make([]StatusDef, 0, len(defs)),
		byKey:    make(map[string]StatusDef, len(defs)),
	}

	for i, def := range defs {
		if def.Key == "" {
			return nil, fmt.Errorf("status at index %d has empty key", i)
		}

		normalized := NormalizeStatusKey(def.Key)
		def.Key = normalized

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

	// If no explicit default, use the first status
	if reg.defaultKey == "" {
		reg.defaultKey = reg.statuses[0].Key
		slog.Warn("no status marked default; using first status", "key", reg.defaultKey)
	}

	return reg, nil
}
