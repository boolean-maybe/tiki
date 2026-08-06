package runtime

import (
	"github.com/boolean-maybe/ruki"
	"github.com/boolean-maybe/tiki/workflow"
)

// workflowSchema adapts a snapshot of workflow.Fields() (system + workflow)
// into the ruki.Schema interface used by the parser and executor. The field
// catalog is snapshotted at construction time so an old schema never
// observes newly loaded workflow fields through live global lookups.
type workflowSchema struct {
	fieldsByName map[string]ruki.FieldSpec
}

// NewSchema constructs a ruki.Schema backed by the loaded workflow fields.
// Snapshots the current field catalog (system + workflow) so the schema is
// immutable after creation. Must be called after config.LoadWorkflowFields().
func NewSchema() ruki.Schema {
	return NewSchemaFromFields(workflow.Fields())
}

// NewSchemaFromFields constructs a ruki.Schema from explicitly provided
// field definitions, without touching global state. Used by init/validation
// to validate a candidate workflow file. The caller is responsible for
// passing the system fields (workflow.SystemFields()) plus any workflow
// fields they want included.
func NewSchemaFromFields(fields []workflow.FieldDef) ruki.Schema {
	byName := make(map[string]ruki.FieldSpec, len(fields))
	for _, fd := range fields {
		spec := ruki.FieldSpec{
			Name:   fd.Name,
			Type:   mapValueType(fd.Type),
			Custom: fd.Custom,
		}
		if vals := fd.AllowedValues(); vals != nil {
			spec.AllowedValues = vals
		}
		byName[fd.Name] = spec
	}
	return &workflowSchema{fieldsByName: byName}
}

func (s *workflowSchema) Field(name string) (ruki.FieldSpec, bool) {
	spec, ok := s.fieldsByName[name]
	if !ok {
		return ruki.FieldSpec{}, false
	}
	out := spec
	if spec.AllowedValues != nil {
		out.AllowedValues = make([]string, len(spec.AllowedValues))
		copy(out.AllowedValues, spec.AllowedValues)
	}
	return out, true
}

// mapValueType converts workflow.ValueType to ruki.ValueType.
func mapValueType(wt workflow.ValueType) ruki.ValueType {
	switch wt {
	case workflow.TypeString, workflow.TypeUser:
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
	case workflow.TypeEnum:
		return ruki.ValueEnum
	default:
		return ruki.ValueString
	}
}
