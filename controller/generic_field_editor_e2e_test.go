package controller

import (
	"testing"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store/tikistore"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
	"github.com/boolean-maybe/tiki/workflow/value"
)

// TestGenericFieldEditors_EndToEndPersistToDisk drives the exact flow the
// running app would for each custom field type declared in a bug-tracker-style
// workflow: enter edit mode (which wires the generic SaveWorkflowField handlers
// via wireEditFieldHandlers), fire each field's handler with a new value,
// commit, then reload from DISK through a real TikiStore and assert every value
// round-tripped. This stands in for the interactive smoke test when the display
// is unavailable — it exercises the real controller + edit session + mutation
// gate + on-disk store, not mocks.
func TestGenericFieldEditors_EndToEndPersistToDisk(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// escalations (TypeInt) is already in the canonical test catalog; only the
	// other three custom types need declaring here.
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "reportedBy", Type: workflow.TypeString},
		{Name: "reviewer", Type: workflow.TypeUser},
		{Name: "dueBy", Type: workflow.TypeTimestamp},
		{Name: "regression", Type: workflow.TypeBool},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	dir := t.TempDir()
	tikiStore, err := tikistore.NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	nav := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, nav, nil)

	tk := tikipkg.New()
	tk.SetID("BUG001")
	tk.SetTitle("Login fails")
	tk.Set("status", "ready")
	if err := tikiStore.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	layout := []string{"status", "reportedBy", "reviewer", "dueBy", "regression", "escalations"}
	pluginDef := newTestDetailPlugin(layout, nil)
	dc := NewDetailController(pluginDef, nav, nil, nil, tikiStore, gate, rukiRuntime.NewSchema(), tc)
	dc.SetSelectedTikiID(tk.ID())

	view := newFakeDetailEditView()
	view.layout = layout
	dc.BindEditView(view)

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}

	// each custom field must have a handler installed by wireEditFieldHandlers.
	edits := map[string]string{
		"reportedBy":  "alice@example.com",
		"reviewer":    "not-in-suggestions",
		"dueBy":       "2026-07-10 09:00",
		"regression":  "true",
		"escalations": "5",
	}
	for name, raw := range edits {
		h, ok := view.fieldHandlers[name]
		if !ok || h == nil {
			t.Fatalf("no handler installed for %q (editable field must be wired)", name)
		}
		h(raw)
	}

	if !dc.HandleAction(ActionDetailSave) {
		t.Fatal("ActionDetailSave returned false")
	}

	// reload from disk — the true persistence check.
	if err := tikiStore.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	got := tikiStore.GetTiki("BUG001")
	if got == nil {
		t.Fatal("tiki missing after reload")
	}

	if v, _, _ := got.StringField("reportedBy"); v != "alice@example.com" {
		t.Errorf("reportedBy = %q, want alice@example.com", v)
	}
	if v, _, _ := got.StringField("reviewer"); v != "not-in-suggestions" {
		t.Errorf("reviewer = %q, want not-in-suggestions", v)
	}
	if dueBy, _, _ := got.TimeField("dueBy"); value.FormatDateTime(dueBy) != "2026-07-10 09:00" {
		t.Errorf("dueBy = %q, want 2026-07-10 09:00", value.FormatDateTime(dueBy))
	}
	if raw, ok := got.Get("regression"); !ok || raw != true {
		t.Errorf("regression = %v (ok=%v), want true", raw, ok)
	}
	if n, present, _ := got.IntField("escalations"); !present || n != 5 {
		t.Errorf("escalations = %d (present=%v), want 5", n, present)
	}
}
