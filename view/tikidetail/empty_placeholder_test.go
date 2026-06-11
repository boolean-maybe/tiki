package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"

	"github.com/rivo/tview"
)

// TestMeasureFieldValue_EmptyDateMatchesRenderedWidth is the regression guard
// for the original bug: an empty date used to MEASURE the "—" sentinel (width 1)
// while it RENDERS "None" (width 4), so a content-sized column under-reserved and
// clipped to "N". The measure must equal the drawn placeholder width.
func TestMeasureFieldValue_EmptyDateMatchesRenderedWidth(t *testing.T) {
	tk := tikipkg.New() // due never set → empty date

	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles(), FieldName: tikipkg.FieldDue}
	rendered := extractTextView(renderDateValue(tk, ctx), true)
	if strings.TrimSpace(rendered) != "None" {
		t.Fatalf("empty due rendered %q, want %q", rendered, "None")
	}

	got := MeasureFieldValue(tikipkg.FieldDue, tk, ctx)
	want := tview.TaggedStringWidth(rendered)
	if got != want {
		t.Errorf("MeasureFieldValue(empty due) = %d, want %d (rendered width of %q)", got, want, rendered)
	}
	if got != 4 {
		t.Errorf("MeasureFieldValue(empty due) = %d, want 4 (len of \"None\")", got)
	}
}

// TestMeasureFieldValue_EmptyDateTimeMatchesRenderedWidth mirrors the date case
// for a TypeTimestamp field, whose empty placeholder is "Unknown" (width 7).
func TestMeasureFieldValue_EmptyDateTimeMatchesRenderedWidth(t *testing.T) {
	tk := tikipkg.New() // createdAt zero

	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles(), FieldName: "createdAt"}
	got := MeasureFieldValue("createdAt", tk, ctx)
	if got != len("Unknown") {
		t.Errorf("MeasureFieldValue(empty createdAt) = %d, want %d (len of \"Unknown\")", got, len("Unknown"))
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
	full.Set("count", 3)

	mustField := func(name string) workflow.FieldDef {
		fd, ok := workflow.Field(name)
		if !ok {
			t.Fatalf("field %q not registered", name)
		}
		return fd
	}

	emptyCases := []string{"when", "labels", "deps", "owner", "count"}
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
		{"per-field override beats type default", tikipkg.FieldType, SemanticEnum, "(none)"},
		{"per-type default when no override", tikipkg.FieldDue, SemanticDate, "None"},
		{"enum with no override falls to dash", tikipkg.FieldStatus, SemanticEnum, "─"},
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
	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles(), FieldName: tikipkg.FieldDue}
	got := strings.TrimSpace(extractTextView(renderDateValue(tk, ctx), true))
	if got != emptyPlaceholder(tikipkg.FieldDue, SemanticDate) {
		t.Errorf("renderDateValue empty = %q, want placeholder %q", got, emptyPlaceholder(tikipkg.FieldDue, SemanticDate))
	}
}
