package config

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"
	"gopkg.in/yaml.v3"
)

// customFieldYAML represents a single field entry in the workflow.yaml fields: section.
type customFieldYAML struct {
	Name    string          `yaml:"name"`
	Type    string          `yaml:"type"`
	Values  []enumValueYAML `yaml:"values,omitempty"`  // enum only
	Default interface{}     `yaml:"default,omitempty"` // optional creation default
}

// customFieldFileData is the minimal YAML structure for reading fields from workflow.yaml.
type customFieldFileData struct {
	Fields   []customFieldYAML `yaml:"fields"`
	Statuses yaml.Node         `yaml:"statuses"`
	Types    yaml.Node         `yaml:"types"`
}

type enumValueYAML struct {
	Value      string
	Label      string
	Emoji      string
	Active     bool
	Default    bool
	Done       bool
	HasActive  bool
	HasDefault bool
	HasDone    bool
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
		case "emoji":
			if err := val.Decode(&v.Emoji); err != nil {
				return fmt.Errorf("emoji: %w", err)
			}
		case "active":
			if err := val.Decode(&v.Active); err != nil {
				return fmt.Errorf("active: %w", err)
			}
			v.HasActive = true
		case "default":
			if err := val.Decode(&v.Default); err != nil {
				return fmt.Errorf("default: %w", err)
			}
			v.HasDefault = true
		case "done":
			if err := val.Decode(&v.Done); err != nil {
				return fmt.Errorf("done: %w", err)
			}
			v.HasDone = true
		default:
			return fmt.Errorf("unknown enum value key %q (valid keys: value, label, emoji, active, default, done)", key)
		}
	}
	return nil
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
// workflow-registry-based fields (status, type, custom fields) from
// workflow.yaml files. Callers must build a fresh ruki.Schema after this returns.
func LoadWorkflowRegistries() error {
	if err := checkWorkflowFileVersion(); err != nil {
		return err
	}
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
		if isRegistryField(raw.Name) {
			continue
		}
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

func isRegistryField(name string) bool {
	return name == "status" || name == "type"
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
		for i, v := range def.Values {
			if v.Structured {
				return workflow.FieldDef{}, fmt.Errorf("structured enum values are only supported for status and type fields")
			}
			fd.AllowedValues[i] = v.Value
		}
	} else if len(def.Values) > 0 {
		return workflow.FieldDef{}, fmt.Errorf("values list is only valid for enum fields")
	}

	if def.Default != nil {
		coerced, err := coerceFieldDefault(vt, def.Default, fd.AllowedValues)
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

// coerceFieldDefault validates and coerces a raw YAML default value to the
// expected Go type for the given field type. Returns an error if the value
// is incompatible.
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

	case workflow.TypeTimestamp:
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

	case workflow.TypeEnum:
		s, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", raw)
		}
		for _, v := range allowed {
			if v == s {
				return s, nil
			}
		}
		return nil, fmt.Errorf("value %q not in allowed values %v", s, allowed)

	case workflow.TypeListString:
		return coerceStringList(raw)

	case workflow.TypeListRef:
		ss, err := coerceStringList(raw)
		if err != nil {
			return nil, err
		}
		return collectionutil.NormalizeRefSet(ss), nil

	default:
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
