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

// LoadStatusRegistry reads the statuses: section from workflow.yaml files.
// Uses FindRegistryWorkflowFiles (no views filtering) so files with empty views:
// still contribute status definitions.
// The last file that contains a non-empty statuses list wins
// (most specific location takes precedence, matching plugin merge behavior).
// Returns an error if no statuses are defined anywhere (no Go fallback).
func LoadStatusRegistry() error {
	files := FindRegistryWorkflowFiles()
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
	globalStatusRegistry = reg
	registryMu.Unlock()
	slog.Debug("loaded status registry", "file", path, "count", len(reg.All()))

	// also initialize type registry with defaults
	typeReg, err := workflow.NewTypeRegistry(workflow.DefaultTypeDefs())
	if err != nil {
		return fmt.Errorf("initializing type registry: %w", err)
	}
	registryMu.Lock()
	globalTypeRegistry = typeReg
	registryMu.Unlock()

	return nil
}

// loadStatusRegistryFromFiles iterates workflow files and returns the registry
// from the last file that contains a non-empty statuses section.
// Returns a parse error immediately if any file is malformed.
func loadStatusRegistryFromFiles(files []string) (*workflow.StatusRegistry, string, error) {
	var lastReg *workflow.StatusRegistry
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
// Also clears custom fields so test helpers don't leak registry state.
// Intended for tests only.
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

// --- internal ---

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
