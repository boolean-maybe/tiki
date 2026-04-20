package config

import (
	"fmt"
	"log/slog"
	"os"
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

// LoadCustomFields reads the fields: section from the single highest-priority
// workflow.yaml and registers them with workflow.RegisterCustomFields.
// Missing fields: section means no custom fields.
func LoadCustomFields() error {
	files := FindRegistryWorkflowFiles()
	if len(files) == 0 {
		workflow.ClearCustomFields()
		return nil
	}

	rawDefs, err := readCustomFieldsFromFile(files[0])
	if err != nil {
		return fmt.Errorf("reading custom fields from %s: %w", files[0], err)
	}

	if len(rawDefs) == 0 {
		workflow.ClearCustomFields()
		return nil
	}

	defs := make([]workflow.FieldDef, 0, len(rawDefs))
	for _, raw := range rawDefs {
		def, err := convertCustomFieldDef(raw)
		if err != nil {
			return fmt.Errorf("field %q in %s: %w", raw.Name, files[0], err)
		}
		defs = append(defs, def)
	}

	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})

	if err := workflow.RegisterCustomFields(defs); err != nil {
		return fmt.Errorf("registering custom fields: %w", err)
	}

	slog.Debug("loaded custom fields", "count", len(defs))
	return nil
}

// FindRegistryWorkflowFiles returns a single-element slice with the
// highest-priority workflow.yaml, or nil if none found.
// Delegates to FindWorkflowFiles — both now share the same semantics.
func FindRegistryWorkflowFiles() []string {
	return FindWorkflowFiles()
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
