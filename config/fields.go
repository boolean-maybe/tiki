package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/boolean-maybe/tiki/workflow"
	"gopkg.in/yaml.v3"
)

// customFieldYAML represents a single field entry in the workflow.yaml fields: section.
type customFieldYAML struct {
	Name   string   `yaml:"name"`
	Type   string   `yaml:"type"`
	Values []string `yaml:"values,omitempty"` // enum only
}

// customFieldFileData is the minimal YAML structure for reading fields from workflow.yaml.
type customFieldFileData struct {
	Fields []customFieldYAML `yaml:"fields"`
}

// registriesLoaded tracks whether LoadWorkflowRegistries has been called.
var registriesLoaded atomic.Bool

// RequireWorkflowRegistriesLoaded returns an error if LoadWorkflowRegistries
// (or LoadStatusRegistry + LoadCustomFields) has not been called yet.
// Intended for use by store/template code that needs registries to be ready
// but should not auto-load them from disk.
func RequireWorkflowRegistriesLoaded() error {
	if !registriesLoaded.Load() {
		return fmt.Errorf("workflow registries not loaded; call config.LoadWorkflowRegistries() first")
	}
	return nil
}

// MarkRegistriesLoadedForTest sets the registriesLoaded flag without loading
// from disk. Use in tests that call workflow.RegisterCustomFields directly.
func MarkRegistriesLoadedForTest() {
	registriesLoaded.Store(true)
}

// ResetRegistriesLoadedForTest clears the registriesLoaded flag.
// Use in tests that need to verify the unloaded-registry error path.
func ResetRegistriesLoadedForTest() {
	registriesLoaded.Store(false)
}

// LoadWorkflowRegistries is the shared startup helper that loads all
// workflow-registry-based sections (statuses, types, custom fields) from
// workflow.yaml files. Callers must build a fresh ruki.Schema after this returns.
func LoadWorkflowRegistries() error {
	if err := LoadStatusRegistry(); err != nil {
		return err
	}
	if err := LoadCustomFields(); err != nil {
		return err
	}
	registriesLoaded.Store(true)
	return nil
}

// LoadCustomFields reads the fields: section from all workflow.yaml files,
// validates and merges definitions, and registers them with workflow.RegisterCustomFields.
// Uses FindRegistryWorkflowFiles (no views filtering) so files with empty views:
// still contribute custom field definitions.
// Merge semantics: identical redefinitions allowed, conflicting redefinitions error.
func LoadCustomFields() error {
	files := FindRegistryWorkflowFiles()
	if len(files) == 0 {
		// no workflow files at all — no custom fields to register, clear any stale state
		workflow.ClearCustomFields()
		return nil
	}

	// collect all field definitions with their source file
	type fieldSource struct {
		def  customFieldYAML
		file string
	}
	var allFields []fieldSource

	for _, path := range files {
		defs, err := readCustomFieldsFromFile(path)
		if err != nil {
			return fmt.Errorf("reading custom fields from %s: %w", path, err)
		}
		for _, d := range defs {
			allFields = append(allFields, fieldSource{def: d, file: path})
		}
	}

	if len(allFields) == 0 {
		workflow.ClearCustomFields()
		return nil
	}

	// merge: identical definitions allowed, conflicting definitions error
	type mergedField struct {
		def        workflow.FieldDef
		sourceFile string
	}
	merged := make(map[string]*mergedField)

	for _, fs := range allFields {
		def, err := convertCustomFieldDef(fs.def)
		if err != nil {
			return fmt.Errorf("field %q in %s: %w", fs.def.Name, fs.file, err)
		}

		if existing, ok := merged[def.Name]; ok {
			if !fieldDefsEqual(existing.def, def) {
				return fmt.Errorf("conflicting definition for custom field %q: defined differently in %s and %s",
					def.Name, existing.sourceFile, fs.file)
			}
			// identical redefinition — skip
			continue
		}

		merged[def.Name] = &mergedField{def: def, sourceFile: fs.file}
	}

	// build ordered slice for registration
	defs := make([]workflow.FieldDef, 0, len(merged))
	for _, m := range merged {
		defs = append(defs, m.def)
	}
	// sort by name for deterministic ordering
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})

	if err := workflow.RegisterCustomFields(defs); err != nil {
		return fmt.Errorf("registering custom fields: %w", err)
	}

	slog.Debug("loaded custom fields", "count", len(defs))
	return nil
}

// FindRegistryWorkflowFiles returns all workflow.yaml files that exist,
// without the views-filtering that FindWorkflowFiles applies.
// Used by registry loaders (statuses, custom fields) that need to read
// configuration sections regardless of whether the file defines views.
func FindRegistryWorkflowFiles() []string {
	pm := mustGetPathManager()

	candidates := []string{
		pm.UserConfigWorkflowFile(),
		filepath.Join(pm.ProjectConfigDir(), defaultWorkflowFilename),
		defaultWorkflowFilename, // relative to cwd
	}

	var result []string
	seen := make(map[string]bool)

	for _, path := range candidates {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if seen[abs] {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		seen[abs] = true
		result = append(result, path)
	}

	return result
}

// readCustomFieldsFromFile reads the fields: section from a single workflow.yaml.
func readCustomFieldsFromFile(path string) ([]customFieldYAML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var fd customFieldFileData
	if err := yaml.Unmarshal(data, &fd); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return fd.Fields, nil
}

// convertCustomFieldDef converts a YAML field definition to a workflow.FieldDef.
func convertCustomFieldDef(def customFieldYAML) (workflow.FieldDef, error) {
	if def.Name == "" {
		return workflow.FieldDef{}, fmt.Errorf("field name is required")
	}

	if err := workflow.ValidateFieldName(def.Name); err != nil {
		return workflow.FieldDef{}, err
	}

	vt, err := parseFieldType(def.Type)
	if err != nil {
		return workflow.FieldDef{}, err
	}

	fd := workflow.FieldDef{
		Name:   def.Name,
		Type:   vt,
		Custom: true,
	}

	if vt == workflow.TypeEnum {
		if len(def.Values) == 0 {
			return workflow.FieldDef{}, fmt.Errorf("enum field requires non-empty values list")
		}
		fd.AllowedValues = make([]string, len(def.Values))
		copy(fd.AllowedValues, def.Values)
	} else if len(def.Values) > 0 {
		return workflow.FieldDef{}, fmt.Errorf("values list is only valid for enum fields")
	}

	return fd, nil
}

// parseFieldType maps workflow.yaml type strings to workflow.ValueType.
func parseFieldType(s string) (workflow.ValueType, error) {
	switch strings.ToLower(s) {
	case "text":
		return workflow.TypeString, nil
	case "integer":
		return workflow.TypeInt, nil
	case "boolean":
		return workflow.TypeBool, nil
	case "datetime":
		return workflow.TypeTimestamp, nil
	case "enum":
		return workflow.TypeEnum, nil
	case "stringlist":
		return workflow.TypeListString, nil
	case "taskidlist":
		return workflow.TypeListRef, nil
	default:
		return 0, fmt.Errorf("unknown field type %q (valid: text, integer, boolean, datetime, enum, stringList, taskIdList)", s)
	}
}

// fieldDefsEqual returns true if two FieldDefs are structurally identical
// (same name, same type, and for enums, same normalized values).
func fieldDefsEqual(a, b workflow.FieldDef) bool {
	if a.Name != b.Name || a.Type != b.Type {
		return false
	}
	if a.Type == workflow.TypeEnum {
		if len(a.AllowedValues) != len(b.AllowedValues) {
			return false
		}
		// require exact spelling and order for duplicate enum declarations
		for i := range a.AllowedValues {
			if a.AllowedValues[i] != b.AllowedValues[i] {
				return false
			}
		}
	}
	return true
}
