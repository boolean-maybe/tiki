package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/boolean-maybe/tiki/workflow"
)

// StatusDef is a type alias for workflow.StatusDef.
// Kept for backward compatibility during migration.
type StatusDef = workflow.StatusDef

// StatusRegistry is a type alias for workflow.StatusRegistry.
type StatusRegistry = workflow.StatusRegistry

// NormalizeStatusKey delegates to workflow.NormalizeStatusKey.
func NormalizeStatusKey(key string) string {
	return workflow.NormalizeStatusKey(key)
}

var (
	globalStatusRegistry *workflow.StatusRegistry
	globalTypeRegistry   *workflow.TypeRegistry
	registryMu           sync.RWMutex
)

// LoadStatusRegistry reads status/type field definitions from the single
// highest-priority workflow.yaml. Returns an error if the file is missing or
// either registry field is not declared.
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
		return fmt.Errorf("no status field defined in %s; add fields: entry name: status, type: enum", path)
	}

	typeReg, present, err := loadTypesFromFile(path)
	if err != nil {
		return fmt.Errorf("loading types from %s: %w", path, err)
	}
	if !present {
		return fmt.Errorf("no type field defined in %s; add fields: entry name: type, type: enum", path)
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

// LoadRegistriesFromFile validates and loads statuses, types, and custom fields
// from a single explicit workflow file path. Returns local registries without
// touching global state. Used by init to validate a candidate workflow file.
func LoadRegistriesFromFile(path string) (*workflow.StatusRegistry, *workflow.TypeRegistry, []workflow.FieldDef, error) {
	if err := CheckFileVersionCompatibility(path); err != nil {
		return nil, nil, nil, err
	}

	statusReg, err := loadStatusesFromFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading statuses: %w", err)
	}
	if statusReg == nil {
		return nil, nil, nil, fmt.Errorf("no status field defined; add fields: entry name: status, type: enum")
	}

	typeReg, present, err := loadTypesFromFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading types: %w", err)
	}
	if !present {
		return nil, nil, nil, fmt.Errorf("no type field defined; add fields: entry name: type, type: enum")
	}

	rawDefs, err := readCustomFieldsFromFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reading custom fields: %w", err)
	}

	var fieldDefs []workflow.FieldDef
	for _, raw := range rawDefs {
		if isRegistryField(raw.Name) {
			continue
		}
		def, convErr := convertCustomFieldDef(raw)
		if convErr != nil {
			return nil, nil, nil, fmt.Errorf("field %q: %w", raw.Name, convErr)
		}
		fieldDefs = append(fieldDefs, def)
	}

	if err := workflow.ValidateCustomFields(fieldDefs); err != nil {
		return nil, nil, nil, fmt.Errorf("custom fields: %w", err)
	}

	return statusReg, typeReg, fieldDefs, nil
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

func loadStatusesFromFile(path string) (*workflow.StatusRegistry, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	fields, err := readCustomFieldsFromFile(path)
	if err != nil {
		return nil, err
	}
	field, ok, err := findWorkflowField(fields, "status")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	defs, err := convertStatusField(field)
	if err != nil {
		return nil, err
	}
	return workflow.NewStatusRegistry(defs)
}

// loadTypesFromFile loads the type field from a single workflow.yaml.
// Returns (registry, present, error):
//   - (nil, false, nil) when the type field is absent
//   - (reg, true, nil) when type is present and valid
//   - (nil, true, err) when type is present but invalid
func loadTypesFromFile(path string) (*workflow.TypeRegistry, bool, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, false, err
	}
	fields, err := readCustomFieldsFromFile(path)
	if err != nil {
		return nil, false, err
	}
	field, ok, err := findWorkflowField(fields, "type")
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	defs, err := convertTypeField(field)
	if err != nil {
		return nil, true, err
	}
	reg, err := workflow.NewTypeRegistry(defs)
	if err != nil {
		return nil, true, err
	}
	return reg, true, nil
}

func findWorkflowField(fields []customFieldYAML, name string) (customFieldYAML, bool, error) {
	var found customFieldYAML
	matched := false
	for _, field := range fields {
		if field.Name == name {
			if matched {
				return customFieldYAML{}, false, fmt.Errorf("duplicate field %q", name)
			}
			found = field
			matched = true
		}
	}
	return found, matched, nil
}

func convertStatusField(field customFieldYAML) ([]workflow.StatusDef, error) {
	if err := validateRegistryField(field, "status"); err != nil {
		return nil, err
	}
	defs := make([]workflow.StatusDef, 0, len(field.Values))
	for i, value := range field.Values {
		if err := validateStructuredEnumValue("status", i, value); err != nil {
			return nil, err
		}
		defs = append(defs, workflow.StatusDef{
			Key:     value.Value,
			Label:   value.Label,
			Emoji:   value.Emoji,
			Active:  value.Active,
			Default: value.Default,
			Done:    value.Done,
		})
	}
	return defs, nil
}

func convertTypeField(field customFieldYAML) ([]workflow.TypeDef, error) {
	if err := validateRegistryField(field, "type"); err != nil {
		return nil, err
	}
	defs := make([]workflow.TypeDef, 0, len(field.Values))
	for i, value := range field.Values {
		if err := validateStructuredEnumValue("type", i, value); err != nil {
			return nil, err
		}
		if value.HasActive || value.HasDone {
			return nil, fmt.Errorf("type enum value %q: active and done flags are only valid for status", value.Value)
		}
		defs = append(defs, workflow.TypeDef{
			Key:     value.Value,
			Label:   value.Label,
			Emoji:   value.Emoji,
			Default: value.Default,
		})
	}
	return defs, nil
}

func validateRegistryField(field customFieldYAML, name string) error {
	if !strings.EqualFold(field.Type, "enum") {
		return fmt.Errorf("field %q must declare type: enum", name)
	}
	if len(field.Values) == 0 {
		return fmt.Errorf("field %q requires a non-empty values list", name)
	}
	if field.Default != nil {
		return fmt.Errorf("field %q does not support field-level default; mark one enum value default: true", name)
	}
	return nil
}

func validateStructuredEnumValue(fieldName string, index int, value enumValueYAML) error {
	if !value.Structured {
		return fmt.Errorf("field %q enum value at index %d must be a mapping with value:", fieldName, index)
	}
	if value.Value == "" {
		return fmt.Errorf("field %q enum value at index %d has empty value", fieldName, index)
	}
	return nil
}
