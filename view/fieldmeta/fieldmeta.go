// Package fieldmeta holds the tview-free metadata that decides how a
// workflow-declared field is classified for the detail view: its semantic
// type, whether it is read-only, and whether it has a generic in-place editor.
//
// It exists as a leaf so BOTH the view (view/tikidetail) and the controller
// can share one editability authority without an import cycle — view/tikidetail
// imports controller, so the controller cannot import view/tikidetail. The
// tview-coupled rendering/editing registry stays in view/tikidetail; only the
// pure classification facts live here.
package fieldmeta

import "github.com/boolean-maybe/tiki/workflow"

// SemanticType identifies how a detail-view field is rendered and edited. The
// view registry keys its renderer/editor factories by this type; here it is
// used only as the key into the tview-free editability table.
type SemanticType string

const (
	SemanticEnum       SemanticType = "enum"
	SemanticText       SemanticType = "text"
	SemanticInteger    SemanticType = "integer"
	SemanticBoolean    SemanticType = "boolean"
	SemanticDate       SemanticType = "date"
	SemanticDateTime   SemanticType = "datetime"
	SemanticRecurrence SemanticType = "recurrence"
	SemanticStringList SemanticType = "string_list"
	SemanticTikiIDList SemanticType = "tiki_id_list"
)

// ForValueType bridges the workflow catalog's ValueType to a SemanticType, so
// classification resolves even for catalog-only fields that have no static
// descriptor (e.g. user-declared enums or a custom datetime like dueBy). The
// string family (TypeString/TypeID/TypeRef/TypeDuration) and any unmapped type
// fall through to SemanticText.
func ForValueType(t workflow.ValueType) SemanticType {
	switch t {
	case workflow.TypeEnum:
		return SemanticEnum
	case workflow.TypeInt:
		return SemanticInteger
	case workflow.TypeBool:
		return SemanticBoolean
	case workflow.TypeDate:
		return SemanticDate
	case workflow.TypeTimestamp:
		return SemanticDateTime
	case workflow.TypeRecurrence:
		return SemanticRecurrence
	case workflow.TypeListString:
		return SemanticStringList
	case workflow.TypeListRef:
		return SemanticTikiIDList
	}
	return SemanticText
}

// editableSemantics is the single source of truth for which semantic types the
// detail view can edit in place. The view registry derives its per-type
// EditorCapability from this table (EditorImplemented iff true), so a new
// editable type is enabled by flipping ONE entry here — in lockstep with adding
// its Edit factory in the view registry (registerBuiltinTypes panics if a type
// is marked editable here but has no factory). integer and boolean flip to true
// when their editors land; tiki_id_list stays false (dependency-picker
// follow-up).
var editableSemantics = map[SemanticType]bool{
	SemanticEnum:       true,
	SemanticText:       true,
	SemanticInteger:    true,
	SemanticBoolean:    true,
	SemanticDate:       true,
	SemanticDateTime:   true,
	SemanticRecurrence: true,
	SemanticStringList: true,
	SemanticTikiIDList: false,
}

// readOnlyFields names the fields that must never be edited. They are the
// computed system fields whose static descriptors carry ReadOnly=true; a
// catalog-only field is never read-only. Kept here (not derived from the view
// registry) so the controller can consult it without importing the view.
var readOnlyFields = map[string]bool{
	"createdBy": true,
	"createdAt": true,
	"updatedAt": true,
}

// SemanticEditable reports whether a semantic type has a generic in-place
// editor. It is the authority the view registry reads to set each type's
// EditorCapability, keeping "editable" stated exactly once.
func SemanticEditable(sem SemanticType) bool {
	return editableSemantics[sem]
}

// FieldIsReadOnly reports whether the named field must never be edited.
func FieldIsReadOnly(name string) bool {
	return readOnlyFields[name]
}

// FieldHasEditor reports whether the user can focus and edit the named field.
// It is the single predicate both the view (focusability) and the controller
// (save-handler wiring) derive from, so "focusable" and "saveable" can never
// drift for any field — including catalog-only fields with no static
// descriptor. A field is editable iff it is not read-only, resolves to a
// semantic type, and that type is in editableSemantics. The former
// {TypeEnum, TypeTimestamp} controller whitelist is gone.
func FieldHasEditor(name string) bool {
	if FieldIsReadOnly(name) {
		return false
	}
	wfd, ok := workflow.Field(name)
	if !ok {
		return false
	}
	return SemanticEditable(ForValueType(wfd.Type))
}
