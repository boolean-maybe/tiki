package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestEditStringListValueUsesConfiguredField(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "labels", Type: workflow.TypeListString},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.Set("tags", []string{"wrong"})
	tk.Set("labels", []string{"alpha", "beta"})

	editor := buildFieldEditor(
		"labels",
		tk,
		FieldRenderContext{FieldName: "labels", Roles: theme.Roles()},
		func(string) {},
	)
	if editor == nil {
		t.Fatal("string-list editor is nil")
	}
	if got := editor.GetText(); got != "alpha beta" {
		t.Fatalf("string-list editor seeded %q, want alpha beta", got)
	}
}

func TestRenderConfiguredFieldUsesDeclaredTypeForFormerStringListName(t *testing.T) {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{Name: "tags", Type: workflow.TypeString},
	})
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.Set("tags", "plain text")

	row := renderConfiguredField(
		"tags",
		tk,
		FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()},
	)
	textRow, ok := row.(interface{ GetText(bool) string })
	if !ok {
		t.Fatalf("rendered row %T does not expose text", row)
	}
	if got := textRow.GetText(false); !strings.Contains(got, "plain text") {
		t.Fatalf("rendered row %q does not contain declared text value", got)
	}
}

func TestStringListHeightUsesConfiguredField(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "labels", Type: workflow.TypeListString},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.Set("labels", []string{"alpha", "beta"})

	if got := FieldHeight("labels", tk, 5); got != 2 {
		t.Fatalf("string-list height = %d, want 2", got)
	}
}
