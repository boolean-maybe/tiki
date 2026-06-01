package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	collectionutil "github.com/boolean-maybe/ruki/collections"
	"github.com/boolean-maybe/tiki/workflow"
	"gopkg.in/yaml.v3"
)

// customFieldYAML represents a single field entry in workflow.yaml fields:.
type customFieldYAML struct {
	Name    string          `yaml:"name"`
	Type    string          `yaml:"type"`
	Values  []enumValueYAML `yaml:"values,omitempty"`  // enum only
	Default interface{}     `yaml:"default,omitempty"` // creation default for non-enum
}

// customFieldFileData is the minimal YAML structure for reading fields from
// workflow.yaml. statuses: and types: are accepted only to produce a clear
// migration error if a legacy file is encountered.
type customFieldFileData struct {
	Fields   []customFieldYAML `yaml:"fields"`
	Statuses yaml.Node         `yaml:"statuses"`
	Types    yaml.Node         `yaml:"types"`
}

// enumValueYAML is one entry in fields[].values. Both scalar form ("foo")
// and structured form (value: foo, label: ..., visual: ..., default: ...)
// are supported. Legacy keys (active:, done:, emoji:) are explicitly
// rejected so users get a clear migration error.
type enumValueYAML struct {
	Value      string
	Label      string
	Visual     string
	Default    bool
	HasDefault bool
	Structured bool
}

func (v *enumValueYAML) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		v.Value = node.Value
		return nil
	case yaml.MappingNode:
		v.Structured = true
		return v.unmarshalMapping(node)
	default:
		return fmt.Errorf("enum value must be a string or mapping, got YAML kind %d", node.Kind)
	}
}

func (v *enumValueYAML) unmarshalMapping(node *yaml.Node) error {
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "value":
			if err := val.Decode(&v.Value); err != nil {
				return fmt.Errorf("value: %w", err)
			}
		case "label":
			if err := val.Decode(&v.Label); err != nil {
				return fmt.Errorf("label: %w", err)
			}
		case "visual":
			if err := val.Decode(&v.Visual); err != nil {
				return fmt.Errorf("visual: %w", err)
			}
		case "default":
			if err := val.Decode(&v.Default); err != nil {
				return fmt.Errorf("default: %w", err)
			}
			v.HasDefault = true
		case "emoji":
			return fmt.Errorf("enum value key %q was renamed to %q — move the glyph into visual: (it accepts the same content plus optional <role> color markup)", key, "visual")
		case "active", "done":
			return fmt.Errorf("enum value key %q is no longer supported; status semantics are not built into the runtime — remove it (rendering hints can use the visual: field)", key)
		default:
			return fmt.Errorf("unknown enum value key %q (valid keys: value, label, visual, default)", key)
		}
	}
	return nil
}

// workflowFieldsLoaded tracks whether LoadWorkflowFields has been called.
var workflowFieldsLoaded atomic.Bool

// RequireWorkflowFieldsLoaded returns an error if LoadWorkflowFields has not
// been called yet. Intended for use by store/template code that needs the
// workflow field catalog to be ready but should not auto-load it from disk.
func RequireWorkflowFieldsLoaded() error {
	if !workflowFieldsLoaded.Load() {
		return fmt.Errorf("workflow fields not loaded; call config.LoadWorkflowFields() first")
	}
	return nil
}

// MarkWorkflowFieldsLoadedForTest sets the loaded flag without loading from
// disk. Use in tests that call workflow.RegisterWorkflowFields directly.
func MarkWorkflowFieldsLoadedForTest() {
	workflowFieldsLoaded.Store(true)
}

// ResetWorkflowFieldsLoadedForTest clears the loaded flag.
func ResetWorkflowFieldsLoadedForTest() {
	workflowFieldsLoaded.Store(false)
}

// LoadWorkflowFields reads the fields: section from the highest-priority
// workflow.yaml and registers it as the runtime field catalog. Returns an
// error if no workflow.yaml is found or if the file is malformed.
func LoadWorkflowFields() error {
	if err := checkWorkflowFileVersion(); err != nil {
		return err
	}

	files := FindWorkflowFiles()
	if len(files) == 0 {
		return fmt.Errorf("no workflow.yaml found; workflow fields must be defined in workflow.yaml")
	}

	defs, err := loadWorkflowFieldsFromFile(files[0])
	if err != nil {
		return fmt.Errorf("loading workflow fields from %s: %w", files[0], err)
	}

	if err := workflow.RegisterWorkflowFields(defs); err != nil {
		return fmt.Errorf("registering workflow fields from %s: %w", files[0], err)
	}

	workflowFieldsLoaded.Store(true)
	slog.Debug("loaded workflow fields", "count", len(defs), "file", files[0])
	return nil
}

// LoadWorkflowFieldsFromFile validates and loads workflow field definitions
// from a single explicit workflow file path, without touching global state.
// Used by init to validate a candidate workflow file.
func LoadWorkflowFieldsFromFile(path string) ([]workflow.FieldDef, error) {
	if err := CheckFileVersionCompatibility(path); err != nil {
		return nil, err
	}
	defs, err := loadWorkflowFieldsFromFile(path)
	if err != nil {
		return nil, err
	}
	if err := workflow.ValidateWorkflowFields(defs); err != nil {
		return nil, err
	}
	return defs, nil
}

// ClearWorkflowFields clears the loaded workflow field catalog. Intended for
// test teardown.
func ClearWorkflowFields() {
	workflow.ClearWorkflowFields()
	workflowFieldsLoaded.Store(false)
}

// ResetWorkflowFieldsForTest replaces the loaded workflow field catalog with
// the given defs. Intended for tests only.
func ResetWorkflowFieldsForTest(defs []workflow.FieldDef) {
	if err := workflow.RegisterWorkflowFields(defs); err != nil {
		panic(fmt.Sprintf("ResetWorkflowFieldsForTest: %v", err))
	}
	workflowFieldsLoaded.Store(true)
}

// FindRegistryWorkflowFiles is retained as an alias for callers in the test
// helpers; delegates to FindWorkflowFiles.
func FindRegistryWorkflowFiles() []string {
	return FindWorkflowFiles()
}

func loadWorkflowFieldsFromFile(path string) ([]workflow.FieldDef, error) {
	rawDefs, err := readCustomFieldsFromFile(path)
	if err != nil {
		return nil, err
	}
	defs := make([]workflow.FieldDef, 0, len(rawDefs))
	for _, raw := range rawDefs {
		if workflow.IsSystemField(raw.Name) {
			return nil, fmt.Errorf("workflow field %q collides with reserved system field; remove it from workflow.yaml", raw.Name)
		}
		def, err := convertWorkflowFieldDef(raw)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", raw.Name, err)
		}
		defs = append(defs, def)
	}
	// Preserve declaration order from workflow.yaml — no name-based ranking.
	return defs, nil
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
	if err := rejectLegacyRegistrySections(fd); err != nil {
		return nil, err
	}
	return fd.Fields, nil
}

func rejectLegacyRegistrySections(fd customFieldFileData) error {
	if fd.Statuses.Kind != 0 {
		return fmt.Errorf("top-level statuses: is no longer supported; define status as a fields: enum")
	}
	if fd.Types.Kind != 0 {
		return fmt.Errorf("top-level types: is no longer supported; define type as a fields: enum")
	}
	return nil
}

// convertWorkflowFieldDef converts a YAML field definition into a workflow.FieldDef.
func convertWorkflowFieldDef(def customFieldYAML) (workflow.FieldDef, error) {
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
		if def.Default != nil {
			return workflow.FieldDef{}, fmt.Errorf("field-level default is not supported for enum fields; mark one enum value default: true")
		}
		fd.EnumValues = make([]workflow.EnumValue, 0, len(def.Values))
		for _, v := range def.Values {
			if v.Value == "" {
				return workflow.FieldDef{}, fmt.Errorf("enum value has empty value")
			}
			visual := strings.TrimSpace(v.Visual)
			if err := workflow.ValidateVisualMarkup(visual); err != nil {
				return workflow.FieldDef{}, fmt.Errorf("enum value %q: %w", v.Value, err)
			}
			fd.EnumValues = append(fd.EnumValues, workflow.EnumValue{
				Value:   v.Value,
				Label:   v.Label,
				Visual:  visual,
				Default: v.Default,
			})
		}
		return fd, nil
	}

	if len(def.Values) > 0 {
		return workflow.FieldDef{}, fmt.Errorf("values list is only valid for enum fields")
	}
	if def.Default != nil {
		coerced, err := coerceFieldDefault(vt, def.Default, nil)
		if err != nil {
			return workflow.FieldDef{}, fmt.Errorf("invalid default: %w", err)
		}
		fd.DefaultValue = coerced
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
	case "date":
		return workflow.TypeDate, nil
	case "enum":
		return workflow.TypeEnum, nil
	case "stringlist":
		return workflow.TypeListString, nil
	case "tikiidlist":
		return workflow.TypeListRef, nil
	case "recurrence":
		return workflow.TypeRecurrence, nil
	default:
		return 0, fmt.Errorf("unknown field type %q (valid: text, integer, boolean, date, datetime, enum, stringList, tikiIdList, recurrence)", s)
	}
}

// coerceFieldDefault validates and coerces a raw YAML default value to the
// expected Go type for the given field type.
func coerceFieldDefault(vt workflow.ValueType, raw interface{}, allowed []string) (interface{}, error) {
	switch vt {
	case workflow.TypeString:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", raw)
		}
		return s, nil

	case workflow.TypeInt:
		switch v := raw.(type) {
		case int:
			return v, nil
		case float64:
			if v != float64(int(v)) {
				return nil, fmt.Errorf("expected integer, got float %v", v)
			}
			return int(v), nil
		default:
			return nil, fmt.Errorf("expected integer, got %T", raw)
		}

	case workflow.TypeBool:
		b, ok := raw.(bool)
		if !ok {
			return nil, fmt.Errorf("expected boolean, got %T", raw)
		}
		return b, nil

	case workflow.TypeTimestamp, workflow.TypeDate:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("expected datetime string, got %T", raw)
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, nil
		}
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t, nil
		}
		return nil, fmt.Errorf("cannot parse timestamp %q (expected RFC3339 or YYYY-MM-DD)", s)

	case workflow.TypeListString:
		return coerceStringList(raw)

	case workflow.TypeListRef:
		ss, err := coerceStringList(raw)
		if err != nil {
			return nil, err
		}
		return collectionutil.NormalizeRefSet(ss), nil

	default:
		_ = allowed
		return nil, fmt.Errorf("default values not supported for field type %d", vt)
	}
}

func coerceStringList(raw interface{}) ([]string, error) {
	switch v := raw.(type) {
	case []interface{}:
		result := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("element %d: expected string, got %T", i, item)
			}
			result = append(result, s)
		}
		return collectionutil.NormalizeStringSet(result), nil
	case []string:
		cp := make([]string, len(v))
		copy(cp, v)
		return collectionutil.NormalizeStringSet(cp), nil
	default:
		return nil, fmt.Errorf("expected list, got %T", raw)
	}
}
