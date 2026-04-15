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
	statusReg    *workflow.StatusRegistry
	typeReg      *workflow.TypeRegistry
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
		if fd.AllowedValues != nil {
			spec.AllowedValues = make([]string, len(fd.AllowedValues))
			copy(spec.AllowedValues, fd.AllowedValues)
		}
		byName[fd.Name] = spec
	}
	return &workflowSchema{
		statusReg:    config.GetStatusRegistry(),
		typeReg:      config.GetTypeRegistry(),
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

func (s *workflowSchema) NormalizeStatus(raw string) (string, bool) {
	def, ok := s.statusReg.Lookup(raw)
	if !ok {
		return "", false
	}
	return def.Key, true
}

func (s *workflowSchema) NormalizeType(raw string) (string, bool) {
	canonical, ok := s.typeReg.ParseType(raw)
	return string(canonical), ok
}

// mapValueType converts workflow.ValueType to ruki.ValueType.
// The two enums are defined in lockstep, so this is a 1:1 mapping.
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
	case workflow.TypeStatus:
		return ruki.ValueStatus
	case workflow.TypeTaskType:
		return ruki.ValueTaskType
	case workflow.TypeEnum:
		return ruki.ValueEnum
	default:
		return ruki.ValueString
	}
}
