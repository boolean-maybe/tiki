package taskdetail

import (
	"fmt"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/rivo/tview"
)

// SemanticType identifies how a configurable detail field is rendered and
// edited. The registry routes a field by its semantic type to the matching
// renderer/editor factory; immutable types like ID/Author are handled by the
// title block, not by this registry.
type SemanticType string

const (
	SemanticStatus     SemanticType = "status"
	SemanticType_      SemanticType = "type"
	SemanticPriority   SemanticType = "priority"
	SemanticText       SemanticType = "text"
	SemanticInteger    SemanticType = "integer"
	SemanticBoolean    SemanticType = "boolean"
	SemanticDate       SemanticType = "date"
	SemanticDateTime   SemanticType = "datetime"
	SemanticRecurrence SemanticType = "recurrence"
	SemanticEnum       SemanticType = "enum"
	SemanticStringList SemanticType = "string_list"
	SemanticTaskIDList SemanticType = "task_id_list"
)

// EditorCapability tracks whether the type UI registry supports in-place
// editing for a semantic type. Phase 1 implements only a small subset; the
// rest are recorded as stubs so Phase 2 can wire them in without changing
// callers.
type EditorCapability int

const (
	// EditorStub: renderer exists but no in-place editor yet — pressing Edit
	// for this field type should produce predictable stub behavior (no-op or
	// surfacing a "not yet supported" message).
	EditorStub EditorCapability = iota
	// EditorImplemented: renderer and editor are both available.
	EditorImplemented
)

// FieldRenderer renders a tiki's value for the field as a read-only primitive.
type FieldRenderer func(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive

// FieldEditorWidget is the focusable primitive returned by an editor factory.
// It must satisfy tview.Primitive so the view can give it focus, and it
// exposes its current display value so the detail view can route the value
// to the per-field save callback.
type FieldEditorWidget interface {
	tview.Primitive
	GetText() string
}

// FieldEditor builds an in-place editor widget for a tiki's current value.
// The onChange callback fires whenever the user changes the editor value
// (typing, arrow-key cycling, ...) and carries the new display value. Phase
// 2 implements concrete editors for status, type, and priority; other
// semantic types return nil and remain stubs surfaced via Capability.
type FieldEditor func(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget

// TypeUI bundles the rendering and editing primitives for a semantic type.
type TypeUI struct {
	Render     FieldRenderer
	Edit       FieldEditor
	Capability EditorCapability
}

// FieldDescriptor describes a single configurable detail-view field.
//
// Getter/Setter are kept generic (interface{}) so the registry can support
// fields beyond the current schema-known set — including future custom
// fields — without re-shaping this struct each time.
type FieldDescriptor struct {
	Name            string                              // canonical field name (matches tiki.Field*)
	Label           string                              // user-facing label
	Semantic        SemanticType                        // routes to TypeUI registry
	EditField       model.EditField                     // model EditField mapping (for hint/status integration)
	Get             func(tk *tikipkg.Tiki) any          // current value (typed or nil when absent)
	Set             func(tk *tikipkg.Tiki, v any) error // future Phase 2 hook; may be nil for read-only fields
	ReadOnly        bool                                // true for immutable fields (created, updated, author, …)
	EditTraversable bool                                // participates in Tab traversal during edit mode
}

// fieldRegistry maps a field name to its descriptor. Fields not present here
// can still be requested via metadata; the renderer for the unknown field
// returns a "(no renderer)" placeholder so misconfiguration is visible
// without crashing the view.
var fieldRegistry = map[string]FieldDescriptor{}

// typeRegistry maps a semantic type to its rendering/editing primitives.
var typeRegistry = map[SemanticType]TypeUI{}

func init() {
	registerBuiltinTypes()
	registerBuiltinFields()
	publishRenderableFields()
}

// publishRenderableFields tells the plugin loader which metadata field names
// the detail view can actually render. Without this, the workflow loader
// would accept fields it knows the schema for (e.g. createdAt, custom enums)
// but the view would render them as "(no renderer)" placeholders.
func publishRenderableFields() {
	for name := range fieldRegistry {
		plugin.RegisterRenderableMetadataField(name)
	}
}

// LookupField returns the descriptor for a field name. Returns ok=false for
// unknown names so the caller can render a placeholder.
func LookupField(name string) (FieldDescriptor, bool) {
	fd, ok := fieldRegistry[name]
	return fd, ok
}

// LookupType returns the TypeUI for a semantic type. Returns ok=false for
// unregistered types.
func LookupType(t SemanticType) (TypeUI, bool) {
	ui, ok := typeRegistry[t]
	return ui, ok
}

// registerBuiltinFields wires the schema-known fields into the registry.
// Phase 1 implements the three default fields (status, type, priority) and
// the rest are recorded as read-only renderers using existing helpers.
func registerBuiltinFields() {
	fieldRegistry[tikipkg.FieldStatus] = FieldDescriptor{
		Name:            tikipkg.FieldStatus,
		Label:           "Status",
		Semantic:        SemanticStatus,
		EditField:       model.EditFieldStatus,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField(tikipkg.FieldStatus); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldType] = FieldDescriptor{
		Name:            tikipkg.FieldType,
		Label:           "Type",
		Semantic:        SemanticType_,
		EditField:       model.EditFieldType,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField(tikipkg.FieldType); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldPriority] = FieldDescriptor{
		Name:            tikipkg.FieldPriority,
		Label:           "Priority",
		Semantic:        SemanticPriority,
		EditField:       model.EditFieldPriority,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.IntField(tikipkg.FieldPriority); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldPoints] = FieldDescriptor{
		Name:            tikipkg.FieldPoints,
		Label:           "Points",
		Semantic:        SemanticInteger,
		EditField:       model.EditFieldPoints,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.IntField(tikipkg.FieldPoints); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldAssignee] = FieldDescriptor{
		Name:            tikipkg.FieldAssignee,
		Label:           "Assignee",
		Semantic:        SemanticText,
		EditField:       model.EditFieldAssignee,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField(tikipkg.FieldAssignee); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldDue] = FieldDescriptor{
		Name:            tikipkg.FieldDue,
		Label:           "Due",
		Semantic:        SemanticDate,
		EditField:       model.EditFieldDue,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.TimeField(tikipkg.FieldDue); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldRecurrence] = FieldDescriptor{
		Name:            tikipkg.FieldRecurrence,
		Label:           "Recurrence",
		Semantic:        SemanticRecurrence,
		EditField:       model.EditFieldRecurrence,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField(tikipkg.FieldRecurrence); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldTags] = FieldDescriptor{
		Name:            tikipkg.FieldTags,
		Label:           "Tags",
		Semantic:        SemanticStringList,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringSliceField(tikipkg.FieldTags); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldDependsOn] = FieldDescriptor{
		Name:            tikipkg.FieldDependsOn,
		Label:           "Depends On",
		Semantic:        SemanticTaskIDList,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn); return v },
		EditTraversable: true,
	}
}

// registerBuiltinTypes wires renderers for each semantic type. Phase 2 adds
// concrete editor factories for the three default fields (status, type,
// priority); the rest remain stubs surfaced via Capability so callers can
// distinguish "renderer exists, editor stub" from "renderer + editor
// implemented" without invoking a nil factory.
func registerBuiltinTypes() {
	typeRegistry[SemanticStatus] = TypeUI{
		Render:     renderStatusValue,
		Edit:       editStatusValue,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticType_] = TypeUI{
		Render:     renderTypeValue,
		Edit:       editTypeValue,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticPriority] = TypeUI{
		Render:     renderPriorityValue,
		Edit:       editPriorityValue,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticText] = TypeUI{
		Render:     renderTextValue,
		Capability: EditorStub,
	}
	typeRegistry[SemanticInteger] = TypeUI{
		Render:     renderIntegerValue,
		Capability: EditorStub,
	}
	typeRegistry[SemanticBoolean] = TypeUI{
		Render:     renderBooleanValue,
		Capability: EditorStub,
	}
	typeRegistry[SemanticDate] = TypeUI{
		Render:     renderDateValue,
		Capability: EditorStub,
	}
	typeRegistry[SemanticDateTime] = TypeUI{
		Render:     renderDateTimeValue,
		Capability: EditorStub,
	}
	typeRegistry[SemanticRecurrence] = TypeUI{
		Render:     renderRecurrenceValue,
		Capability: EditorStub,
	}
	typeRegistry[SemanticEnum] = TypeUI{
		Render:     renderEnumValue,
		Capability: EditorStub,
	}
	typeRegistry[SemanticStringList] = TypeUI{
		Render:     renderStringListValue,
		Capability: EditorStub,
	}
	typeRegistry[SemanticTaskIDList] = TypeUI{
		Render:     renderTaskIDListValue,
		Capability: EditorStub,
	}
}

// renderConfiguredField looks up the field descriptor and routes through the
// type registry to produce a primitive. Unknown fields and unknown semantic
// types return a placeholder text view so misconfiguration is visible.
func renderConfiguredField(name string, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, ok := LookupField(name)
	if !ok {
		return placeholderRow(fmt.Sprintf("%s: (unknown field)", name))
	}
	ui, ok := LookupType(fd.Semantic)
	if !ok || ui.Render == nil {
		return placeholderRow(fmt.Sprintf("%s: (no renderer)", fd.Label))
	}
	return ui.Render(tk, withFieldDescriptor(ctx, fd))
}

// withFieldDescriptor passes the descriptor's label down through the existing
// context shape used by the legacy renderers. Existing helpers don't read it
// today, so this is a no-op for them; the new generic renderers below use it
// to produce the leading "Label: " text.
func withFieldDescriptor(ctx FieldRenderContext, _ FieldDescriptor) FieldRenderContext {
	return ctx
}

// placeholderRow produces a single-line text view used for unknown/stub
// renderers so the layout still allocates a row.
func placeholderRow(text string) tview.Primitive {
	tv := tview.NewTextView().SetDynamicColors(true).SetText(text)
	tv.SetBorderPadding(0, 0, 0, 0)
	return tv
}

// labeledLine returns a "Label: value" row using the dim label / value colors
// already used by the legacy renderers, keeping visual continuity with the
// hardcoded layout.
func labeledLine(label, value string, colors *config.ColorConfig) tview.Primitive {
	labelTag := colors.TaskDetailLabelText.Tag().String()
	valueTag := colors.TaskDetailValueText.Tag().String()
	text := fmt.Sprintf("%s%-12s%s%s", labelTag, label+":", valueTag, value)
	tv := tview.NewTextView().SetDynamicColors(true).SetText(text)
	tv.SetBorderPadding(0, 0, 0, 0)
	return tv
}

// --- semantic-type renderers ---

func renderStatusValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	return RenderStatusText(tk, ctx)
}

func renderTypeValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	return RenderTypeText(tk, ctx)
}

func renderPriorityValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	return RenderPriorityText(tk, ctx)
}

func renderTextValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	// Phase 1 only routes text-type for assignee; other text fields will
	// follow once their descriptors land.
	value, _, _ := tk.StringField(tikipkg.FieldAssignee)
	if value == "" {
		value = "Unassigned"
	}
	return labeledLine("Assignee", value, ctx.Colors)
}

func renderIntegerValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, _ := LookupField(tikipkg.FieldPoints)
	v, present, _ := tk.IntField(fd.Name)
	value := "─"
	if present {
		value = fmt.Sprintf("%d", v)
	}
	return labeledLine(fd.Label, value, ctx.Colors)
}

func renderBooleanValue(_ *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	return labeledLine("Boolean", "(stub)", ctx.Colors)
}

func renderDateValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	due, _, _ := tk.TimeField(tikipkg.FieldDue)
	value := "None"
	if !due.IsZero() {
		value = due.Format("2006-01-02")
	}
	return labeledLine("Due", value, ctx.Colors)
}

func renderDateTimeValue(_ *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	return labeledLine("DateTime", "(stub)", ctx.Colors)
}

func renderRecurrenceValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	recurrenceStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
	display := taskpkg.RecurrenceDisplay(taskpkg.Recurrence(recurrenceStr))
	return labeledLine("Recurrence", display, ctx.Colors)
}

func renderEnumValue(_ *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	return labeledLine("Enum", "(stub)", ctx.Colors)
}

func renderStringListValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	if len(tags) == 0 {
		return labeledLine("Tags", "(none)", ctx.Colors)
	}
	value := ""
	for i, t := range tags {
		if i > 0 {
			value += ", "
		}
		value += t
	}
	return labeledLine("Tags", value, ctx.Colors)
}

func renderTaskIDListValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	deps, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn)
	if len(deps) == 0 {
		return labeledLine("Depends On", "(none)", ctx.Colors)
	}
	value := ""
	for i, d := range deps {
		if i > 0 {
			value += ", "
		}
		value += d
	}
	return labeledLine("Depends On", value, ctx.Colors)
}

// --- Phase 2 editor factories ---
//
// Each factory builds an EditSelectList configured with the canonical option
// list for the field, seeded with the tiki's current value, and wires the
// onChange callback to fire whenever the user types or arrows through the
// list. The view supplies onChange to forward the change to the
// per-semantic save callback.

// editStatusValue builds the status editor.
func editStatusValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	allStatuses := taskpkg.AllStatuses()
	options := make([]string, len(allStatuses))
	for i, s := range allStatuses {
		options[i] = taskpkg.StatusDisplay(s)
	}
	statusStr, _, _ := tk.StringField(tikipkg.FieldStatus)
	editor := component.NewEditSelectList(options, false)
	editor.SetLabel(getFocusMarker(ctx.Colors) + "Status:   ")
	editor.SetInitialValue(taskpkg.StatusDisplay(taskpkg.Status(statusStr)))
	editor.SetSubmitHandler(func(text string) {
		if onChange != nil {
			onChange(text)
		}
	})
	return editor
}

// editTypeValue builds the type editor.
func editTypeValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	allTypes := taskpkg.AllTypes()
	options := make([]string, len(allTypes))
	for i, t := range allTypes {
		options[i] = taskpkg.TypeDisplay(t)
	}
	typeStr, _, _ := tk.StringField(tikipkg.FieldType)
	editor := component.NewEditSelectList(options, false)
	editor.SetLabel(getFocusMarker(ctx.Colors) + "Type:     ")
	editor.SetInitialValue(taskpkg.TypeDisplay(taskpkg.Type(typeStr)))
	editor.SetSubmitHandler(func(text string) {
		if onChange != nil {
			onChange(text)
		}
	})
	return editor
}

// editPriorityValue builds the priority editor.
func editPriorityValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	options := taskpkg.AllPriorityDisplayValues()
	priority, _, _ := tk.IntField(tikipkg.FieldPriority)
	editor := component.NewEditSelectList(options, false)
	editor.SetLabel(getFocusMarker(ctx.Colors) + "Priority: ")
	editor.SetInitialValue(taskpkg.PriorityDisplay(priority))
	editor.SetSubmitHandler(func(text string) {
		if onChange != nil {
			onChange(text)
		}
	})
	return editor
}

// buildFieldEditor is a convenience that looks up the type registry and
// returns the editor widget if the type supports editing. Returns nil for
// stub types or unknown fields, letting callers fall back to read-only
// rendering. ctx.FocusedField is set to the descriptor's EditField so
// renderers and editors can paint the focused state consistently.
func buildFieldEditor(name string, tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	fd, ok := LookupField(name)
	if !ok {
		return nil
	}
	ui, ok := LookupType(fd.Semantic)
	if !ok || ui.Capability != EditorImplemented || ui.Edit == nil {
		return nil
	}
	return ui.Edit(tk, ctx, onChange)
}

// FieldHasEditor reports whether the named field has a registered, fully
// implemented editor. Used by the detail view to skip stub fields during
// Tab traversal.
func FieldHasEditor(name string) bool {
	fd, ok := LookupField(name)
	if !ok {
		return false
	}
	ui, ok := LookupType(fd.Semantic)
	if !ok {
		return false
	}
	return ui.Capability == EditorImplemented && ui.Edit != nil
}
