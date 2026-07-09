package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// textGetter is satisfied by the tview.TextView the value renderers return,
// letting the test read rendered text without depending on gridbox's unexported
// truncatingTextView type.
type textGetter interface {
	GetText(stripTags bool) string
}

func TestRenderBooleanValue(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "blocked", Type: workflow.TypeBool},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("BOOL01")
	tk.Set("blocked", true)
	ctx := FieldRenderContext{FieldName: "blocked", Roles: theme.Roles()}
	p := renderBooleanValue(tk, ctx)
	tv, ok := p.(textGetter)
	if !ok {
		t.Fatalf("renderBooleanValue = %T, want a text view", p)
	}
	if !strings.Contains(tv.GetText(true), "true") {
		t.Fatalf("rendered %q, want to contain \"true\"", tv.GetText(true))
	}

	// absent → "false" (booleans default false, never empty)
	tk2 := tikipkg.New()
	tk2.SetID("BOOL0A")
	p2 := renderBooleanValue(tk2, ctx)
	tv2, _ := p2.(textGetter)
	if !strings.Contains(tv2.GetText(true), "false") {
		t.Fatalf("absent bool rendered %q, want \"false\"", tv2.GetText(true))
	}
}

func TestEditBooleanValue_Toggles(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "blocked", Type: workflow.TypeBool},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("BOOL02") // no value → defaults to false
	ctx := FieldRenderContext{FieldName: "blocked", Roles: theme.Roles()}
	w := editBooleanValue(tk, ctx, func(string) {})
	if w.GetText() != "false" {
		t.Fatalf("default GetText=%q want false", w.GetText())
	}
	if !w.CycleValue(1) {
		t.Fatal("bool editor should be cyclable")
	}
	if w.GetText() != "true" {
		t.Fatalf("after cycle GetText=%q want true", w.GetText())
	}
	w.CycleValue(1)
	if w.GetText() != "false" {
		t.Fatalf("after second cycle GetText=%q want false", w.GetText())
	}
}

func TestFieldHasEditor_Boolean(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "blocked", Type: workflow.TypeBool},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	if !FieldHasEditor("blocked") {
		t.Fatal("boolean field should now be editable")
	}
}
