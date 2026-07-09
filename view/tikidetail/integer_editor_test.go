package tikidetail

import (
	"testing"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestEditIntegerValue(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "estimate", Type: workflow.TypeInt},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("INT001")
	tk.Set("estimate", 42)
	ctx := FieldRenderContext{FieldName: "estimate", Roles: theme.Roles()}
	w := editIntegerValue(tk, ctx, func(string) {})
	if w == nil {
		t.Fatal("editIntegerValue nil")
	}
	if w.GetText() != "42" {
		t.Fatalf("GetText=%q want 42", w.GetText())
	}
	if w.CycleValue(1) {
		t.Fatal("integer editor should not be cyclable (free-type)")
	}
	// digit filter: an empty tiki opens blank, not "0"
	tk2 := tikipkg.New()
	tk2.SetID("INT002")
	w2 := editIntegerValue(tk2, ctx, func(string) {})
	if w2.GetText() != "" {
		t.Fatalf("empty int GetText=%q want empty", w2.GetText())
	}
}

func TestAcceptSignedInteger(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"-", true},
		{"5", true},
		{"-5", true},
		{"123", true},
		{"12a", false},
		{"1.5", false},
		{"--5", false},
	}
	for _, c := range cases {
		if got := acceptSignedInteger(c.in, 0); got != c.want {
			t.Errorf("acceptSignedInteger(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFieldHasEditor_Integer(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "estimate", Type: workflow.TypeInt},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	if !FieldHasEditor("estimate") {
		t.Fatal("integer field should now be editable")
	}
}
