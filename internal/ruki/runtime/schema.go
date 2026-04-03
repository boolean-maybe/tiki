package runtime

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/workflow"
)

// workflowSchema adapts workflow.Fields(), config.GetStatusRegistry(), and
// config.GetTypeRegistry() into the ruki.Schema interface used by the parser
// and executor.
type workflowSchema struct {
	statusReg *workflow.StatusRegistry
	typeReg   *workflow.TypeRegistry
}

// NewSchema constructs a ruki.Schema backed by the loaded workflow registries.
// Must be called after config.LoadStatusRegistry().
func NewSchema() ruki.Schema {
	return &workflowSchema{
		statusReg: config.GetStatusRegistry(),
		typeReg:   config.GetTypeRegistry(),
	}
}

func (s *workflowSchema) Field(name string) (ruki.FieldSpec, bool) {
	fd, ok := workflow.Field(name)
	if !ok {
		return ruki.FieldSpec{}, false
	}
	return ruki.FieldSpec{
		Name: fd.Name,
		Type: mapValueType(fd.Type),
	}, true
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
	default:
		return ruki.ValueString
	}
}
