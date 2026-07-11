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
	"github.com/boolean-maybe/tiki/view/fieldmeta"
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
//
// The type and its constants are aliases into the tview-free fieldmeta leaf, so
// the editability tables there and the renderer/editor registry here key off
// exactly the same values. SemanticEnum is the unified renderer/editor for any
// TypeEnum field declared in workflow.yaml.
type SemanticType = fieldmeta.SemanticType

const (
	SemanticEnum       = fieldmeta.SemanticEnum
	SemanticText       = fieldmeta.SemanticText
	SemanticUser       = fieldmeta.SemanticUser
	SemanticInteger    = fieldmeta.SemanticInteger
	SemanticBoolean    = fieldmeta.SemanticBoolean
	SemanticDate       = fieldmeta.SemanticDate
	SemanticDateTime   = fieldmeta.SemanticDateTime
	SemanticRecurrence = fieldmeta.SemanticRecurrence
	SemanticStringList = fieldmeta.SemanticStringList
	SemanticTikiIDList = fieldmeta.SemanticTikiIDList
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
// (e.g. moved to a new option, incremented an integer), false for a
// non-cyclable widget. Used by both views to
// route Up/Down keypresses through a single dispatcher rather than typed
// per-widget switch tables.
type FieldEditorWidget interface {
	tview.Primitive
	GetText() string
	CycleValue(direction int) bool
}

// FieldEditor builds an in-place editor widget for a tiki's current value.
// onChange fires with the editor's new typed value rendered as a string.
// Each factory owns the typed→string conversion (for example, an integer
// adapter formats its value in decimal) so the receiver can
// parse the string back to the typed value at the receive boundary.
type FieldEditor func(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget

// FieldHeightFn computes the row count a field needs at a given inner
// column width. Single-row types return 1; list types return 1 + wrapped
// content rows. The grid solver clamps the result against the anchor's
// declared row span.
type FieldHeightFn func(name string, tk *tikipkg.Tiki, width int) int

// FieldEmptyFn reports whether tk holds no value for the named field. It is the
// typed emptiness predicate (date→IsZero, list→len==0, string→==""), keyed off
// the semantic type — NOT a formatted string. A nil FieldEmptyFn means the type
// is never empty (it always renders a concrete value, e.g. recurrence→"None"),
// so such fields are never `?`-hidden.
type FieldEmptyFn func(tk *tikipkg.Tiki, name string) bool

// EditMeasureFn returns the minimum column width (content cells, before the
// focus-marker reserve) a type needs in EDIT mode. It exists for in-place
// editors that cycle through values WITHOUT the grid re-solving: the column is
// sized once from the stored value, so an editor that can swap in a wider value
// (recurrence cycling Monday→Wednesday) would clip. Such a type returns its
// widest reachable rendered width here so the column fits any value the editor
// can produce. A nil EditMeasureFn means the edit-mode width equals the stored
// value's width (the default for fixed-width editors like dates).
type EditMeasureFn func() int

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
	// EmptyPlaceholderSet allows a semantic type to declare an intentionally
	// blank placeholder. EmptyPlaceholder alone cannot represent that because
	// the zero value means "fall through to the default dash".
	EmptyPlaceholderSet bool
	// EditMeasure, when set, overrides the stored-value width in edit mode with
	// the type's widest reachable value (see EditMeasureFn).
	EditMeasure EditMeasureFn
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
	// text renderers emit it verbatim. This replaces per-field-name
	// placeholder switches.
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

// FieldHeight resolves workflow type → HeightFn for a field. Single-row types
// and unknown fields return 1, ensuring an empty list field still reserves a
// row for its placeholder.
func FieldHeight(name string, tk *tikipkg.Tiki, width int) int {
	ui, ok := resolvedTypeUI(name)
	if !ok || ui.HeightFn == nil {
		return 1
	}
	return ui.HeightFn(name, tk, width)
}

// MeasureFieldValue returns the visible content width (in cells) of a field's
// rendered value for tk, used by the grid solver to size `auto` columns. It
// reuses genericFieldValueString — the same value-formatting path the view
// renders — so the measured width matches what is drawn. Color/markup tags do
// not count toward the width. Unknown fields measure 0 (the solver floors at 1).
//
// The two list types measure differently because they render differently:
//   - stringList renders as a word-wrapping column, so its useful
//     width is the longest single token — the comma-joined length would
//     massively over-reserve and wrongly squeeze neighbours.
//   - tikiIdList renders one non-wrapping "ID title" row per
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
	case workflow.TypeEnum:
		// the in-place enum editor cycles labels without the grid re-solving, so
		// the column must fit the WIDEST declared label — not just the stored
		// value — or cycling to a longer label clips. Sized identically in view
		// and edit mode so the column never shifts width as the value changes.
		return measureWidestEnumValue(fd, ctx)
	}
	// an empty scalar renders its placeholder ("None"/"Unknown"/"─"/…), not the
	// "—" sentinel genericFieldValueString emits — measure the SAME string the
	// renderer draws so the solver reserves the cell's true width.
	stored := scalarCellWidth(emptyPlaceholder(name, semanticForValueType(fd.Type)))
	if !fieldIsEmpty(tk, fd) {
		stored = scalarCellWidth(genericFieldValueString(fd, tk, ctx))
	}
	return maxInt(stored, editModeWidthFloor(name, ctx))
}

// measureWidestEnumValue returns the column width an enum field needs to hold
// its widest declared value — plus the scalar breathing cell — so an in-place
// cycle to a longer label never clips. It formats each value the same way
// renderEnumValue draws it: the visual form (label + emoji) for a `.visual`
// cell, the bare label otherwise. An enum with no declared values (should not
// happen for a TypeEnum field) falls back to the empty placeholder width.
func measureWidestEnumValue(fd workflow.FieldDef, ctx FieldRenderContext) int {
	widest := scalarCellWidth(emptyPlaceholder(fd.Name, SemanticEnum))
	for _, key := range fd.AllowedValues() {
		widest = maxInt(widest, scalarCellWidth(enumValueDisplay(fd, key, ctx)))
	}
	return widest
}

// enumValueDisplay renders a single enum key exactly as renderEnumValue does:
// the color-expanded visual form for a `.visual` cell, else the bare label.
func enumValueDisplay(fd workflow.FieldDef, key string, ctx FieldRenderContext) string {
	if ctx.Display == gridlayout.DisplayVisual {
		return expandFieldText(fd.EnumDisplay(key), ctx.Roles)
	}
	return tview.Escape(fd.EnumLabel(key))
}

// editModeWidthFloor returns the minimum content width a field's column needs in
// edit mode, independent of its stored value. It is non-zero only for types
// whose in-place editor cycles through values without re-solving the grid (see
// EditMeasureFn) — currently recurrence. Outside edit mode, or for types with no
// EditMeasure, it is 0 so the stored-value width governs.
func editModeWidthFloor(name string, ctx FieldRenderContext) int {
	if ctx.Mode != RenderModeEdit {
		return 0
	}
	ui, ok := editModeTypeUI(name)
	if !ok || ui.EditMeasure == nil {
		return 0
	}
	// EditMeasure() is the raw content width; add the breathing cell the
	// truncating value view reserves, mirroring scalarCellWidth.
	return ui.EditMeasure() + scalarBreathingCell
}

// resolvedTypeUI resolves the semantic TypeUI for a field the same way both the
// value-measure and the edit paths do: workflow metadata wins, else a static
// descriptor's semantic is used for descriptor-only fields. This is the single
// view-side authority the focusable/editable/measure layers derive from; the
// controller derives the same answer from fieldmeta.FieldHasEditor, which reads
// the same editability table this registry's Capability is built on.
func resolvedTypeUI(name string) (TypeUI, bool) {
	if wfd, ok := workflow.Field(name); ok {
		return LookupType(semanticForValueType(wfd.Type))
	}
	if fd, ok := LookupField(name); ok {
		return LookupType(fd.Semantic)
	}
	return TypeUI{}, false
}

// fieldIsReadOnly reports whether a field must never be edited. It defers to the
// fieldmeta leaf's read-only set, the single authority both view and controller
// share.
func fieldIsReadOnly(name string) bool {
	return fieldmeta.FieldIsReadOnly(name)
}

// editModeTypeUI resolves the TypeUI for a field's semantic in edit mode. It is
// an alias for resolvedTypeUI kept for its call sites (the EditMeasure floor
// path); a catalog-only field with no descriptor — e.g. a user-declared
// datetime like bug-tracker's dueBy — still resolves its type-level EditMeasure.
func editModeTypeUI(name string) (TypeUI, bool) {
	return resolvedTypeUI(name)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// scalarBreathingCell is the single column truncatingTextView.Draw reserves at
// the right edge of every scalar value cell (it truncates to width-1 so text
// never butts against the box border). The measure must add it back so the
// solver reserves content+1 — otherwise an N-cell value gets an N-cell column,
// the draw clips it to N-1, and a full-width datetime renders "2026-06-11 20:…".
const scalarBreathingCell = 1

// scalarCellWidth is the on-screen footprint of a scalar value: its rendered
// content width plus the breathing cell the truncating view reserves.
func scalarCellWidth(rendered string) int {
	return tview.TaggedStringWidth(rendered) + scalarBreathingCell
}

// measureStringListField returns the longest single token width across a string
// list field's values, mirroring the wordListColumn renderer (which wraps per
// word). An empty list measures its "(none)" placeholder width, so the solver
// reserves room for the placeholder the renderer draws instead of clipping it.
func measureStringListField(name string, tk *tikipkg.Tiki) int {
	vals, _, _ := tk.StringSliceField(name)
	if len(vals) == 0 {
		// empty list renders the "(none)" placeholder through valueOnlyLine's
		// truncating text view (see renderStringListValue), NOT the wordListColumn
		// the non-empty branch uses. The truncating view draws to width-1, so the
		// measure needs the placeholder width PLUS the breathing cell — exactly
		// like the scalar empty path. Without the +1 the last glyph clips ("(no…").
		return scalarCellWidth(emptyPlaceholder(name, SemanticStringList))
	}
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
// the renderer). An empty list measures its "(none)" placeholder width.
func measureTikiIDListField(name string, tk *tikipkg.Tiki, tikiStore store.Store) int {
	ids, _, _ := tk.StringSliceField(name)
	if len(ids) == 0 {
		// empty list renders the "(none)" placeholder through valueOnlyLine's
		// truncating text view (see renderTikiIDListValue), so it needs the
		// placeholder width plus a breathing cell — mirroring the stringList branch.
		return scalarCellWidth(emptyPlaceholder(name, SemanticTikiIDList))
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
		// a single-row composite draws through the truncating value view, which
		// reserves one right-edge cell (width-1). Add it back so the solver
		// reserves content+1 — mirroring scalarCellWidth / the literal +1 — else
		// a composite that exactly fills its column clips (e.g. a detail enum
		// cell "In Progress ⚙️" rendered "In Progress …").
		return tview.TaggedStringWidth(buildCompositeText(a, tk, ctx)) + scalarBreathingCell
	case gridlayout.AnchorLiteral:
		return gridbox.MeasureAnchorText(a)
	}
	if a.Display == gridlayout.DisplayCaption {
		return len([]rune(fieldCaptionText(a.Name))) + 1
	}
	ctx.Display = a.Display
	return MeasureFieldValue(a.Name, tk, ctx) + editFocusMarkerReserve(a.Name, ctx)
}

// focusMarkerWidth is the on-screen cell footprint of the edit-mode focus
// marker ("► ") that an editable field's editor renders inside its value
// cell when focused (getFocusMarker → InputField label). Computed via the
// same width function the renderer uses so the two never drift.
var focusMarkerWidth = tview.TaggedStringWidth("► ")

// editFocusMarkerReserve returns the extra cells an editable value cell needs
// in edit mode to fit the focus marker the focused editor draws inside it.
// Reserved for every editable field (not only the currently focused one) so
// the grid shape stays stable as Tab moves focus between fields — a column
// that grew and shrank by 2 cells per Tab would reflow the whole row. Returns
// 0 outside edit mode and for non-editable (read-only) fields, which never
// render the marker. Without this the solver sizes the column to the
// read-only measure and the focused editor truncates its tail (e.g. the
// recurrence editor clipped "Weekly > Tuesday" to "Weekly > T").
func editFocusMarkerReserve(name string, ctx FieldRenderContext) int {
	if ctx.Mode != RenderModeEdit {
		return 0
	}
	if !FieldHasEditor(name) {
		return 0
	}
	return focusMarkerWidth
}

// registerBuiltinFields wires the workflow-declared fields into the registry.
func registerBuiltinFields() {
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

// registerBuiltinTypes wires the renderer/editor primitives for every semantic
// type. Each entry declares its Render/Edit/HeightFn/etc.; the per-type
// EditorCapability is NOT set here — deriveCapabilities() computes it from the
// fieldmeta editability table so "editable" is stated exactly once (in the leaf)
// and cannot drift from what FieldHasEditor reports.
func registerBuiltinTypes() {
	typeRegistry[SemanticText] = TypeUI{
		Render:   renderTextValue,
		Edit:     editTextValue,
		HeightFn: singleRowHeight,
		IsEmpty:  stringFieldEmpty,
		// no per-type default: the "─" fallback in emptyPlaceholder applies;
		// per-field overrides supply values such as "Unknown".
	}
	typeRegistry[SemanticUser] = TypeUI{
		Render:              renderUserValue,
		Edit:                editUserValue,
		HeightFn:            singleRowHeight,
		IsEmpty:             stringFieldEmpty,
		EmptyPlaceholderSet: true,
	}
	typeRegistry[SemanticInteger] = TypeUI{
		Render:           renderIntegerValue,
		Edit:             editIntegerValue,
		HeightFn:         singleRowHeight,
		IsEmpty:          intFieldEmpty,
		EmptyPlaceholder: "─",
	}
	typeRegistry[SemanticBoolean] = TypeUI{
		Render:   renderBooleanValue,
		Edit:     editBooleanValue,
		HeightFn: singleRowHeight,
		// IsEmpty nil: booleans default to false and are never empty.
		// EditMeasure: the editor cycles false↔true without the grid re-solving,
		// so the column must fit the widest reachable value ("false").
		EditMeasure: func() int { return len("false") },
	}
	typeRegistry[SemanticDate] = TypeUI{
		Render:           renderDateValue,
		Edit:             editDateValue,
		HeightFn:         singleRowHeight,
		IsEmpty:          timeFieldEmpty,
		EmptyPlaceholder: "None",
	}
	typeRegistry[SemanticDateTime] = TypeUI{
		Render:           renderDateTimeValue,
		Edit:             editDateTimeValue,
		HeightFn:         singleRowHeight,
		IsEmpty:          timeFieldEmpty,
		EmptyPlaceholder: "Unknown",
		// EditMeasure: focusing an EMPTY datetime seeds a full "YYYY-MM-DD HH:MM"
		// (the segmented editor never shows the "Unknown" placeholder once
		// focused), so the column must reserve the full 16-cell value width even
		// when the stored value is empty — otherwise the seeded value clips.
		EditMeasure: widestDateTimeWidth,
	}
	typeRegistry[SemanticRecurrence] = TypeUI{
		Render:   renderRecurrenceValue,
		Edit:     editRecurrenceValue,
		HeightFn: singleRowHeight,
		// IsEmpty nil: an empty recurrence renders "None" via RecurrenceDisplay,
		// so it is never `?`-hidden.
		// EditMeasure: the in-place editor cycles frequency/weekday without the
		// grid re-solving, so the column must fit the widest reachable value.
		EditMeasure: widestRecurrenceWidth,
	}
	typeRegistry[SemanticEnum] = TypeUI{
		Render:           renderEnumValue,
		Edit:             editEnumValue,
		HeightFn:         singleRowHeight,
		IsEmpty:          stringFieldEmpty,
		EmptyPlaceholder: "─",
	}
	typeRegistry[SemanticStringList] = TypeUI{
		Render:           renderStringListValue,
		Edit:             editStringListValue,
		HeightFn:         stringListHeight,
		IsEmpty:          listFieldEmpty,
		EmptyPlaceholder: "(none)",
	}
	typeRegistry[SemanticTikiIDList] = TypeUI{
		Render:           renderTikiIDListValue,
		HeightFn:         tikiIDListHeight,
		IsEmpty:          listFieldEmpty,
		EmptyPlaceholder: "(none)",
	}
	deriveCapabilities()
}

// deriveCapabilities sets each registered type's EditorCapability from the
// fieldmeta editability table — EditorImplemented iff the leaf marks the
// semantic editable, else EditorStub. This makes the leaf the single source of
// truth: enabling a new editable type is one flip in fieldmeta, and the registry
// and FieldHasEditor stay consistent by construction. It also guards the two
// halves against drift: a type the leaf calls editable but whose Edit factory is
// still nil is a programming error, so it panics at init rather than silently
// rendering a focusable-but-unbuildable field.
func deriveCapabilities() {
	for sem, ui := range typeRegistry {
		if !fieldmeta.SemanticEditable(sem) {
			ui.Capability = EditorStub
			typeRegistry[sem] = ui
			continue
		}
		if ui.Edit == nil {
			panic(fmt.Sprintf("fieldmeta marks %q editable but its TypeUI has no Edit factory", sem))
		}
		ui.Capability = EditorImplemented
		typeRegistry[sem] = ui
	}
}

// typed emptiness predicates shared across semantic types. Each reads the
// underlying value via the canonical accessor (the descriptor's Get func when
// registered, else the tiki accessor) — no string formatting, no sentinel.
// Consulting the descriptor Get is what lets computed system fields (createdAt /
// updatedAt) report emptiness from their struct-backed value rather than from
// the frontmatter map, which never holds them; otherwise they always measured
// as empty and their column was sized to the "Unknown" placeholder.
func stringFieldEmpty(tk *tikipkg.Tiki, name string) bool {
	if fd, ok := LookupField(name); ok && fd.Get != nil {
		s, _ := fd.Get(tk).(string)
		return s == ""
	}
	v, _, _ := tk.StringField(name)
	return v == ""
}
func timeFieldEmpty(tk *tikipkg.Tiki, name string) bool {
	if fd, ok := LookupField(name); ok && fd.Get != nil {
		t, _ := fd.Get(tk).(time.Time)
		return t.IsZero()
	}
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
// enums). It delegates to fieldmeta.ForValueType so the view and the tview-free
// editability leaf share one bridge. The string family
// (TypeString/TypeID/TypeRef/TypeDuration) and any unmapped type fall through to
// SemanticText — string emptiness — matching genericFieldValueString's default.
func semanticForValueType(t workflow.ValueType) SemanticType {
	return fieldmeta.ForValueType(t)
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
	if ui, ok := LookupType(semantic); ok {
		if ui.EmptyPlaceholderSet {
			return ui.EmptyPlaceholder
		}
		if ui.EmptyPlaceholder != "" {
			return ui.EmptyPlaceholder
		}
	}
	return "─"
}

// singleRowHeight is the HeightFn for fixed one-line fields.
func singleRowHeight(_ string, _ *tikipkg.Tiki, _ int) int { return 1 }

// stringListHeight uses WordList wrap to compute the wrapped row count. The
// list-value column carries no padding, so the wrap width is the column width
// as-is. Returns 1 for an empty list so the placeholder still gets a row.
func stringListHeight(name string, tk *tikipkg.Tiki, width int) int {
	values, _, _ := tk.StringSliceField(name)
	if len(values) == 0 {
		return 1
	}
	inner := width
	if inner < 1 {
		inner = 1
	}
	wrapped := component.NewWordList(values).WrapWords(inner)
	if len(wrapped) == 0 {
		return 1
	}
	return len(wrapped)
}

// tikiIDListHeight returns the reference row count, capped at
// TikiListMetadataMaxRows. It counts unresolved references because the renderer
// emits a placeholder row for each ID.
func tikiIDListHeight(name string, tk *tikipkg.Tiki, _ int) int {
	references, _, _ := tk.StringSliceField(name)
	if len(references) == 0 {
		return 1
	}
	rows := len(references)
	if rows > config.TikiListMetadataMaxRows {
		rows = config.TikiListMetadataMaxRows
	}
	return rows
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
// even when they don't have a FieldDescriptor, so every enum gets the same
// display-with-emoji rendering and focus-aware coloring.
func renderConfiguredField(name string, tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	if ctx.Display == gridlayout.DisplayCount {
		return valueOnlyLine(listFieldCountText(name, tk), ctx.Roles)
	}
	if wfd, ok := workflow.Field(name); ok {
		ctx.FieldName = wfd.Name
		switch wfd.Type {
		case workflow.TypeUser:
			return renderUserValue(tk, ctx)
		case workflow.TypeDate:
			return renderDateValue(tk, ctx)
		case workflow.TypeRecurrence:
			return renderRecurrenceValue(tk, ctx)
		case workflow.TypeListString:
			return renderStringListValue(tk, ctx)
		case workflow.TypeListRef:
			return renderTikiIDListValue(tk, ctx)
		}
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

// fieldRawValue resolves a field's raw value from its canonical source. The
// registry descriptor's Get func is that source when one is registered — it is
// the accessor the renderers use, and for computed system fields (createdAt /
// updatedAt, backed by tk.CreatedAt()/UpdatedAt() struct fields rather than the
// frontmatter map) it is the ONLY source. Plain frontmatter fields with no
// descriptor Get fall back to tk.Get. Routing both the value formatter and the
// emptiness check through here keeps the width measure aligned with what the
// renderer draws — without it a computed timestamp measured its empty
// placeholder ("Unknown", 7 cells) while the renderer drew the full datetime
// (16 cells), starving the column so it clipped to "2026-06-11…".
func fieldRawValue(name string, tk *tikipkg.Tiki) (any, bool) {
	if fd, ok := LookupField(name); ok && fd.Get != nil {
		return fd.Get(tk), true
	}
	return tk.Get(name)
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
	// otherwise an absent list field renders a dash instead of zero. Validated
	// list-only at load, so this is safe for any DisplayCount.
	if ctx.Display == gridlayout.DisplayCount {
		return listFieldCountText(fd.Name, tk)
	}
	raw, ok := fieldRawValue(fd.Name, tk)
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
			return t.Format(value.DateTimeFormat)
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

func renderUserValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	value, _, _ := tk.StringField(ctx.FieldName)
	if value == "" {
		return valueOnlyLine(emptyPlaceholder(ctx.FieldName, SemanticUser), ctx.Roles)
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

// renderBooleanValue renders a boolean field's value ("true"/"false"). Absent →
// "false" (booleans default false and are never empty, so no placeholder path).
func renderBooleanValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	return valueOnlyLine(boolFieldString(tk, ctx.FieldName), ctx.Roles)
}

// boolFieldString reads a boolean field's canonical string, defaulting to
// "false" for absent or non-bool values. Shared by the renderer and editor seed.
func boolFieldString(tk *tikipkg.Tiki, name string) string {
	if raw, ok := tk.Get(name); ok {
		if b, ok := raw.(bool); ok && b {
			return "true"
		}
	}
	return "false"
}

func renderDateValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	t, _, _ := tk.TimeField(ctx.FieldName)
	value := emptyPlaceholder(ctx.FieldName, SemanticDate)
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
	display := emptyPlaceholder(fd.Name, SemanticDateTime)
	if fd.Get != nil {
		if t, ok := fd.Get(tk).(time.Time); ok && !t.IsZero() {
			display = t.Format(value.DateTimeFormat)
		}
	}
	return valueOnlyLine(display, ctx.Roles)
}

// textEmptyPlaceholder returns the empty-value placeholder for a text field —
// a thin wrapper over emptyPlaceholder pinned to the text semantic.
func textEmptyPlaceholder(name string) string {
	return emptyPlaceholder(name, SemanticText)
}

func renderRecurrenceValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	recurrenceStr, _, _ := tk.StringField(ctx.FieldName)
	display := recurrence.RecurrenceDisplay(recurrence.Recurrence(recurrenceStr))
	return valueOnlyLine(display, ctx.Roles)
}

// widestRecurrenceWidth returns the widest read-only display width across every
// value the recurrence editor can cycle to (each weekly weekday and each
// monthly day-of-month). It is the EditMeasure for SemanticRecurrence: the
// in-place editor swaps values without the grid re-solving, so the column must
// be sized for the worst case up front. The read-only display ("Weekly on
// Wednesday") is at least as wide as the editor string ("Weekly > Wednesday"),
// so sizing to it covers the editor too. Computed lazily once and cached.
var widestRecurrenceWidthCached int

// widestDateTimeWidth is the on-screen width of a fully-populated datetime
// value ("YYYY-MM-DD HH:MM"). The segmented editor can only ever display a
// value of exactly this width, so it is both the floor and the ceiling.
func widestDateTimeWidth() int {
	return len(value.DateTimeFormat) // "2006-01-02 15:04" → 16
}

func widestRecurrenceWidth() int {
	if widestRecurrenceWidthCached > 0 {
		return widestRecurrenceWidthCached
	}
	widest := 0
	consider := func(cron string) {
		if w := tview.TaggedStringWidth(recurrence.RecurrenceDisplay(recurrence.Recurrence(cron))); w > widest {
			widest = w
		}
	}
	for _, weekday := range recurrence.AllWeekdays() {
		consider(string(recurrence.WeeklyRecurrence(weekday)))
	}
	for day := 1; day <= 31; day++ {
		consider(string(recurrence.MonthlyRecurrence(day)))
	}
	widestRecurrenceWidthCached = widest
	return widest
}

// renderEnumValue is the generic read-only renderer for any TypeEnum field.
// It replaces the former per-field enum renderers:
// look up the workflow descriptor, format the current value via EnumLabel
// (preferring the human-readable label over the compact visual), and apply
// the same focus-aware dim/full color treatment as the legacy renderers.
//
// The workflow field name is also its edit-focus identity.
func renderEnumValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	fd, hasFD := LookupField(ctx.FieldName)
	wfd, hasWFD := workflow.Field(ctx.FieldName)
	if !hasFD && !hasWFD {
		return placeholderRow("(unknown)")
	}
	name := ctx.FieldName
	editField := model.EditField(name)

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
// The workflow FieldDef is the authoritative source for allowed values and
// display formatting.
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
// correctly. Empty → "(none)" placeholder so the grid's height contract
// (always ≥ 1 row) holds.
func renderStringListValue(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	values, _, _ := tk.StringSliceField(ctx.FieldName)
	if len(values) == 0 {
		return valueOnlyLine(emptyPlaceholder(ctx.FieldName, SemanticStringList), ctx.Roles)
	}
	return wordListColumn(values)
}

// renderTikiIDListValue renders a tiki-id-list field's value as a column of
// "ID title" rows. The field is read by ctx.FieldName so any tikiIdList field
// renders correctly. Each declared ID is resolved against the store; unresolved
// IDs render as a placeholder row so the rendered row count matches the height
// contract. Empty → "(none)"; no store → comma-joined IDs as a safe fallback.
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

// editTextValue is the generic in-place editor for any SemanticText field. It
// always builds a plain input. User suggestions live under SemanticUser so the
// workflow field type, not a field name or descriptor trait, chooses the picker.
func editTextValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	current, _, _ := tk.StringField(ctx.FieldName)
	input := tview.NewInputField()
	roles := ctx.Roles
	input.SetFieldBackgroundColor(roles.SurfaceCanvas().TCell())
	input.SetFieldTextColor(roles.TextPrimary().TCell())
	input.SetLabel(getFocusMarker(ctx.Roles))
	input.SetBorder(false)
	input.SetText(current)
	input.SetChangedFunc(func(text string) {
		if onChange != nil {
			onChange(text)
		}
	})
	return &textInputEditAdapter{InputField: input}
}

func editUserValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	current, _, _ := tk.StringField(ctx.FieldName)
	var options []string
	if ctx.Store != nil {
		if users, err := ctx.Store.GetAllUsers(); err == nil {
			options = users
		}
	}
	if len(options) == 0 && current != "" {
		options = []string{current}
	}
	editor := component.NewEditSelectList(options, true)
	editor.SetLabel(getFocusMarker(ctx.Roles))
	editor.SetInitialValue(current)
	editor.SetSubmitHandler(func(text string) {
		if onChange != nil {
			onChange(text)
		}
	})
	return &selectListAdapter{EditSelectList: editor}
}

// editIntegerValue is the generic in-place editor for any SemanticInteger
// field. It is a free-type input with a digit(+leading-minus) filter — integers
// are unbounded in the workflow schema, so there is no spinner range to cycle.
// An absent value opens blank; onChange fires the decimal string (or "" when
// cleared).
func editIntegerValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	input := tview.NewInputField()
	roles := ctx.Roles
	input.SetFieldBackgroundColor(roles.SurfaceCanvas().TCell())
	input.SetFieldTextColor(roles.TextPrimary().TCell())
	input.SetLabel(getFocusMarker(ctx.Roles))
	input.SetBorder(false)
	input.SetAcceptanceFunc(acceptSignedInteger)
	if v, present, _ := tk.IntField(ctx.FieldName); present {
		input.SetText(strconv.Itoa(v))
	}
	input.SetChangedFunc(func(text string) {
		if onChange != nil {
			onChange(text)
		}
	})
	return &intEditAdapter{InputField: input}
}

// acceptSignedInteger permits an optional leading '-' followed by digits, and
// the intermediate states a user types through ("" and "-"). tview calls this on
// every keystroke with the prospective text; returning false rejects the key.
func acceptSignedInteger(textToCheck string, _ rune) bool {
	if textToCheck == "" || textToCheck == "-" {
		return true
	}
	s := strings.TrimPrefix(textToCheck, "-")
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// editBooleanValue is the generic in-place editor for any SemanticBoolean
// field. It is a two-value cycle (false↔true) surfaced through a select-list so
// the grid's Up/Down cycle dispatch works uniformly. Absent seeds "false".
func editBooleanValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	editor := component.NewEditSelectList([]string{"false", "true"}, false)
	editor.SetLabel(getFocusMarker(ctx.Roles))
	editor.SetInitialValue(boolFieldString(tk, ctx.FieldName))
	editor.SetSubmitHandler(func(text string) {
		if onChange != nil {
			onChange(text)
		}
	})
	return &boolEditAdapter{selectListAdapter: selectListAdapter{EditSelectList: editor}}
}

// editDateValue builds a date editor. The widget's onChange fires with
// the validated formatted string after each accepted change.
func editDateValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	date, _, _ := tk.TimeField(ctx.FieldName)
	editor := component.NewDateEdit()
	editor.SetLabel(getFocusMarker(ctx.Roles))
	editor.SetChangeHandler(func(s string) {
		if onChange != nil {
			onChange(s)
		}
	})
	var initial string
	if !date.IsZero() {
		initial = date.Format(value.DateFormat)
	}
	editor.SetInitialValue(initial)
	return &dateEditAdapter{DateEdit: editor}
}

// editDateTimeValue builds the segmented datetime editor. It reads the field's
// value generically by ctx.FieldName so it serves both descriptor-backed and
// catalog-only (workflow TypeTimestamp) fields. The widget fires the canonical
// "YYYY-MM-DD HH:MM" string (or "" when cleared) on every accepted change.
func editDateTimeValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	editor := component.NewDateTimeEdit()
	editor.SetLabel(getFocusMarker(ctx.Roles))
	editor.SetChangeHandler(func(s string) {
		if onChange != nil {
			onChange(s)
		}
	})
	if t, present, _ := tk.TimeField(ctx.FieldName); present && !t.IsZero() {
		editor.SetInitialValue(value.FormatDateTime(t))
	}
	return &dateTimeEditAdapter{DateTimeEdit: editor}
}

// editRecurrenceValue builds the recurrence editor. RecurrenceEdit.GetValue()
// assembles a canonical cron expression from the freq/value parts; the
// adapter exposes that as GetText() so the registry boundary stays uniform.
func editRecurrenceValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	recurrenceStr, _, _ := tk.StringField(ctx.FieldName)
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

// editStringListValue builds a string-list textarea editor. Values are whitespace-joined
// for transport so a single string round-trips through onChange/GetText.
func editStringListValue(tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	textArea := tview.NewTextArea()
	textArea.SetBorder(false)
	// no horizontal padding: the list value cell is sized by the solver to the
	// read-only WordList footprint (no padding — see wordListColumn), so a padded
	// editor would have an inner width 2 cells narrower than the column the solver
	// reserved and wrap a seed value mid-word ("idea" → "id"/"ea") the moment focus
	// landed on the field. The editor's footprint must match its measured width.
	textArea.SetBorderPadding(0, 0, 0, 0)
	textArea.SetPlaceholder("space-separated values")
	textArea.SetPlaceholderStyle(tcell.StyleDefault.Foreground(ctx.Roles.TextMuted().TCell()))

	values, _, _ := tk.StringSliceField(ctx.FieldName)
	textArea.SetText(strings.Join(values, " "), false)

	a := &stringListEditAdapter{TextArea: textArea, onChange: onChange}
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

// buildFieldEditor returns the editor widget for a field via the single semantic
// resolver. title keeps a dedicated factory (editTitleValue is not
// type-registered and title needs plain-input behavior distinct from the generic
// text editor) — it is the sole intentional field-name branch, NOT a type
// whitelist. ctx is stamped with the field name so generic factories (SemanticEnum,
// SemanticText, datetime) resolve their target via ctx.FieldName, working for
// descriptor-backed and catalog-only fields alike.
func buildFieldEditor(name string, tk *tikipkg.Tiki, ctx FieldRenderContext, onChange func(string)) FieldEditorWidget {
	if name == "title" {
		return editTitleValue(tk, ctx, onChange)
	}
	if !FieldHasEditor(name) {
		return nil
	}
	ui, ok := resolvedTypeUI(name)
	if !ok || ui.Edit == nil {
		return nil
	}
	ctx.FieldName = name
	return ui.Edit(tk, ctx, onChange)
}

// FieldHasEditor reports whether the named field has a registered, fully
// implemented editor. It delegates to the tview-free fieldmeta leaf so the view
// (focusability) and the controller (save-handler wiring) share one predicate
// and can never disagree — the former {TypeEnum, TypeTimestamp} catalog
// whitelist is gone. Editable iff the field is not read-only and its resolved
// semantic type is marked editable in fieldmeta.
func FieldHasEditor(name string) bool {
	return fieldmeta.FieldHasEditor(name)
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

// dateEditAdapter wraps DateEdit to satisfy FieldEditorWidget.
type dateEditAdapter struct {
	*component.DateEdit
}

func (a *dateEditAdapter) CycleValue(direction int) bool {
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

// dateTimeEditAdapter wraps DateTimeEdit to satisfy FieldEditorWidget.
type dateTimeEditAdapter struct {
	*component.DateTimeEdit
}

// CycleValue routes grid-level Up/Down through the widget's segment cycle.
// direction >= 0 (Down/next) increments; direction < 0 (Up/prev) decrements —
// matching dateEditAdapter's mapping so Tab-cycle behaves consistently.
func (a *dateTimeEditAdapter) CycleValue(direction int) bool {
	key := tcell.KeyDown
	if direction < 0 {
		key = tcell.KeyUp
	}
	a.InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), nil)
	return true
}

// GetText returns the canonical formatted datetime (or "" when empty).
func (a *dateTimeEditAdapter) GetText() string {
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

// stringListEditAdapter wraps tview.TextArea — non-cyclable, so CycleValue
// always returns false.
type stringListEditAdapter struct {
	*tview.TextArea
	onChange func(string)
}

func (a *stringListEditAdapter) CycleValue(int) bool { return false }

// GetText returns the textarea content.
func (a *stringListEditAdapter) GetText() string {
	return a.TextArea.GetText()
}

// textInputEditAdapter wraps tview.InputField for generic text editing —
// non-cyclable, GetText returns the field content.
type textInputEditAdapter struct {
	*tview.InputField
}

func (a *textInputEditAdapter) CycleValue(int) bool { return false }
func (a *textInputEditAdapter) GetText() string     { return a.InputField.GetText() }

// intEditAdapter wraps tview.InputField for integer editing — non-cyclable
// (free-type), GetText returns the decimal string as typed.
type intEditAdapter struct {
	*tview.InputField
}

func (a *intEditAdapter) CycleValue(int) bool { return false }
func (a *intEditAdapter) GetText() string     { return a.InputField.GetText() }

// boolEditAdapter reuses the select-list cycle; GetText returns the chosen
// "true"/"false" string directly (the option labels ARE the canonical values,
// so no key/label conversion is needed).
type boolEditAdapter struct {
	selectListAdapter
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
// stringListEditAdapter — non-cyclable, GetText returns the textarea content.
type descriptionEditAdapter struct {
	*tview.TextArea
}

func (a *descriptionEditAdapter) CycleValue(int) bool { return false }

// GetText returns the textarea content (the full description body).
func (a *descriptionEditAdapter) GetText() string {
	return a.TextArea.GetText()
}
