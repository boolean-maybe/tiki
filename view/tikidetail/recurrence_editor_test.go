package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/ruki/recurrence"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestEditRecurrenceValueUsesConfiguredField(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "schedule", Type: workflow.TypeRecurrence},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.Set("recurrence", string(recurrence.RecurrenceDaily))
	tk.Set("schedule", "0 0 * * MON")

	editor := buildFieldEditor(
		"schedule",
		tk,
		FieldRenderContext{FieldName: "schedule", Roles: theme.Roles()},
		func(string) {},
	)
	if editor == nil {
		t.Fatal("recurrence editor is nil")
	}
	if got := editor.GetText(); got != "0 0 * * MON" {
		t.Fatalf("recurrence editor seeded %q, want weekly Monday", got)
	}
}

func TestRenderConfiguredFieldUsesDeclaredTypeForFormerRecurrenceName(t *testing.T) {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{Name: "recurrence", Type: workflow.TypeString},
	})
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.Set("recurrence", "later")

	row := renderConfiguredField(
		"recurrence",
		tk,
		FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()},
	)
	textRow, ok := row.(interface{ GetText(bool) string })
	if !ok {
		t.Fatalf("rendered row %T does not expose text", row)
	}
	if got := textRow.GetText(false); !strings.Contains(got, "later") {
		t.Fatalf("rendered row %q does not contain declared text value", got)
	}
}

func TestRecurrencePartNavigationUsesFocusedFieldType(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "schedule", Type: workflow.TypeRecurrence},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	s := store.NewInMemoryStore()
	tk := newTestViewTiki("RECT01")
	tk.Set("schedule", "0 0 * * MON")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	cv := NewConfigurableDetailView(
		s,
		tk.ID(),
		detailPluginFromFields([]string{"status", "schedule"}),
		controller.DetailViewActions(),
		nil,
		nil,
		nil,
		nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())
	if !cv.EnterEditModeWithFocus(model.EditField("schedule")) {
		t.Fatal("EnterEditModeWithFocus(schedule) returned false")
	}

	ctx := FieldRenderContext{Mode: RenderModeEdit, Roles: theme.Roles(), FieldName: "schedule"}
	editor := buildFieldEditor("schedule", tk, ctx, cv.onEditFieldChange["schedule"])
	if editor == nil {
		t.Fatal("schedule editor is nil")
	}
	cv.editors["schedule"] = editor

	if !cv.MoveRecurrencePartRight() {
		t.Fatal("MoveRecurrencePartRight returned false for recurrence-typed field")
	}
	if !cv.IsRecurrenceValueFocused() {
		t.Fatal("recurrence value is not focused after moving right")
	}
}
