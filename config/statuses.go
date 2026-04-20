package config

import (
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/boolean-maybe/tiki/workflow"
	"gopkg.in/yaml.v3"
)

// StatusDef is a type alias for workflow.StatusDef.
// Kept for backward compatibility during migration.
type StatusDef = workflow.StatusDef

// StatusRegistry is a type alias for workflow.StatusRegistry.
type StatusRegistry = workflow.StatusRegistry

// NormalizeStatusKey delegates to workflow.NormalizeStatusKey.
func NormalizeStatusKey(key string) string {
	return string(workflow.NormalizeStatusKey(key))
}

var (
	globalStatusRegistry *workflow.StatusRegistry
	globalTypeRegistry   *workflow.TypeRegistry
	registryMu           sync.RWMutex
)

// LoadStatusRegistry reads statuses: and types: from the single highest-priority
// workflow.yaml. Returns an error if the file is missing, has no statuses, or
// has no types.
func LoadStatusRegistry() error {
	files := FindRegistryWorkflowFiles()
	if len(files) == 0 {
		return fmt.Errorf("no workflow.yaml found; statuses must be defined in workflow.yaml")
	}

	path := files[0]

	statusReg, err := loadStatusesFromFile(path)
	if err != nil {
		return fmt.Errorf("loading statuses from %s: %w", path, err)
	}
	if statusReg == nil {
		return fmt.Errorf("no statuses defined in %s; add a statuses: section", path)
	}

	typeReg, present, err := loadTypesFromFile(path)
	if err != nil {
		return fmt.Errorf("loading types from %s: %w", path, err)
	}
	if !present {
		return fmt.Errorf("no types defined in %s; add a types: section", path)
	}

	registryMu.Lock()
	globalStatusRegistry = statusReg
	globalTypeRegistry = typeReg
	registryMu.Unlock()

	slog.Debug("loaded status registry", "file", path, "count", len(statusReg.All()))
	slog.Debug("loaded type registry", "file", path, "count", len(typeReg.All()))
	return nil
}

// GetStatusRegistry returns the global StatusRegistry.
// Panics if LoadStatusRegistry() was never called — this is a programming error,
// not a user-facing path.
func GetStatusRegistry() *workflow.StatusRegistry {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if globalStatusRegistry == nil {
		panic("config: GetStatusRegistry called before LoadStatusRegistry")
	}
	return globalStatusRegistry
}

// GetTypeRegistry returns the global TypeRegistry.
// Panics if LoadStatusRegistry() was never called.
func GetTypeRegistry() *workflow.TypeRegistry {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if globalTypeRegistry == nil {
		panic("config: GetTypeRegistry called before LoadStatusRegistry")
	}
	return globalTypeRegistry
}

// MaybeGetTypeRegistry returns the global TypeRegistry if it has been
// initialized, or (nil, false) when LoadStatusRegistry() has not run yet.
func MaybeGetTypeRegistry() (*workflow.TypeRegistry, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return globalTypeRegistry, globalTypeRegistry != nil
}

// ResetStatusRegistry replaces the global registry with one built from the given defs.
// Also resets types to built-in defaults and clears custom fields so test helpers
// don't leak registry state. Intended for tests only.
func ResetStatusRegistry(defs []workflow.StatusDef) {
	reg, err := workflow.NewStatusRegistry(defs)
	if err != nil {
		panic(fmt.Sprintf("ResetStatusRegistry: %v", err))
	}
	typeReg, err := workflow.NewTypeRegistry(workflow.DefaultTypeDefs())
	if err != nil {
		panic(fmt.Sprintf("ResetStatusRegistry: type registry: %v", err))
	}
	registryMu.Lock()
	globalStatusRegistry = reg
	globalTypeRegistry = typeReg
	registryMu.Unlock()
	workflow.ClearCustomFields()
	registriesLoaded.Store(true)
}

// ResetTypeRegistry replaces the global type registry with one built from the
// given defs, without touching the status registry. Intended for tests that
// need custom type configurations while keeping existing status setup.
func ResetTypeRegistry(defs []workflow.TypeDef) {
	reg, err := workflow.NewTypeRegistry(defs)
	if err != nil {
		panic(fmt.Sprintf("ResetTypeRegistry: %v", err))
	}
	registryMu.Lock()
	globalTypeRegistry = reg
	registryMu.Unlock()
}

// ClearStatusRegistry removes the global registries and clears custom fields.
// Intended for test teardown.
func ClearStatusRegistry() {
	registryMu.Lock()
	globalStatusRegistry = nil
	globalTypeRegistry = nil
	registryMu.Unlock()
	workflow.ClearCustomFields()
	registriesLoaded.Store(false)
}

// --- internal: statuses ---

// workflowStatusData is the YAML shape we unmarshal to extract just the statuses key.
type workflowStatusData struct {
	Statuses []workflow.StatusDef `yaml:"statuses"`
}

func loadStatusesFromFile(path string) (*workflow.StatusRegistry, error) {
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

	return workflow.NewStatusRegistry(ws.Statuses)
}

// --- internal: types ---

// validTypeDefKeys is the set of allowed keys inside a types: entry.
var validTypeDefKeys = map[string]bool{
	"key": true, "label": true, "emoji": true,
}

// loadTypesFromFile loads types from a single workflow.yaml.
// Returns (registry, present, error):
//   - (nil, false, nil)  when the types: key is absent
//   - (reg, true, nil)   when types: is present and valid
//   - (nil, true, err)   when types: is present but invalid (empty list, bad entries)
func loadTypesFromFile(path string) (*workflow.TypeRegistry, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("reading %s: %w", path, err)
	}

	// first pass: check whether the types key exists at all
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, false, fmt.Errorf("parsing %s: %w", path, err)
	}

	rawTypes, exists := raw["types"]
	if !exists {
		return nil, false, nil // types: key absent
	}

	// present: validate the raw structure
	typesSlice, ok := rawTypes.([]interface{})
	if !ok {
		return nil, true, fmt.Errorf("types: must be a list, got %T", rawTypes)
	}
	if len(typesSlice) == 0 {
		return nil, true, fmt.Errorf("types section must define at least one type")
	}

	// validate each entry for unknown keys and convert to TypeDef
	defs := make([]workflow.TypeDef, 0, len(typesSlice))
	for i, entry := range typesSlice {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			return nil, true, fmt.Errorf("type at index %d: expected mapping, got %T", i, entry)
		}
		for k := range entryMap {
			if !validTypeDefKeys[k] {
				return nil, true, fmt.Errorf("type at index %d: unknown key %q (valid keys: key, label, emoji)", i, k)
			}
		}

		var def workflow.TypeDef
		keyRaw, _ := entryMap["key"].(string)
		def.Key = workflow.TaskType(keyRaw)
		def.Label, _ = entryMap["label"].(string)
		def.Emoji, _ = entryMap["emoji"].(string)
		defs = append(defs, def)
	}

	reg, err := workflow.NewTypeRegistry(defs)
	if err != nil {
		return nil, true, err
	}
	return reg, true, nil
}
