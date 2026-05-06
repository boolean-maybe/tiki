package runtime

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/workflow"
)

// workflowSchema adapts a snapshot of workflow.Fields(), config.GetStatusRegistry(),
// and config.GetTypeRegistry() into the ruki.Schema interface used by the parser
// and executor. The field catalog is snapshotted at construction time so an old
// schema never observes newly loaded custom fields through live global lookups.
type workflowSchema struct {
	fieldsByName map[string]ruki.FieldSpec // snapshotted at construction
}

// NewSchema constructs a ruki.Schema backed by the loaded workflow registries.
// Snapshots the current field catalog (built-in + custom) so the schema is
// immutable after creation. Must be called after config.LoadStatusRegistry()
// (and config.LoadCustomFields() if custom fields are in use).
func NewSchema() ruki.Schema {
	fields := workflow.Fields() // includes custom fields
	byName := make(map[string]ruki.FieldSpec, len(fields))
	for _, fd := range fields {
		spec := ruki.FieldSpec{
			Name:   fd.Name,
			Type:   mapValueType(fd.Type),
			Custom: fd.Custom,
		}
		if values := enumAllowedValues(fd, config.GetStatusRegistry(), config.GetTypeRegistry()); values != nil {
			spec.AllowedValues = values
		} else if fd.AllowedValues != nil {
			spec.AllowedValues = make([]string, len(fd.AllowedValues))
			copy(spec.AllowedValues, fd.AllowedValues)
		}
		byName[fd.Name] = spec
	}
	return &workflowSchema{
		fieldsByName: byName,
	}
}

// NewSchemaFromRegistries constructs a ruki.Schema from explicitly provided
// registries and custom field definitions, without touching global state.
// Used by init to validate a candidate workflow file.
func NewSchemaFromRegistries(statusReg *workflow.StatusRegistry, typeReg *workflow.TypeRegistry, customFields []workflow.FieldDef) ruki.Schema {
	fields := workflow.BuiltinFields()
	fields = append(fields, customFields...)
	byName := make(map[string]ruki.FieldSpec, len(fields))
	for _, fd := range fields {
		spec := ruki.FieldSpec{
			Name:   fd.Name,
			Type:   mapValueType(fd.Type),
			Custom: fd.Custom,
		}
		if values := enumAllowedValues(fd, statusReg, typeReg); values != nil {
			spec.AllowedValues = values
		} else if fd.AllowedValues != nil {
			spec.AllowedValues = make([]string, len(fd.AllowedValues))
			copy(spec.AllowedValues, fd.AllowedValues)
		}
		byName[fd.Name] = spec
	}
	return &workflowSchema{
		fieldsByName: byName,
	}
}

func (s *workflowSchema) Field(name string) (ruki.FieldSpec, bool) {
	spec, ok := s.fieldsByName[name]
	if !ok {
		return ruki.FieldSpec{}, false
	}
	// return a defensive copy so callers cannot mutate schema state
	out := spec
	if spec.AllowedValues != nil {
		out.AllowedValues = make([]string, len(spec.AllowedValues))
		copy(out.AllowedValues, spec.AllowedValues)
	}
	return out, true
}

// mapValueType converts workflow.ValueType to ruki.ValueType. Workflow's
// status, task type, and custom enum domains all collapse to ruki.ValueEnum.
func mapValueType(wt workflow.ValueType) ruki.ValueType {
	switch wt {
	case workflow.TypeString:
		return ruki.ValueString
	case workflow.TypeInt:
		return ruki.ValueInt
	case workflow.TypeDate:
		return ruki.ValueDate
	case workflow.TypeTimestamp:
		return ruki.ValueTimestamp
	case workflow.TypeDuration:
		return ruki.ValueDuration
	case workflow.TypeBool:
		return ruki.ValueBool
	case workflow.TypeID:
		return ruki.ValueID
	case workflow.TypeRef:
		return ruki.ValueRef
	case workflow.TypeRecurrence:
		return ruki.ValueRecurrence
	case workflow.TypeListString:
		return ruki.ValueListString
	case workflow.TypeListRef:
		return ruki.ValueListRef
	case workflow.TypeStatus, workflow.TypeTaskType, workflow.TypeEnum:
		return ruki.ValueEnum
	default:
		return ruki.ValueString
	}
}

func enumAllowedValues(fd workflow.FieldDef, statusReg *workflow.StatusRegistry, typeReg *workflow.TypeRegistry) []string {
	switch fd.Type {
	case workflow.TypeStatus:
		keys := statusReg.Keys()
		values := make([]string, len(keys))
		for i, key := range keys {
			values[i] = string(key)
		}
		return values
	case workflow.TypeTaskType:
		keys := typeReg.Keys()
		values := make([]string, len(keys))
		for i, key := range keys {
			values[i] = string(key)
		}
		return values
	case workflow.TypeEnum:
		values := make([]string, len(fd.AllowedValues))
		copy(values, fd.AllowedValues)
		return values
	default:
		return nil
	}
}
