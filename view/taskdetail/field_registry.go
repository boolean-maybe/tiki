package taskdetail

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/gdamore/tcell/v2"
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
// editing for a semantic type.
type EditorCapability int

const (
	// EditorStub: renderer exists but no in-place editor yet.
	EditorStub EditorCapability = iota
	// EditorImplemented: renderer and editor are both available.
	EditorImplemented
)

// FieldRenderer renders a tiki's value for the field as a read-only primitive.
type FieldRenderer func(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive

// FieldEditorWidget is the focusable primitive returned by an editor factory.
//
// CycleValue advances the widget's value by `direction` steps (typically +1
// for Down/next, -1 for Up/prev). Returns true when the cycle was applied
// (e.g. moved to a new option, incremented an integer), false when the
// widget refuses (e.g. due editor in read-only mode when recurrence is set,
// or a non-cyclable widget like the tags textarea). Used by both views to
// route Up/Down keypresses through a single dispatcher rather than typed
// per-widget switch tables.
type FieldEditorWidget interface {
	tview.Primitive
	GetText() string
	CycleValue(direction int) bool
}

// FieldEditor builds an in-place editor widget for a tiki's current value.
// onChange fires with the editor's new typed value rendered as a string.
// Each factory owns the typed→string conversion (e.g. strconv.Itoa for
// points, RecurrenceEdit.GetValue() for recurrence) so the receiver can
// parse the string back to the typed value at the receive boundary.
type FieldEditor func(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget

// FieldHeightFn computes the row count a field needs at a given inner
// column width. Single-row types return 1; list types return 1 + wrapped
// content rows. The grid clamps the result to [1, rowsPerColumn].
type FieldHeightFn func(tk *tikipkg.Tiki, width int) int

// TypeUI bundles the rendering and editing primitives for a semantic type.
type TypeUI struct {
	Render     FieldRenderer
	Edit       FieldEditor
	HeightFn   FieldHeightFn
	Capability EditorCapability
}

// FieldDescriptor describes a single configurable detail-view field.
type FieldDescriptor struct {
	Name            string
	Label           string
	Semantic        SemanticType
	EditField       model.EditField
	Get             func(tk *tikipkg.Tiki) any
	Set             func(tk *tikipkg.Tiki, v any) error
	ReadOnly        bool
	EditTraversable bool
}

var fieldRegistry = map[string]FieldDescriptor{}
var typeRegistry = map[SemanticType]TypeUI{}

func init() {
	registerBuiltinTypes()
	registerBuiltinFields()
	publishRenderableFields()
}

// publishRenderableFields tells the plugin loader which metadata field names
// the detail view can actually render.
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

// LookupType returns the TypeUI for a semantic type.
func LookupType(t SemanticType) (TypeUI, bool) {
	ui, ok := typeRegistry[t]
	return ui, ok
}

// FieldHeight resolves descriptor → type → HeightFn for a field. Single-row
// types and unknown fields return 1, ensuring an empty list-field still
// reserves a row for its "(none)" placeholder.
func FieldHeight(name string, tk *tikipkg.Tiki, width int) int {
	fd, ok := LookupField(name)
	if !ok {
		return 1
	}
	ui, ok := LookupType(fd.Semantic)
	if !ok || ui.HeightFn == nil {
		return 1
	}
	return ui.HeightFn(tk, width)
}

// registerBuiltinFields wires the schema-known fields into the registry.
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
		EditField:       model.EditFieldTags,
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
	fieldRegistry["createdBy"] = FieldDescriptor{
		Name:     "createdBy",
		Label:    "Author",
		Semantic: SemanticText,
		Get:      func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField("createdBy"); return v },
		ReadOnly: true,
	}
	fieldRegistry["createdAt"] = FieldDescriptor{
		Name:     "createdAt",
		Label:    "Created",
		Semantic: SemanticDateTime,
		Get:      func(tk *tikipkg.Tiki) any { return tk.CreatedAt },
		ReadOnly: true,
	}
	fieldRegistry["updatedAt"] = FieldDescriptor{
		Name:     "updatedAt",
		Label:    "Updated",
		Semantic: SemanticDateTime,
		Get:      func(tk *tikipkg.Tiki) any { return tk.UpdatedAt },
		ReadOnly: true,
	}
}

func registerBuiltinTypes() {
	typeRegistry[SemanticStatus] = TypeUI{
		Render:     renderStatusValue,
		Edit:       editStatusValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticType_] = TypeUI{
		Render:     renderTypeValue,
		Edit:       editTypeValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticPriority] = TypeUI{
		Render:     renderPriorityValue,
		Edit:       editPriorityValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticText] = TypeUI{
		Render:     renderTextValue,
		Edit:       editAssigneeValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticInteger] = TypeUI{
		Render:     renderIntegerValue,
		Edit:       editPointsValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticBoolean] = TypeUI{
		Render:     renderBooleanValue,
		HeightFn:   singleRowHeight,
		Capability: EditorStub,
	}
	typeRegistry[SemanticDate] = TypeUI{
		Render:     renderDateValue,
		Edit:       editDueValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticDateTime] = TypeUI{
		Render:     renderDateTimeValue,
		HeightFn:   singleRowHeight,
		Capability: EditorStub,
	}
	typeRegistry[SemanticRecurrence] = TypeUI{
		Render:     renderRecurrenceValue,
		Edit:       editRecurrenceValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticEnum] = TypeUI{
		Render:     renderEnumValue,
		HeightFn:   singleRowHeight,
		Capability: EditorStub,
	}
	typeRegistry[SemanticStringList] = TypeUI{
		Render:     renderStringListValue,
		Edit:       editTagsValue,
		HeightFn:   stringListHeight,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticTaskIDList] = TypeUI{
		Render:     renderTaskIDListValue,
		HeightFn:   taskIDListHeight,
		Capability: EditorStub,
	}
}

// singleRowHeight is the HeightFn for fixed one-line fields.
func singleRowHeight(_ *tikipkg.Tiki, _ int) int { return 1 }

// stringListHeight uses WordList wrap to compute the wrapped row count.
// The +2 accounts for the BorderPadding(0,0,1,1) on the column container
// (RenderTagsColumn). Returns 1 for empty tags so the (none) placeholder
// still gets a row.
func stringListHeight(tk *tikipkg.Tiki, width int) int {
	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	if len(tags) == 0 {
		return 1
	}
	inner := width - 2
	if inner < 1 {
		inner = 1
	}
	wrapped := component.NewWordList(tags).WrapWords(inner)
	if len(wrapped) == 0 {
		return 1
	}
	return 1 + len(wrapped)
}

// taskIDListHeight returns 1 + min(len(deps), TaskListMetadataMaxRows).
// Counts every declared dependency (resolved or not) because the renderer
// emits one row per id even when unresolved (placeholder display).
func taskIDListHeight(tk *tikipkg.Tiki, _ int) int {
	deps, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn)
	if len(deps) == 0 {
		return 1
	}
	depRows := len(deps)
	if depRows > config.TaskListMetadataMaxRows {
		depRows = config.TaskListMetadataMaxRows
	}
	return 1 + depRows
}

// renderConfiguredField looks up the field descriptor and routes through the
// type registry to produce a primitive.
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

// withFieldDescriptor stamps the descriptor's name onto the context so generic
// renderers can resolve their target field.
func withFieldDescriptor(ctx FieldRenderContext, fd FieldDescriptor) FieldRenderContext {
	ctx.FieldName = fd.Name
	return ctx
}

// placeholderRow produces a single-line text view used for unknown/stub
// renderers so the layout still allocates a row.
func placeholderRow(text string) tview.Primitive {
	tv := tview.NewTextView().SetDynamicColors(true).SetText(text)
	tv.SetBorderPadding(0, 0, 0, 0)
	return tv
}

// labeledLine returns a "Label: value" row using the dim label / value colors.
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
	fd, ok := LookupField(ctx.FieldName)
	if !ok {
		return placeholderRow(fmt.Sprintf("%s: (unknown)", ctx.FieldName))
	}
	value, _, _ := tk.StringField(fd.Name)
	if value == "" {
		value = textEmptyPlaceholder(fd.Name)
	}
	return labeledLine(fd.Label, value, ctx.Colors)
}

func renderIntegerValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, ok := LookupField(ctx.FieldName)
	if !ok {
		return placeholderRow(fmt.Sprintf("%s: (unknown)", ctx.FieldName))
	}
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
	fd, ok := LookupField(ctx.FieldName)
	if !ok {
		return placeholderRow(fmt.Sprintf("%s: (unknown)", ctx.FieldName))
	}
	t, _, _ := tk.TimeField(fd.Name)
	value := "None"
	if !t.IsZero() {
		value = t.Format("2006-01-02")
	}
	return labeledLine(fd.Label, value, ctx.Colors)
}

func renderDateTimeValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, ok := LookupField(ctx.FieldName)
	if !ok {
		return placeholderRow(fmt.Sprintf("%s: (unknown)", ctx.FieldName))
	}
	value := "Unknown"
	if fd.Get != nil {
		if t, ok := fd.Get(tk).(time.Time); ok && !t.IsZero() {
			value = t.Format("2006-01-02 15:04")
		}
	}
	return labeledLine(fd.Label, value, ctx.Colors)
}

// textEmptyPlaceholder returns the historical empty-value placeholder for
// well-known fields.
func textEmptyPlaceholder(name string) string {
	switch name {
	case tikipkg.FieldAssignee:
		return "Unassigned"
	case "createdBy":
		return "Unknown"
	default:
		return "─"
	}
}

func renderRecurrenceValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	recurrenceStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
	display := taskpkg.RecurrenceDisplay(taskpkg.Recurrence(recurrenceStr))
	return labeledLine("Recurrence", display, ctx.Colors)
}

func renderEnumValue(_ *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	return labeledLine("Enum", "(stub)", ctx.Colors)
}

// renderStringListValue renders the tags column. Non-empty → multi-row
// RenderTagsColumn; empty → single labeledLine "(none)" so the grid's
// height contract (always ≥ 1 row) holds.
func renderStringListValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	if len(tags) == 0 {
		return labeledLine("Tags", "(none)", ctx.Colors)
	}
	return RenderTagsColumn(tk)
}

// renderTaskIDListValue renders the depends-on column. Non-empty → multi-row
// RenderDependsOnColumn (which now emits placeholder rows for unresolved
// IDs so its row count matches taskIDListHeight); empty → labeledLine.
func renderTaskIDListValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	deps, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn)
	if len(deps) == 0 {
		return labeledLine("Depends On", "(none)", ctx.Colors)
	}
	if ctx.Store == nil {
		return labeledLine("Depends On", strings.Join(deps, ", "), ctx.Colors)
	}
	if col := RenderDependsOnColumn(tk, ctx.Store); col != nil {
		return col
	}
	return labeledLine("Depends On", strings.Join(deps, ", "), ctx.Colors)
}

// --- editor factories ---

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
	return &selectListAdapter{EditSelectList: editor}
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
	return &selectListAdapter{EditSelectList: editor}
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
	return &selectListAdapter{EditSelectList: editor}
}

// editAssigneeValue builds the assignee editor (free-text + suggestions).
func editAssigneeValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	var options []string
	if ctx.Store != nil {
		if users, err := ctx.Store.GetAllUsers(); err == nil {
			options = append(options, users...)
		}
	}
	if len(options) == 0 {
		options = []string{"Unassigned"}
	}
	assignee, _, _ := tk.StringField(tikipkg.FieldAssignee)
	if assignee == "" {
		assignee = "Unassigned"
	}
	editor := component.NewEditSelectList(options, true)
	editor.SetLabel(getFocusMarker(ctx.Colors) + "Assignee: ")
	editor.SetInitialValue(assignee)
	editor.SetSubmitHandler(func(text string) {
		if onChange != nil {
			onChange(text)
		}
	})
	return &selectListAdapter{EditSelectList: editor}
}

// editPointsValue builds the points editor with Up/Down arrow cycling.
// IntEditSelect.GetValue() returns int; the adapter formats it as decimal
// for the registry's string-shaped onChange contract.
func editPointsValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	points, _, _ := tk.IntField(tikipkg.FieldPoints)
	editor := component.NewIntEditSelect(1, config.GetMaxPoints(), false)
	editor.SetLabel(getFocusMarker(ctx.Colors) + "Points:   ")
	editor.SetChangeHandler(func(v int) {
		if onChange != nil {
			onChange(strconv.Itoa(v))
		}
	})
	editor.SetValue(points)
	return &intEditAdapter{IntEditSelect: editor}
}

// editDueValue builds the date editor. The widget's onChange fires with
// the validated formatted string after each accepted change.
func editDueValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	due, _, _ := tk.TimeField(tikipkg.FieldDue)
	editor := component.NewDateEdit()
	editor.SetLabel(getFocusMarker(ctx.Colors) + "Due:      ")
	editor.SetChangeHandler(func(s string) {
		if onChange != nil {
			onChange(s)
		}
	})
	var initial string
	if !due.IsZero() {
		initial = due.Format(taskpkg.DateFormat)
	}
	editor.SetInitialValue(initial)
	a := &dateEditAdapter{DateEdit: editor}
	// Read-only when the tiki has a non-empty recurrence: due is auto-computed.
	recurrenceStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
	if recurrenceStr != "" && taskpkg.Recurrence(recurrenceStr) != taskpkg.RecurrenceNone {
		a.readOnly = true
	}
	return a
}

// editRecurrenceValue builds the recurrence editor. RecurrenceEdit.GetValue()
// assembles a canonical cron expression from the freq/value parts; the
// adapter exposes that as GetText() so the registry boundary stays uniform.
func editRecurrenceValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	recurrenceStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
	editor := component.NewRecurrenceEdit()
	editor.SetLabel(getFocusMarker(ctx.Colors) + "Recurrence: ")
	editor.SetChangeHandler(func(_ string) {
		if onChange != nil {
			onChange(editor.GetValue())
		}
	})
	editor.SetInitialValue(recurrenceStr)
	return &recurrenceEditAdapter{RecurrenceEdit: editor}
}

// editTagsValue builds the tags textarea editor. Tags are whitespace-joined
// for transport so a single string round-trips through onChange/GetText.
func editTagsValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	textArea := tview.NewTextArea()
	textArea.SetBorder(false)
	textArea.SetBorderPadding(0, 0, 1, 1)
	textArea.SetPlaceholder("space-separated tags")
	textArea.SetPlaceholderStyle(tcell.StyleDefault.Foreground(ctx.Colors.TaskDetailPlaceholderColor.TCell()))

	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	textArea.SetText(strings.Join(tags, " "), false)

	a := &tagsEditAdapter{TextArea: textArea, onChange: onChange}
	textArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlS {
			if a.onChange != nil {
				a.onChange(a.GetText())
			}
			return nil
		}
		return event
	})
	return a
}

// buildFieldEditor is a convenience that looks up the type registry and
// returns the editor widget if the type supports editing.
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
// implemented editor.
func FieldHasEditor(name string) bool {
	fd, ok := LookupField(name)
	if !ok {
		return false
	}
	if fd.ReadOnly {
		return false
	}
	ui, ok := LookupType(fd.Semantic)
	if !ok {
		return false
	}
	return ui.Capability == EditorImplemented && ui.Edit != nil
}

// --- adapter widgets ---
//
// Each adapter wraps an existing component to satisfy FieldEditorWidget
// (specifically, to add CycleValue) without modifying the component itself.
// Components are shared across the codebase; widening their public APIs to
// support the in-grid editor protocol would force ripple changes elsewhere.

// selectListAdapter delegates CycleValue to MoveToNext/MoveToPrevious.
type selectListAdapter struct {
	*component.EditSelectList
}

func (a *selectListAdapter) CycleValue(direction int) bool {
	if direction >= 0 {
		a.MoveToNext()
	} else {
		a.MoveToPrevious()
	}
	return true
}

// intEditAdapter delegates CycleValue to the widget's InputHandler with
// synthesized Up/Down events — exactly how task_edit_nav.go drove it before
// the migration.
type intEditAdapter struct {
	*component.IntEditSelect
}

func (a *intEditAdapter) CycleValue(direction int) bool {
	key := tcell.KeyDown
	if direction < 0 {
		key = tcell.KeyUp
	}
	a.InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), nil)
	return true
}

// GetText conforms to the FieldEditorWidget contract — IntEditSelect's
// natural type is int; format it as decimal for the registry boundary.
func (a *intEditAdapter) GetText() string {
	return strconv.Itoa(a.GetValue())
}

// dateEditAdapter handles read-only suppression: when recurrence is set on
// the underlying tiki, due is auto-computed and CycleValue refuses to
// advance the date.
type dateEditAdapter struct {
	*component.DateEdit
	readOnly bool
}

func (a *dateEditAdapter) CycleValue(direction int) bool {
	if a.readOnly {
		return false
	}
	key := tcell.KeyDown
	if direction < 0 {
		key = tcell.KeyUp
	}
	a.InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), nil)
	return true
}

// GetText returns the validated formatted date.
func (a *dateEditAdapter) GetText() string {
	return a.GetCurrentText()
}

// recurrenceEditAdapter delegates CycleValue to CyclePrev/CycleNext.
type recurrenceEditAdapter struct {
	*component.RecurrenceEdit
}

func (a *recurrenceEditAdapter) CycleValue(direction int) bool {
	if direction >= 0 {
		a.CycleNext()
	} else {
		a.CyclePrev()
	}
	return true
}

// GetText returns the assembled cron expression.
func (a *recurrenceEditAdapter) GetText() string {
	return a.GetValue()
}

// tagsEditAdapter wraps tview.TextArea — non-cyclable, so CycleValue
// always returns false.
type tagsEditAdapter struct {
	*tview.TextArea
	onChange func(string)
}

func (a *tagsEditAdapter) CycleValue(int) bool { return false }

// GetText returns the textarea content (whitespace-joined tags).
func (a *tagsEditAdapter) GetText() string {
	return a.TextArea.GetText()
}
