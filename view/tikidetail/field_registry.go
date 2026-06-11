package tikidetail

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/boolean-maybe/ruki/recurrence"
	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/view/gridbox"
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
	SemanticTikiIDList SemanticType = "tiki_id_list"
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
// content rows. The grid solver clamps the result against the anchor's
// declared row span.
type FieldHeightFn func(tk *tikipkg.Tiki, width int) int

// FieldEmptyFn reports whether tk holds no value for the named field. It is the
// typed emptiness predicate (date→IsZero, list→len==0, string→==""), keyed off
// the semantic type — NOT a formatted string. A nil FieldEmptyFn means the type
// is never empty (it always renders a concrete value, e.g. recurrence→"None"),
// so such fields are never `?`-hidden.
type FieldEmptyFn func(tk *tikipkg.Tiki, name string) bool

// TypeUI bundles the rendering and editing primitives for a semantic type.
// IsEmpty and EmptyPlaceholder are the per-type halves of the empty-value
// contract: IsEmpty answers "has no value", EmptyPlaceholder is the default
// render string for that empty state (a per-field FieldDescriptor.EmptyPlaceholder
// overrides it). Both feed emptyPlaceholder / fieldIsEmpty so renderers and the
// width measure agree on what an empty cell draws.
type TypeUI struct {
	Render           FieldRenderer
	Edit             FieldEditor
	HeightFn         FieldHeightFn
	Capability       EditorCapability
	IsEmpty          FieldEmptyFn
	EmptyPlaceholder string
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
	// EmptyPlaceholder is the text shown when the field has no value. Empty
	// string means the default "─". Enum renderers treat it as a marker that
	// is styled with the muted role at render time (see renderEnumValue);
	// text renderers emit it verbatim. This replaces the per-field-name
	// switches that previously special-cased assignee/createdBy/type.
	EmptyPlaceholder string
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

// MeasureFieldValue returns the visible content width (in cells) of a field's
// rendered value for tk, used by the grid solver to size `auto` columns. It
// reuses genericFieldValueString — the same value-formatting path the view
// renders — so the measured width matches what is drawn. Color/markup tags do
// not count toward the width. Unknown fields measure 0 (the solver floors at 1).
//
// The two list types measure differently because they render differently:
//   - stringList (e.g. tags) renders as a word-wrapping column, so its useful
//     width is the longest single token — the comma-joined length would
//     massively over-reserve and wrongly squeeze neighbours.
//   - tikiIdList (e.g. dependsOn) renders one non-wrapping "ID title" row per
//     id, so its useful width is the widest such row. Measuring only the id
//     token here under-reserved the column and the rendered title clipped
//     against the box frame — the measure must see what tikiIDListColumn draws.
//
// The list-value columns carry no padding (their first cell aligns with the
// `.caption` cell), so the measure is the bare content width.
func MeasureFieldValue(name string, tk *tikipkg.Tiki, ctx FieldRenderContext) int {
	fd, ok := workflow.Field(name)
	if !ok {
		return 0
	}
	// a `.count` cell renders the integer item count, not the list contents, so
	// it measures the digit width — checked before the list cases, which would
	// otherwise over-reserve the column to the longest token / widest id row.
	if ctx.Display == gridlayout.DisplayCount {
		return len(listFieldCountText(name, tk))
	}
	switch fd.Type {
	case workflow.TypeListString:
		return measureStringListField(name, tk)
	case workflow.TypeListRef:
		return measureTikiIDListField(name, tk, ctx.Store)
	}
	// an empty scalar renders its placeholder ("None"/"Unknown"/"─"/…), not the
	// "—" sentinel genericFieldValueString emits — measure the SAME string the
	// renderer draws so the solver reserves the cell's true width.
	if fieldIsEmpty(tk, fd) {
		return tview.TaggedStringWidth(emptyPlaceholder(name, semanticForValueType(fd.Type)))
	}
	return tview.TaggedStringWidth(genericFieldValueString(fd, tk, ctx))
}

// measureStringListField returns the longest single token width across a string
// list field's values, mirroring the wordListColumn renderer (which wraps per
// word). Returns at least 1 so an empty/placeholder list still reserves a cell.
func measureStringListField(name string, tk *tikipkg.Tiki) int {
	vals, _, _ := tk.StringSliceField(name)
	maxWord := 1
	for _, v := range vals {
		if w := tview.TaggedStringWidth(v); w > maxWord {
			maxWord = w
		}
	}
	return maxWord
}

// measureTikiIDListField returns the width of the widest "ID title" row a
// tikiIdList field renders via tikiIDListColumn, so the solver reserves enough
// room for the resolved titles instead of clipping them. Each id contributes
// idColumnWidth + 1 + titleWidth (resolved title, or "(unresolved)" when the id
// is not in the store, or the bare id when no store is available — mirroring
// the renderer). Returns at least 1 for an empty list.
func measureTikiIDListField(name string, tk *tikipkg.Tiki, tikiStore store.Store) int {
	ids, _, _ := tk.StringSliceField(name)
	if len(ids) == 0 {
		return 1
	}
	idColumnWidth := 0
	for _, id := range ids {
		if w := len([]rune(id)); w > idColumnWidth {
			idColumnWidth = w
		}
	}
	widest := 1
	for _, id := range ids {
		title := ""
		switch {
		case tikiStore == nil:
			// no store: renderer shows the bare id only.
		default:
			if dep := tikiStore.GetTiki(id); dep != nil {
				title = dep.Title()
			} else {
				title = "(unresolved)"
			}
		}
		row := idColumnWidth // padded id column
		if title != "" {
			row += 1 + len([]rune(title)) // single space + title
		}
		if row > widest {
			widest = row
		}
	}
	return widest
}

// MeasureAnchor is the content-measure callback both grid surfaces (detail box
// and tiki card) pass to the solver. Literal/composite anchors defer to the
// text-based measurer. A `.caption` field anchor renders its caption text, so
// it is measured by that text — not the field value. Other field anchors
// measure their rendered value; the anchor's Display mode is threaded in so an
// enum rendered as `.visual` measures its glyph, not its label.
func MeasureAnchor(a gridlayout.Anchor, tk *tikipkg.Tiki, ctx FieldRenderContext) int {
	switch a.Kind {
	case gridlayout.AnchorComposite:
		// composites render a concatenated string of segment values/literals.
		// a row-spanned composite word-wraps within its rows, so it only needs
		// room for its longest word — measuring the full single line would make
		// the solver shed a prose blurb that would otherwise wrap to fit (see
		// the Project detail view). Single-row composites measure the full line.
		if a.RowSpan > 1 {
			return gridbox.LongestWordWidth(compositePlainText(a, tk, ctx))
		}
		return tview.TaggedStringWidth(buildCompositeText(a, tk, ctx))
	case gridlayout.AnchorLiteral:
		return gridbox.MeasureAnchorText(a)
	}
	if a.Display == gridlayout.DisplayCaption {
		return len([]rune(fieldCaptionText(a.Name))) + 1
	}
	ctx.Display = a.Display
	return MeasureFieldValue(a.Name, tk, ctx)
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
		// muted styling is applied at render time from ctx.Roles (renderEnumValue);
		// the descriptor holds only the literal marker so registration stays
		// theme-independent (registerBuiltinFields runs in init(), before SetTheme).
		EmptyPlaceholder: "(none)",
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
		Name:             tikipkg.FieldAssignee,
		Label:            "Assignee",
		Semantic:         SemanticText,
		EditField:        model.EditFieldAssignee,
		Get:              func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField(tikipkg.FieldAssignee); return v },
		EditTraversable:  true,
		EmptyPlaceholder: "Unassigned",
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
		Semantic:        SemanticTikiIDList,
		Get:             func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn); return v },
		EditTraversable: true,
	}
	fieldRegistry["createdBy"] = FieldDescriptor{
		Name:             "createdBy",
		Label:            "Author",
		Semantic:         SemanticText,
		Get:              func(tk *tikipkg.Tiki) any { v, _, _ := tk.StringField("createdBy"); return v },
		ReadOnly:         true,
		EmptyPlaceholder: "Unknown",
	}
	fieldRegistry["createdAt"] = FieldDescriptor{
		Name:     "createdAt",
		Label:    "Created",
		Semantic: SemanticDateTime,
		Get:      func(tk *tikipkg.Tiki) any { return tk.CreatedAt() },
		ReadOnly: true,
	}
	fieldRegistry["updatedAt"] = FieldDescriptor{
		Name:     "updatedAt",
		Label:    "Updated",
		Semantic: SemanticDateTime,
		Get:      func(tk *tikipkg.Tiki) any { return tk.UpdatedAt() },
		ReadOnly: true,
	}
	fieldRegistry["title"] = FieldDescriptor{
		Name:            "title",
		Label:           "Title",
		Semantic:        SemanticText,
		EditField:       model.EditFieldTitle,
		Get:             func(tk *tikipkg.Tiki) any { return tk.Title() },
		Set:             func(tk *tikipkg.Tiki, v any) error { s, _ := v.(string); tk.SetTitle(s); return nil },
		ReadOnly:        false,
		EditTraversable: true,
	}
}

func registerBuiltinTypes() {
	typeRegistry[SemanticText] = TypeUI{
		Render:     renderTextValue,
		Edit:       editAssigneeValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
		IsEmpty:    stringFieldEmpty,
		// no per-type default: the "─" fallback in emptyPlaceholder applies;
		// per-field overrides supply "Unassigned"/"Unknown".
	}
	typeRegistry[SemanticInteger] = TypeUI{
		Render:           renderIntegerValue,
		HeightFn:         singleRowHeight,
		Capability:       EditorStub,
		IsEmpty:          intFieldEmpty,
		EmptyPlaceholder: "─",
	}
	typeRegistry[SemanticBoolean] = TypeUI{
		Render:     renderBooleanValue,
		HeightFn:   singleRowHeight,
		Capability: EditorStub,
		// IsEmpty nil: boolean is a stub that always renders a concrete value.
	}
	typeRegistry[SemanticDate] = TypeUI{
		Render:           renderDateValue,
		Edit:             editDueValue,
		HeightFn:         singleRowHeight,
		Capability:       EditorImplemented,
		IsEmpty:          timeFieldEmpty,
		EmptyPlaceholder: "None",
	}
	typeRegistry[SemanticDateTime] = TypeUI{
		Render:           renderDateTimeValue,
		HeightFn:         singleRowHeight,
		Capability:       EditorStub,
		IsEmpty:          timeFieldEmpty,
		EmptyPlaceholder: "Unknown",
	}
	typeRegistry[SemanticRecurrence] = TypeUI{
		Render:     renderRecurrenceValue,
		Edit:       editRecurrenceValue,
		HeightFn:   singleRowHeight,
		Capability: EditorImplemented,
		// IsEmpty nil: an empty recurrence renders "None" via RecurrenceDisplay,
		// so it is never `?`-hidden.
	}
	typeRegistry[SemanticEnum] = TypeUI{
		Render:           renderEnumValue,
		Edit:             editEnumValue,
		HeightFn:         singleRowHeight,
		Capability:       EditorImplemented,
		IsEmpty:          stringFieldEmpty,
		EmptyPlaceholder: "─",
	}
	typeRegistry[SemanticStringList] = TypeUI{
		Render:           renderStringListValue,
		Edit:             editTagsValue,
		HeightFn:         stringListHeight,
		Capability:       EditorImplemented,
		IsEmpty:          listFieldEmpty,
		EmptyPlaceholder: "(none)",
	}
	typeRegistry[SemanticTikiIDList] = TypeUI{
		Render:           renderTikiIDListValue,
		HeightFn:         tikiIDListHeight,
		Capability:       EditorStub,
		IsEmpty:          listFieldEmpty,
		EmptyPlaceholder: "(none)",
	}
}

// typed emptiness predicates shared across semantic types. Each reads the
// underlying value via the tiki accessor — no string formatting, no sentinel.
func stringFieldEmpty(tk *tikipkg.Tiki, name string) bool {
	v, _, _ := tk.StringField(name)
	return v == ""
}
func timeFieldEmpty(tk *tikipkg.Tiki, name string) bool {
	t, _, _ := tk.TimeField(name)
	return t.IsZero()
}
func listFieldEmpty(tk *tikipkg.Tiki, name string) bool {
	v, _, _ := tk.StringSliceField(name)
	return len(v) == 0
}
func intFieldEmpty(tk *tikipkg.Tiki, name string) bool {
	_, present, _ := tk.IntField(name)
	return !present
}

// semanticForValueType bridges the workflow catalog's ValueType to the
// detail-view registry's SemanticType, so emptiness/placeholder traits resolve
// even for catalog-only fields that have no FieldDescriptor (e.g. user-declared
// enums). It generalizes the TypeEnum routing renderConfiguredField already
// does. The string family (TypeString/TypeID/TypeRef/TypeDuration) and any
// unmapped type fall through to SemanticText — string emptiness — matching
// genericFieldValueString's default branch.
func semanticForValueType(t workflow.ValueType) SemanticType {
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

// fieldIsEmpty reports whether tk holds no value for the workflow field, via the
// semantic type's typed IsEmpty predicate. Types whose TypeUI declares no
// IsEmpty (recurrence, boolean) are never empty. This is the single emptiness
// authority — fieldHasValue and the empty-cell width measure both call it,
// replacing the old genericFieldValueString(...) != "—" string compare.
func fieldIsEmpty(tk *tikipkg.Tiki, fd workflow.FieldDef) bool {
	ui, ok := LookupType(semanticForValueType(fd.Type))
	if !ok || ui.IsEmpty == nil {
		return false
	}
	return ui.IsEmpty(tk, fd.Name)
}

// emptyPlaceholder resolves the text drawn when a field is empty: the
// descriptor's per-field EmptyPlaceholder override, else the semantic type's
// per-type default, else "─". It is the single source of truth so renderers and
// MeasureFieldValue draw/measure identical bytes.
func emptyPlaceholder(name string, semantic SemanticType) string {
	if fd, ok := LookupField(name); ok && fd.EmptyPlaceholder != "" {
		return fd.EmptyPlaceholder
	}
	if ui, ok := LookupType(semantic); ok && ui.EmptyPlaceholder != "" {
		return ui.EmptyPlaceholder
	}
	return "─"
}

// singleRowHeight is the HeightFn for fixed one-line fields.
func singleRowHeight(_ *tikipkg.Tiki, _ int) int { return 1 }

// stringListHeight uses WordList wrap to compute the wrapped row count. The
// list-value column carries no padding, so the wrap width is the column width
// as-is. Returns 1 for empty tags so the "(none)" placeholder still gets a row.
func stringListHeight(tk *tikipkg.Tiki, width int) int {
	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	if len(tags) == 0 {
		return 1
	}
	inner := width
	if inner < 1 {
		inner = 1
	}
	wrapped := component.NewWordList(tags).WrapWords(inner)
	if len(wrapped) == 0 {
		return 1
	}
	return len(wrapped)
}

// tikiIDListHeight returns the dependency row count, capped at
// TikiListMetadataMaxRows. Counts every declared dependency (resolved or
// not) because the renderer emits one row per id even when unresolved
// (placeholder display).
func tikiIDListHeight(tk *tikipkg.Tiki, _ int) int {
	deps, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn)
	if len(deps) == 0 {
		return 1
	}
	depRows := len(deps)
	if depRows > config.TikiListMetadataMaxRows {
		depRows = config.TikiListMetadataMaxRows
	}
	return depRows
}

// listFieldCountText returns the item count of a list-typed field as a string
// ("0" for empty/missing/un-coercible), via StringSliceField — the same value
// path the list renderers and measures use, so a scalar coerced to a one-element
// list counts as 1, matching how it renders.
func listFieldCountText(name string, tk *tikipkg.Tiki) string {
	vals, _, _ := tk.StringSliceField(name)
	return strconv.Itoa(len(vals))
}

// renderConfiguredField looks up the field descriptor and routes through the
// type registry to produce a primitive. Fields that exist in the workflow
// catalog but not in the typed registry fall back to a generic catalog-
// driven row that reads the value verbatim from the tiki's Fields map.
//
// A `.count` anchor (DisplayCount) short-circuits before the registry lookup:
// keyed off the anchor's Display mode (not the field name), it renders the
// field's item count as a single-line value, honoring the anchor's role like
// any other value. Load-time validation guarantees the field is list-typed.
//
// Workflow-declared TypeEnum fields are routed to the SemanticEnum renderer
// even when they don't have a built-in FieldDescriptor — so user-declared
// enums (e.g. severity in bug-tracker.yaml) get the same display-with-emoji
// rendering and focus-aware coloring as the canonical status/type/priority.
func renderConfiguredField(name string, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if ctx.Display == gridlayout.DisplayCount {
		return valueOnlyLine(listFieldCountText(name, tk), ctx.Roles)
	}
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

// renderGenericWorkflowField produces a value-only row for a workflow-declared
// field that the typed registry doesn't have a custom renderer for. The
// caption (if wanted) is placed by the layout author as a literal cell.
func renderGenericWorkflowField(fd workflow.FieldDef, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	value := genericFieldValueString(fd, tk, ctx)
	return valueOnlyLine(value, ctx.Roles)
}

// genericFieldValueString formats a workflow field's value as a single-line
// string, dispatching on declared type. Empty/absent values render as a dash.
// User-controlled string values are escaped against tview's dynamic-color
// markup and then run through workflow.ExpandVisual so `<role>` color
// markup expands while literal `[...]` stays inert. Values that come from
// a controlled source (enum labels, formatted times, parsed numbers) are
// passed through verbatim.
func genericFieldValueString(fd workflow.FieldDef, tk *tikipkg.Tiki, ctx FieldRenderContext) string {
	// a `.count` cell has a well-defined value (0) even when the field is
	// absent, so it must be resolved before the absent-key dash guard below —
	// otherwise a project with no dependsOn key renders "— tasks" instead of
	// "0 tasks". Validated list-only at load, so this is safe for any DisplayCount.
	if ctx.Display == gridlayout.DisplayCount {
		return listFieldCountText(fd.Name, tk)
	}
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
		rendered := make([]string, len(ss))
		for i, s := range ss {
			rendered[i] = expandFieldText(s, ctx.Roles)
		}
		return strings.Join(rendered, ", ")
	case workflow.TypeRecurrence:
		// recurrence renders as its human display ("Weekly on Monday"), not the
		// raw cron — so the width measure (which routes here) matches what
		// renderRecurrenceValue draws. Without this the measure saw the shorter
		// cron string and the rendered display truncated in a tight column.
		if s, ok := raw.(string); ok && s != "" {
			return recurrence.RecurrenceDisplay(recurrence.Recurrence(s))
		}
		return "—"
	case workflow.TypeBool:
		if b, ok := raw.(bool); ok {
			if b {
				return "true"
			}
			return "false"
		}
	case workflow.TypeEnum:
		if s, ok := raw.(string); ok && s != "" {
			if ctx.Display == gridlayout.DisplayVisual {
				return expandFieldText(fd.EnumDisplay(s), ctx.Roles)
			}
			return fd.EnumLabel(s)
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
		return expandFieldText(v, ctx.Roles)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case time.Time:
		if v.IsZero() {
			return "—"
		}
		return v.Format("2006-01-02")
	default:
		return expandFieldText(fmt.Sprintf("%v", v), ctx.Roles)
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
	return gridbox.NewTruncatingTextView().SetText(text)
}

// valueOnlyLine returns a single-line value row in the value color, with no
// caption. Captions are first-class layout cells (LiteralCell) since the
// caption-from-field coupling was removed; renderers emit only the value.
func valueOnlyLine(value string, roles *theme.Theme) tview.Primitive {
	tag := roles.TextValue().Tag()
	return gridbox.NewTruncatingTextView().SetText(tag + value)
}

// --- semantic-type renderers ---

// renderTextValue is the read-only renderer for SemanticText fields.
// User-controlled values are first tview-escaped (so a stored `[red]` is
// inert) and then passed through workflow.ExpandVisual so deliberate
// `<role>` color markup expands. Unknown roles fail closed to the plain
// escaped text. The empty-placeholder branch above is internal and safe
// to skip the expand step.
func renderTextValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, ok := LookupField(ctx.FieldName)
	if !ok {
		return placeholderRow("(unknown)")
	}
	value, _, _ := tk.StringField(fd.Name)
	if value == "" {
		return valueOnlyLine(textEmptyPlaceholder(fd.Name), ctx.Roles)
	}
	return valueOnlyLine(expandFieldText(value, ctx.Roles), ctx.Roles)
}

func renderIntegerValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, ok := LookupField(ctx.FieldName)
	if !ok {
		return placeholderRow("(unknown)")
	}
	v, present, _ := tk.IntField(fd.Name)
	value := emptyPlaceholder(fd.Name, SemanticInteger)
	if present {
		value = fmt.Sprintf("%d", v)
	}
	return valueOnlyLine(value, ctx.Roles)
}

func renderBooleanValue(_ *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	return valueOnlyLine("(stub)", ctx.Roles)
}

func renderDateValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, ok := LookupField(ctx.FieldName)
	if !ok {
		return placeholderRow("(unknown)")
	}
	t, _, _ := tk.TimeField(fd.Name)
	value := emptyPlaceholder(fd.Name, SemanticDate)
	if !t.IsZero() {
		value = t.Format("2006-01-02")
	}
	return valueOnlyLine(value, ctx.Roles)
}

func renderDateTimeValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, ok := LookupField(ctx.FieldName)
	if !ok {
		return placeholderRow("(unknown)")
	}
	value := emptyPlaceholder(fd.Name, SemanticDateTime)
	if fd.Get != nil {
		if t, ok := fd.Get(tk).(time.Time); ok && !t.IsZero() {
			value = t.Format("2006-01-02 15:04")
		}
	}
	return valueOnlyLine(value, ctx.Roles)
}

// textEmptyPlaceholder returns the empty-value placeholder for a text field —
// a thin wrapper over emptyPlaceholder pinned to the text semantic.
func textEmptyPlaceholder(name string) string {
	return emptyPlaceholder(name, SemanticText)
}

func renderRecurrenceValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	recurrenceStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
	display := recurrence.RecurrenceDisplay(recurrence.Recurrence(recurrenceStr))
	return valueOnlyLine(display, ctx.Roles)
}

// renderEnumValue is the generic read-only renderer for any TypeEnum field.
// It replaces the per-field renderStatus/renderType/renderPriority helpers:
// look up the workflow descriptor, format the current value via EnumLabel
// (preferring the human-readable label over the compact visual), and apply
// the same focus-aware dim/full color treatment as the legacy renderers.
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
		return placeholderRow("(unknown)")
	}
	name := ctx.FieldName
	var editField model.EditField // zero value = unset, never matches a real field
	if hasFD {
		editField = fd.EditField
	}

	value, _, _ := tk.StringField(name)
	display := emptyPlaceholder(name, SemanticEnum)
	if value != "" {
		if hasWFD {
			if ctx.Display == gridlayout.DisplayVisual {
				display = expandFieldText(wfd.EnumDisplay(value), ctx.Roles)
			} else {
				display = tview.Escape(wfd.EnumLabel(value))
			}
		} else {
			display = tview.Escape(value)
		}
	} else if hasFD && fd.EmptyPlaceholder != "" {
		// a descriptor-declared placeholder (e.g. type→"(none)") is styled muted
		// at render time from ctx.Roles (registration is theme-independent — see
		// the EmptyPlaceholder comment on the type descriptor). Enums with no
		// per-field override keep the bare per-type default set above.
		display = ctx.Roles.TextMuted().Tag() + emptyPlaceholder(name, SemanticEnum) + "[-]"
	}

	focused := ctx.Mode == RenderModeEdit && editField != "" && ctx.FocusedField == editField
	valueTag := getDimOrFullColorTag(ctx.Mode, focused, ctx.Roles.TextValue(), ctx.Roles.TextSecondary())
	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Roles)
	}

	text := focusMarker + valueTag + display
	return gridbox.NewTruncatingTextView().SetText(text)
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

	keys := wfd.AllowedValues()
	labels := make([]string, len(keys))
	for i, k := range keys {
		labels[i] = wfd.EnumLabel(k)
	}
	currentKey, _, _ := tk.StringField(ctx.FieldName)
	currentLabel := wfd.EnumLabel(currentKey)

	editor := component.NewEditSelectList(labels, false)
	editor.SetLabel(getFocusMarker(ctx.Roles))
	editor.SetInitialValue(currentLabel)
	editor.SetSubmitHandler(func(text string) {
		if onChange == nil {
			return
		}
		if key, ok := wfd.EnumParseLabel(text); ok {
			onChange(key)
			return
		}
		onChange(text)
	})
	return &enumSelectAdapter{
		selectListAdapter: selectListAdapter{EditSelectList: editor},
		field:             wfd,
	}
}

// renderStringListValue renders a string-list field's value as a word-wrapped
// column. The field is read by ctx.FieldName so any stringList field renders
// correctly — not just the canonical tags field. Empty → "(none)" placeholder
// so the grid's height contract (always ≥ 1 row) holds.
func renderStringListValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	values, _, _ := tk.StringSliceField(ctx.FieldName)
	if len(values) == 0 {
		return valueOnlyLine(emptyPlaceholder(ctx.FieldName, SemanticStringList), ctx.Roles)
	}
	return wordListColumn(values)
}

// renderTikiIDListValue renders a tiki-id-list field's value as a column of
// "ID title" rows. The field is read by ctx.FieldName so any tikiIdList field
// renders correctly — not just the canonical dependsOn field. Each declared ID
// is resolved against the store; unresolved IDs render as a placeholder row so
// the rendered row count matches the height contract. Empty → "(none)"; no
// store → comma-joined IDs as a safe fallback.
func renderTikiIDListValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	ids, _, _ := tk.StringSliceField(ctx.FieldName)
	if len(ids) == 0 {
		return valueOnlyLine(emptyPlaceholder(ctx.FieldName, SemanticTikiIDList), ctx.Roles)
	}
	if ctx.Store == nil {
		return valueOnlyLine(strings.Join(ids, ", "), ctx.Roles)
	}
	return tikiIDListColumn(ids, ctx.Store)
}

// --- editor factories ---

// editTitleValue builds a plain text input for the title field.
func editTitleValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	roles := theme.Roles()
	input := tview.NewInputField()
	input.SetFieldBackgroundColor(roles.SurfaceCanvas().TCell())
	input.SetFieldTextColor(roles.TextPrimary().TCell())
	input.SetLabel(getFocusMarker(ctx.Roles))
	input.SetBorder(false)
	input.SetText(tk.Title())
	input.SetChangedFunc(func(text string) {
		if onChange != nil {
			onChange(text)
		}
	})
	return &titleEditAdapter{InputField: input}
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
	editor.SetLabel(getFocusMarker(ctx.Roles))
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
	editor.SetLabel(getFocusMarker(ctx.Roles))
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
	if recurrenceStr != "" && recurrence.Recurrence(recurrenceStr) != recurrence.RecurrenceNone {
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
	editor.SetLabel(getFocusMarker(ctx.Roles))
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
	textArea.SetPlaceholderStyle(tcell.StyleDefault.Foreground(ctx.Roles.TextMuted().TCell()))

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
	if name == "title" {
		return editTitleValue(tk, ctx, onChange)
	}
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
	label := a.selectListAdapter.GetText()
	if key, ok := a.field.EnumParseLabel(label); ok {
		return key
	}
	return label
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

// titleEditAdapter wraps tview.InputField for title editing — non-cyclable,
// so CycleValue always returns false.
type titleEditAdapter struct {
	*tview.InputField
}

func (a *titleEditAdapter) CycleValue(int) bool { return false }

func (a *titleEditAdapter) GetText() string {
	return a.InputField.GetText()
}

// descriptionEditAdapter wraps tview.TextArea for the inline description
// editor surfaced when Tab lands on the description pseudo-field. Mirrors
// tagsEditAdapter — non-cyclable, GetText returns the textarea content.
type descriptionEditAdapter struct {
	*tview.TextArea
}

func (a *descriptionEditAdapter) CycleValue(int) bool { return false }

// GetText returns the textarea content (the full description body).
func (a *descriptionEditAdapter) GetText() string {
	return a.TextArea.GetText()
}
