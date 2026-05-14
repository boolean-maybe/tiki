package taskdetail

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
	"github.com/boolean-maybe/tiki/workflow/value"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SemanticType identifies how a configurable detail field is rendered and
// edited. The registry routes a field by its semantic type to the matching
// renderer/editor factory; immutable types like ID/Author are handled by the
// title block, not by this registry.
type SemanticType string

const (
	// SemanticEnum is the unified renderer/editor for any TypeEnum field —
	// status, type, priority, and any user-declared enums in workflow.yaml
	// all route through it. The dedicated SemanticStatus/SemanticType_/
	// SemanticPriority constants were removed once the generic editor proved
	// equivalent: each had been a copy of the same select-list with a
	// different hardcoded label.
	SemanticEnum       SemanticType = "enum"
	SemanticText       SemanticType = "text"
	SemanticInteger    SemanticType = "integer"
	SemanticBoolean    SemanticType = "boolean"
	SemanticDate       SemanticType = "date"
	SemanticDateTime   SemanticType = "datetime"
	SemanticRecurrence SemanticType = "recurrence"
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

// registerBuiltinFields wires the workflow-declared fields into the registry.
func registerBuiltinFields() {
	fieldRegistry[tikipkg.FieldStatus] = FieldDescriptor{
		Name:            tikipkg.FieldStatus,
		Label:           "Status",
		Semantic:        SemanticEnum,
		EditField:       model.EditFieldStatus,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField(tikipkg.FieldStatus); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldType] = FieldDescriptor{
		Name:            tikipkg.FieldType,
		Label:           "Type",
		Semantic:        SemanticEnum,
		EditField:       model.EditFieldType,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField(tikipkg.FieldType); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldPriority] = FieldDescriptor{
		Name:            tikipkg.FieldPriority,
		Label:           "Priority",
		Semantic:        SemanticEnum,
		EditField:       model.EditFieldPriority,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField(tikipkg.FieldPriority); return v },
		EditTraversable: true,
	}
	fieldRegistry[tikipkg.FieldPoints] = FieldDescriptor{
		Name:            tikipkg.FieldPoints,
		Label:           "Points",
		Semantic:        SemanticEnum,
		EditField:       model.EditFieldPoints,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField(tikipkg.FieldPoints); return v },
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
	typeRegistry[SemanticText] = TypeUI{
		Render:     renderTextValue,
		Edit:       editAssigneeValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
	}
	typeRegistry[SemanticInteger] = TypeUI{
		Render:     renderIntegerValue,
		HeightFn:   singleRowHeight,
		Capability: EditorStub,
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
		Edit:       editEnumValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
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
// type registry to produce a primitive. Fields that exist in the workflow
// catalog but not in the typed registry fall back to a generic catalog-
// driven row that reads the value verbatim from the tiki's Fields map.
//
// Workflow-declared TypeEnum fields are routed to the SemanticEnum renderer
// even when they don't have a built-in FieldDescriptor — so user-declared
// enums (e.g. severity in bug-tracker.yaml) get the same display-with-emoji
// rendering and focus-aware coloring as the canonical status/type/priority.
func renderConfiguredField(name string, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if fd, ok := LookupField(name); ok {
		ui, ok := LookupType(fd.Semantic)
		if !ok || ui.Render == nil {
			return placeholderRow(fmt.Sprintf("%s: (no renderer)", fd.Label))
		}
		return ui.Render(tk, withFieldDescriptor(ctx, fd))
	}
	if wfd, ok := workflow.Field(name); ok {
		if wfd.Type == workflow.TypeEnum {
			ctx.FieldName = wfd.Name
			return renderEnumValue(tk, ctx)
		}
		return renderGenericWorkflowField(wfd, tk, ctx)
	}
	return placeholderRow(fmt.Sprintf("%s: (unknown field)", name))
}

// renderGenericWorkflowField produces a labeled row for a workflow-declared
// field that the typed registry doesn't have a custom renderer for. The
// label defaults to the field name, and the value is formatted by type.
func renderGenericWorkflowField(fd workflow.FieldDef, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	label := fd.Name
	value := genericFieldValueString(fd, tk)
	return labeledLine(label, value, ctx.Colors)
}

// genericFieldValueString formats a workflow field's value as a single-line
// string, dispatching on declared type. Empty/absent values render as a dash.
// User-controlled string values are escaped against tview's dynamic-color
// markup; values that come from a controlled source (enum labels, formatted
// times, parsed numbers) are passed through verbatim.
func genericFieldValueString(fd workflow.FieldDef, tk *tikipkg.Tiki) string {
	raw, ok := tk.Get(fd.Name)
	if !ok {
		return "—"
	}
	switch fd.Type {
	case workflow.TypeListString, workflow.TypeListRef:
		ss, _, _ := tk.StringSliceField(fd.Name)
		if len(ss) == 0 {
			return "—"
		}
		escaped := make([]string, len(ss))
		for i, s := range ss {
			escaped[i] = tview.Escape(s)
		}
		return strings.Join(escaped, ", ")
	case workflow.TypeBool:
		if b, ok := raw.(bool); ok {
			if b {
				return "true"
			}
			return "false"
		}
	case workflow.TypeEnum:
		if s, ok := raw.(string); ok && s != "" {
			return fd.EnumDisplay(s)
		}
		return "—"
	case workflow.TypeDate:
		// Date-typed fields stay date-only (YYYY-MM-DD) regardless of
		// whether the raw value is a time.Time with non-zero clock —
		// the type contract is that date fields don't carry a clock.
		if t, ok := raw.(time.Time); ok {
			if t.IsZero() {
				return "—"
			}
			return t.Format("2006-01-02")
		}
	case workflow.TypeTimestamp:
		// Timestamp-typed fields preserve the time component — using the
		// date-only format would silently drop hours/minutes from
		// dueBy/createdAt/updatedAt-style fields, which are visibly
		// truncated to "2026-05-08" instead of "2026-05-08 14:30".
		if t, ok := raw.(time.Time); ok {
			if t.IsZero() {
				return "—"
			}
			return t.Format("2006-01-02 15:04")
		}
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return "—"
		}
		return tview.Escape(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case time.Time:
		// Untyped time.Time falls back to date-only — historical default.
		// TypeDate / TypeTimestamp are handled by the typed switch above
		// before reaching this branch.
		if v.IsZero() {
			return "—"
		}
		return v.Format("2006-01-02")
	default:
		// User-controlled YAML can land arbitrarily-shaped values here
		// (lists, maps with embedded strings, etc.). Escape against
		// tview's dynamic-color markup so a value like "[red]hi" can't
		// hijack the row's coloring once it lands in labeledLine.
		return tview.Escape(fmt.Sprintf("%v", v))
	}
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

func renderTextValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, ok := LookupField(ctx.FieldName)
	if !ok {
		return placeholderRow(fmt.Sprintf("%s: (unknown)", ctx.FieldName))
	}
	value, _, _ := tk.StringField(fd.Name)
	if value == "" {
		return labeledLine(fd.Label, textEmptyPlaceholder(fd.Name), ctx.Colors)
	}
	// labeledLine emits the value into a SetDynamicColors(true) TextView,
	// so user-controlled text containing tview color tags (e.g. "[red]")
	// would be parsed as markup. Escape user content; the empty-placeholder
	// path above is internal and safe.
	return labeledLine(fd.Label, tview.Escape(value), ctx.Colors)
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
	display := value.RecurrenceDisplay(value.Recurrence(recurrenceStr))
	return labeledLine("Recurrence", display, ctx.Colors)
}

// renderEnumValue is the generic read-only renderer for any TypeEnum field.
// It replaces the per-field renderStatus/renderType/renderPriority helpers:
// look up the workflow descriptor, format the current value via EnumDisplay,
// and apply the same focus-aware dim/full color treatment as the legacy
// renderers so the visual contract is preserved when the focused field is
// being shown read-only (i.e. edit mode but not the focused row).
//
// Works for both built-in fields (status/type/priority — which have a
// FieldDescriptor in the static registry) and workflow-declared custom
// enums (severity, environment, etc. — which only exist in the workflow
// catalog). Static descriptor wins for label/EditField identity; falls
// back to the field name and EditFieldNone for catalog-only fields.
func renderEnumValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, hasFD := LookupField(ctx.FieldName)
	wfd, hasWFD := workflow.Field(ctx.FieldName)
	if !hasFD && !hasWFD {
		return placeholderRow(fmt.Sprintf("%s: (unknown)", ctx.FieldName))
	}
	name := ctx.FieldName
	label := name
	var editField model.EditField // zero value = unset, never matches a real field
	if hasFD {
		label = fd.Label
		editField = fd.EditField
	}

	value, _, _ := tk.StringField(name)
	display := "─"
	if value != "" {
		// Enum labels (and the raw value fallback) are user-controlled
		// via workflow.yaml. Escape against tview color-tag markup
		// before interpolating into the SetDynamicColors TextView so a
		// label like "[red]High[-]" renders literally instead of
		// hijacking the row's coloring.
		if hasWFD {
			display = tview.Escape(wfd.EnumDisplay(value))
		} else {
			display = tview.Escape(value)
		}
	} else if name == tikipkg.FieldType {
		// preserve legacy "(none)" placeholder color for missing type
		display = ctx.Colors.TaskDetailPlaceholderColor.Tag().String() + "(none)[-]"
	}

	focused := ctx.Mode == RenderModeEdit && editField != "" && ctx.FocusedField == editField
	labelTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor).Tag().String()
	valueTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor).Tag().String()
	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelTag, label+":", valueTag, display)
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)
	return textView
}

// editEnumValue is the generic in-place editor for any TypeEnum field. It
// builds a select-list of display strings (Label + Emoji) from the workflow's
// AllowedValues, initializes to the current value's display, and emits the
// canonical key (not the display string) on submit so downstream save handlers
// don't need to round-trip display↔key conversion.
//
// Works for both built-in fields (status/type/priority — registered in the
// static field registry) and workflow-declared custom enums (severity,
// environment, etc. — present only in the workflow catalog). The workflow
// FieldDef is the authoritative source for allowed values and display
// formatting in either case.
func editEnumValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	wfd, ok := workflow.Field(ctx.FieldName)
	if !ok || wfd.Type != workflow.TypeEnum {
		return nil
	}
	label := ctx.FieldName
	if fd, hasFD := LookupField(ctx.FieldName); hasFD {
		label = fd.Label
	}

	keys := wfd.AllowedValues()
	displays := make([]string, len(keys))
	for i, k := range keys {
		displays[i] = wfd.EnumDisplay(k)
	}
	currentKey, _, _ := tk.StringField(ctx.FieldName)
	currentDisplay := wfd.EnumDisplay(currentKey)

	editor := component.NewEditSelectList(displays, false)
	editor.SetLabel(getFocusMarker(ctx.Colors) + fmt.Sprintf("%-10s", label+":"))
	editor.SetInitialValue(currentDisplay)
	editor.SetSubmitHandler(func(text string) {
		if onChange == nil {
			return
		}
		if key, ok := wfd.EnumParseDisplay(text); ok {
			onChange(key)
			return
		}
		// fallback: emit raw text so unknown values surface at the save
		// boundary rather than being silently dropped.
		onChange(text)
	})
	return &enumSelectAdapter{
		selectListAdapter: selectListAdapter{EditSelectList: editor},
		field:             wfd,
	}
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
		initial = due.Format(value.DateFormat)
	}
	editor.SetInitialValue(initial)
	a := &dateEditAdapter{DateEdit: editor}
	// Read-only when the tiki has a non-empty recurrence: due is auto-computed.
	recurrenceStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
	if recurrenceStr != "" && value.Recurrence(recurrenceStr) != value.RecurrenceNone {
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
// returns the editor widget if the type supports editing. The ctx is
// stamped with the field descriptor before invoking the editor factory so
// generic factories (notably the SemanticEnum editor) can resolve their
// target field via ctx.FieldName.
//
// Workflow-declared TypeEnum fields without a static FieldDescriptor go
// through the SemanticEnum editor directly — same UX as the built-in
// status/type/priority editors, but driven entirely by workflow.yaml.
func buildFieldEditor(name string, tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	if fd, ok := LookupField(name); ok {
		ui, ok := LookupType(fd.Semantic)
		if !ok || ui.Capability != EditorImplemented || ui.Edit == nil {
			return nil
		}
		return ui.Edit(tk, withFieldDescriptor(ctx, fd), onChange)
	}
	if wfd, ok := workflow.Field(name); ok && wfd.Type == workflow.TypeEnum {
		ctx.FieldName = name
		return editEnumValue(tk, ctx, onChange)
	}
	return nil
}

// FieldHasEditor reports whether the named field has a registered, fully
// implemented editor. Returns true for both built-in editable fields and
// workflow-declared custom enum fields.
func FieldHasEditor(name string) bool {
	if fd, ok := LookupField(name); ok {
		if fd.ReadOnly {
			return false
		}
		ui, ok := LookupType(fd.Semantic)
		if !ok {
			return false
		}
		return ui.Capability == EditorImplemented && ui.Edit != nil
	}
	if wfd, ok := workflow.Field(name); ok && wfd.Type == workflow.TypeEnum {
		return true
	}
	return false
}

// --- adapter widgets ---
//
// Each adapter wraps an existing component to satisfy FieldEditorWidget
// (specifically, to add CycleValue) without modifying the component itself.
// Components are shared across the codebase; widening their public APIs to
// support the in-grid editor protocol would force ripple changes elsewhere.

// enumSelectAdapter wraps the generic select-list adapter for SemanticEnum
// editors. The crucial difference: GetText() returns the *canonical key*
// instead of the underlying input field's display string ("High 🔴"). The
// flush path (FlushFocusedEditor) calls GetText() to produce a value for
// the save handler — without this typed adapter, the handler would receive
// the display string and the save would fail enum-key validation. Mirrors
// how intEditAdapter.GetText() returns an int formatted as decimal rather
// than whatever the underlying widget shows.
type enumSelectAdapter struct {
	selectListAdapter
	field workflow.FieldDef
}

func (a *enumSelectAdapter) GetText() string {
	display := a.selectListAdapter.GetText()
	if key, ok := a.field.EnumParseDisplay(display); ok {
		return key
	}
	// Unknown display — return as-is so the save handler can surface the
	// validation error rather than the call silently no-op'ing.
	return display
}

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
