package tikidetail

import (
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/ruki/recurrence"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestEditDateValueUsesConfiguredField(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "deadline", Type: workflow.TypeDate},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.Set("due", time.Date(2026, time.July, 11, 0, 0, 0, 0, time.UTC))
	tk.Set("deadline", time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC))

	editor := buildFieldEditor(
		"deadline",
		tk,
		FieldRenderContext{FieldName: "deadline", Roles: theme.Roles()},
		func(string) {},
	)
	if editor == nil {
		t.Fatal("date editor is nil")
	}
	if got := editor.GetText(); got != "2026-07-12" {
		t.Fatalf("date editor seeded %q, want 2026-07-12", got)
	}
}

func TestEditDateValueDoesNotDependOnOtherFields(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "deadline", Type: workflow.TypeDate},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.Set("recurrence", string(recurrence.RecurrenceDaily))

	editor := buildFieldEditor(
		"deadline",
		tk,
		FieldRenderContext{FieldName: "deadline", Roles: theme.Roles()},
		func(string) {},
	)
	if !editor.CycleValue(1) {
		t.Fatal("date editor unexpectedly became read-only")
	}
}

func TestRenderConfiguredFieldUsesDeclaredTypeForFormerDateName(t *testing.T) {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{Name: "due", Type: workflow.TypeString},
	})
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.Set("due", "tomorrow")

	row := renderConfiguredField(
		"due",
		tk,
		FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()},
	)
	textRow, ok := row.(interface{ GetText(bool) string })
	if !ok {
		t.Fatalf("rendered row %T does not expose text", row)
	}
	if got := textRow.GetText(false); !strings.Contains(got, "tomorrow") {
		t.Fatalf("rendered row %q does not contain declared text value", got)
	}
}
