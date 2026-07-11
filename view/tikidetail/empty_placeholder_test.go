package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"

	"github.com/rivo/tview"
)

// TestMeasureFieldValue_EmptyDateMatchesRenderedWidth is the regression guard
// for the original bug: an empty date used to MEASURE the "—" sentinel (width 1)
// while it RENDERS "None" (width 4), so a content-sized column under-reserved and
// clipped to "N". The measure must cover the drawn placeholder width PLUS the
// one breathing cell the truncating view reserves (it draws to width-1).
func TestMeasureFieldValue_EmptyDateMatchesRenderedWidth(t *testing.T) {
	const fieldName = "when"
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: fieldName, Type: workflow.TypeDate, Custom: true},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles(), FieldName: fieldName}
	rendered := extractTextView(renderConfiguredField(fieldName, tk, ctx), true)
	if strings.TrimSpace(rendered) != "None" {
		t.Fatalf("empty date rendered %q, want %q", rendered, "None")
	}

	got := MeasureFieldValue(fieldName, tk, ctx)
	want := tview.TaggedStringWidth(rendered) + scalarBreathingCell
	if got != want {
		t.Errorf("MeasureFieldValue(empty date) = %d, want %d (rendered %q + breathing cell)", got, want, rendered)
	}
	if got != 4+scalarBreathingCell {
		t.Errorf("MeasureFieldValue(empty date) = %d, want %d (\"None\" + breathing)", got, 4+scalarBreathingCell)
	}
}

// TestMeasureFieldValue_EmptyDateTimeMatchesRenderedWidth mirrors the date case
// for a TypeTimestamp field, whose empty placeholder is "Unknown" (width 7).
func TestMeasureFieldValue_EmptyDateTimeMatchesRenderedWidth(t *testing.T) {
	tk := tikipkg.New() // createdAt zero

	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles(), FieldName: "createdAt"}
	got := MeasureFieldValue("createdAt", tk, ctx)
	if want := len("Unknown") + scalarBreathingCell; got != want {
		t.Errorf("MeasureFieldValue(empty createdAt) = %d, want %d (\"Unknown\" + breathing)", got, want)
	}
}

// TestMeasureFieldValue_EmptyStringListMatchesPlaceholderWidth is the regression
// guard for a clipped list placeholder: an empty stringList RENDERS
// its "(none)" placeholder through valueOnlyLine's truncating text view, which
// draws to width-1. measureStringListField used to return the longest-token
// floor of 1, so the solver reserved a 1-cell column and the placeholder clipped.
// The measure must equal the placeholder width PLUS the breathing cell the
// truncating view reserves — otherwise the last glyph clips ("(no…").
func TestMeasureFieldValue_EmptyStringListMatchesPlaceholderWidth(t *testing.T) {
	const fieldName = "labels"
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: fieldName, Type: workflow.TypeListString},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)
	tk := tikipkg.New()

	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles(), FieldName: fieldName}
	placeholder := emptyPlaceholder(fieldName, SemanticStringList)

	got := MeasureFieldValue(fieldName, tk, ctx)
	if want := tview.TaggedStringWidth(placeholder) + scalarBreathingCell; got != want {
		t.Errorf("MeasureFieldValue(empty list) = %d, want %d (placeholder %q + breathing cell)", got, want, placeholder)
	}
}

func TestFormerPointsFieldNameUsesDeclaredStringPlaceholder(t *testing.T) {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{Name: "points", Type: workflow.TypeString},
	})
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	got := strings.TrimSpace(extractTextView(renderConfiguredField("points", tk, ctx), true))
	if got != "—" {
		t.Fatalf("empty string field named points rendered %q, want %q", got, "—")
	}
}

func TestFormerPriorityFieldNameUsesDeclaredStringPlaceholder(t *testing.T) {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{Name: "priority", Type: workflow.TypeString},
	})
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	got := strings.TrimSpace(extractTextView(renderConfiguredField("priority", tk, ctx), true))
	if got != "—" {
		t.Fatalf("empty string field named priority rendered %q, want %q", got, "—")
	}
}

func TestFormerTypeFieldNameUsesDeclaredStringPlaceholder(t *testing.T) {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{Name: "type", Type: workflow.TypeString},
	})
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	got := strings.TrimSpace(extractTextView(renderConfiguredField("type", tk, ctx), true))
	if got != "—" {
		t.Fatalf("empty string field named type rendered %q, want %q", got, "—")
	}
}

func TestFormerStatusFieldNameUsesDeclaredStringPlaceholder(t *testing.T) {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{Name: "status", Type: workflow.TypeString},
	})
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	got := strings.TrimSpace(extractTextView(renderConfiguredField("status", tk, ctx), true))
	if got != "—" {
		t.Fatalf("empty string field named status rendered %q, want %q", got, "—")
	}
}

// TestMeasureFieldValue_EmptyTikiIDListMatchesPlaceholderWidth mirrors the above
// for a TypeListRef field: empty renders "(none)" through the same truncating
// view, so it needs placeholder width + breathing cell.
func TestMeasureFieldValue_EmptyTikiIDListMatchesPlaceholderWidth(t *testing.T) {
	const fieldName = "refs"
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: fieldName, Type: workflow.TypeListRef},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)
	tk := tikipkg.New()

	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles(), FieldName: fieldName}
	placeholder := emptyPlaceholder(fieldName, SemanticTikiIDList)

	got := MeasureFieldValue(fieldName, tk, ctx)
	if want := tview.TaggedStringWidth(placeholder) + scalarBreathingCell; got != want {
		t.Errorf("MeasureFieldValue(empty refs) = %d, want %d (placeholder %q + breathing cell)", got, want, placeholder)
	}
}

// TestFieldIsEmpty_TypedPredicate pins the typed emptiness check across every
// participating semantic type, plus the never-empty types (recurrence, boolean)
// which report false regardless so they are never `?`-hidden.
func TestFieldIsEmpty_TypedPredicate(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "when", Type: workflow.TypeDate, Custom: true},
		{Name: "labels", Type: workflow.TypeListString, Custom: true},
		{Name: "deps", Type: workflow.TypeListRef, Custom: true},
		{Name: "owner", Type: workflow.TypeString, Custom: true},
		{Name: "reviewer", Type: workflow.TypeUser, Custom: true},
		{Name: "count", Type: workflow.TypeInt, Custom: true},
		{Name: "flag", Type: workflow.TypeBool, Custom: true},
		{Name: "cron", Type: workflow.TypeRecurrence, Custom: true},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	defer teststatuses.Init()

	empty := tikipkg.New()

	full := tikipkg.New()
	full.Set("when", "2026-01-02")
	full.Set("labels", []string{"x"})
	full.Set("deps", []string{"ABC123"})
	full.Set("owner", "alice")
	full.Set("reviewer", "alice")
	full.Set("count", 3)

	mustField := func(name string) workflow.FieldDef {
		fd, ok := workflow.Field(name)
		if !ok {
			t.Fatalf("field %q not registered", name)
		}
		return fd
	}

	emptyCases := []string{"when", "labels", "deps", "owner", "reviewer", "count"}
	for _, name := range emptyCases {
		if !fieldIsEmpty(empty, mustField(name)) {
			t.Errorf("fieldIsEmpty(empty, %q) = false, want true", name)
		}
		if fieldIsEmpty(full, mustField(name)) {
			t.Errorf("fieldIsEmpty(full, %q) = true, want false", name)
		}
	}

	// never-empty types: nil IsEmpty ⇒ always false, even with no value set.
	for _, name := range []string{"flag", "cron"} {
		if fieldIsEmpty(empty, mustField(name)) {
			t.Errorf("fieldIsEmpty(empty, %q) = true, want false (never-empty type)", name)
		}
	}
}

// TestEmptyPlaceholder_ResolutionOrder pins per-field override > per-type
// default > "─".
func TestEmptyPlaceholder_ResolutionOrder(t *testing.T) {
	cases := []struct {
		name     string
		field    string
		semantic SemanticType
		want     string
	}{
		{"per-field override beats type default", "createdBy", SemanticText, "Unknown"},
		{"per-type default when no override", "when", SemanticDate, "None"},
		{"enum with no override falls to dash", "status", SemanticEnum, "─"},
		{"user empty stays blank", "reviewer", SemanticUser, ""},
		{"datetime default", "createdAt", SemanticDateTime, "Unknown"},
		{"unknown field, type default", "does-not-exist", SemanticStringList, "(none)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := emptyPlaceholder(c.field, c.semantic); got != c.want {
				t.Errorf("emptyPlaceholder(%q, %q) = %q, want %q", c.field, c.semantic, got, c.want)
			}
		})
	}
}

// TestSemanticForValueType pins the catalog-type → registry-semantic bridge,
// including the string-family fallback (TypeID/TypeRef/TypeDuration/TypeString
// all → SemanticText).
func TestSemanticForValueType(t *testing.T) {
	cases := []struct {
		in   workflow.ValueType
		want SemanticType
	}{
		{workflow.TypeEnum, SemanticEnum},
		{workflow.TypeInt, SemanticInteger},
		{workflow.TypeBool, SemanticBoolean},
		{workflow.TypeDate, SemanticDate},
		{workflow.TypeTimestamp, SemanticDateTime},
		{workflow.TypeRecurrence, SemanticRecurrence},
		{workflow.TypeListString, SemanticStringList},
		{workflow.TypeListRef, SemanticTikiIDList},
		{workflow.TypeUser, SemanticUser},
		{workflow.TypeString, SemanticText},
		{workflow.TypeID, SemanticText},
		{workflow.TypeRef, SemanticText},
		{workflow.TypeDuration, SemanticText},
	}
	for _, c := range cases {
		if got := semanticForValueType(c.in); got != c.want {
			t.Errorf("semanticForValueType(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestRenderDateValue_EmptyReadsPlaceholderSource confirms the date renderer
// draws the resolved placeholder (not a hardcoded literal), so it stays in
// lockstep with the measure. Also exercises the gridlayout import indirectly
// via ctx.Display default.
func TestRenderDateValue_EmptyReadsPlaceholderSource(t *testing.T) {
	const fieldName = "when"
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: fieldName, Type: workflow.TypeDate, Custom: true},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles(), FieldName: fieldName}
	got := strings.TrimSpace(extractTextView(renderDateValue(tk, ctx), true))
	if got != emptyPlaceholder(fieldName, SemanticDate) {
		t.Errorf("renderDateValue empty = %q, want placeholder %q", got, emptyPlaceholder(fieldName, SemanticDate))
	}
}
