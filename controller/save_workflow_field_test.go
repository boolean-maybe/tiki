package controller

import (
	"testing"
	"time"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
	"github.com/boolean-maybe/tiki/workflow/value"
)

// TestTikiEditSession_SaveWorkflowField pins the generic scalar save path for
// catalog-only fields (text/integer/boolean/datetime) that have no dedicated
// Save* method. Before this method a custom datetime field was editable in the
// UI but its edits were silently dropped on commit (no save handler installed).
func TestTikiEditSession_SaveWorkflowField(t *testing.T) {
	teststatuses.Init()
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "note", Type: workflow.TypeString},
		{Name: "reviewer", Type: workflow.TypeUser},
		{Name: "estimate", Type: workflow.TypeInt},
		{Name: "blocked", Type: workflow.TypeBool},
		{Name: "dueBy", Type: workflow.TypeTimestamp},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	cases := []struct {
		field, raw string
		wantOK     bool
		wantStored interface{} // nil = field deleted/absent
	}{
		{"note", "hello", true, "hello"},
		{"note", "", true, nil},
		{"reviewer", "alice", true, "alice"},
		{"reviewer", "Unassigned", true, "Unassigned"},
		{"reviewer", "", true, nil},
		{"estimate", "240", true, 240},
		{"estimate", "-5", true, -5}, // unbounded
		{"estimate", "x", false, nil},
		{"estimate", "", true, nil},
		{"blocked", "true", true, true},
		{"blocked", "false", true, false},
		{"blocked", "", true, nil},
		{"dueBy", "2026-07-08 14:30", true, "2026-07-08 14:30"}, // stored as time.Time; check via format
		{"dueBy", "", true, nil},
		{"dueBy", "garbage", false, nil},
	}
	for _, c := range cases {
		t.Run(c.field+"="+c.raw, func(t *testing.T) {
			s := store.NewInMemoryStore()
			gate := service.NewTikiMutationGate()
			gate.SetStore(s)
			tc := NewTikiEditSession(s, gate, newMockNavigationController(), nil)
			tk := tikipkg.New()
			tk.SetID("SAVE01")
			tk.SetTitle("t")
			tc.SetDraft(tk)

			got := tc.SaveWorkflowField(c.field, c.raw)
			if got != c.wantOK {
				t.Fatalf("SaveWorkflowField(%q,%q)=%v want %v", c.field, c.raw, got, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			v, present := tc.draftTiki.Get(c.field)
			if c.wantStored == nil {
				if present {
					t.Fatalf("expected %s absent, got %v", c.field, v)
				}
				return
			}
			if c.field == "dueBy" {
				tm, _ := v.(time.Time)
				if got := value.FormatDateTime(tm); got != c.wantStored {
					t.Fatalf("dueBy stored %q want %q", got, c.wantStored)
				}
				return
			}
			if v != c.wantStored {
				t.Fatalf("%s stored %v want %v", c.field, v, c.wantStored)
			}
		})
	}
}

func TestWireEditFieldHandlers_UserPersists(t *testing.T) {
	teststatuses.Init()
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "reviewer", Type: workflow.TypeUser},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	nav := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, nav, nil)

	tk := tikipkg.New()
	tk.SetID("TIKIUS")
	tk.SetTitle("Test")
	tk.Set(tikipkg.FieldStatus, "ready")
	if err := tikiStore.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	pluginDef := newTestDetailPlugin([]string{"status", "reviewer"}, nil)
	dc := NewDetailController(pluginDef, nav, nil, tikiStore, gate, rukiRuntime.NewSchema(), tc)
	dc.SetSelectedTikiID(tk.ID())

	view := newFakeDetailEditView()
	view.layout = []string{"status", "reviewer"}
	dc.BindEditView(view)

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	saver, ok := view.fieldHandlers["reviewer"]
	if !ok || saver == nil {
		t.Fatal("reviewer save handler not installed by controller")
	}
	saver("alice")
	if !dc.HandleAction(ActionDetailSave) {
		t.Fatal("ActionDetailSave returned false")
	}

	got := tikiStore.GetTiki("TIKIUS")
	if got == nil {
		t.Fatal("tiki disappeared from store after save")
	}
	if reviewer, _, _ := got.StringField("reviewer"); reviewer != "alice" {
		t.Fatalf("reviewer persisted as %q, want alice", reviewer)
	}
}

// TestWireEditFieldHandlers_CatalogDatetimePersists reproduces the silent-drop
// bug: a catalog-only datetime field was editable but had no save handler, so
// its edits vanished on commit. After wiring SaveWorkflowField generically, the
// handler exists and persists.
func TestWireEditFieldHandlers_CatalogDatetimePersists(t *testing.T) {
	teststatuses.Init()
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "dueBy", Type: workflow.TypeTimestamp},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	nav := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, nav, nil)

	tk := tikipkg.New()
	tk.SetID("TIKIDT1")
	tk.SetTitle("Test")
	tk.Set(tikipkg.FieldStatus, "ready")
	if err := tikiStore.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	pluginDef := newTestDetailPlugin([]string{"status", "dueBy"}, nil)
	dc := NewDetailController(pluginDef, nav, nil, tikiStore, gate, rukiRuntime.NewSchema(), tc)
	dc.SetSelectedTikiID(tk.ID())

	view := newFakeDetailEditView()
	view.layout = []string{"status", "dueBy"}
	dc.BindEditView(view)

	if !dc.HandleAction(ActionDetailEdit) {
		t.Fatal("EnterEditMode")
	}
	saver, ok := view.fieldHandlers["dueBy"]
	if !ok || saver == nil {
		t.Fatal("dueBy save handler not installed by controller (silent-drop bug)")
	}
	saver("2026-07-08 14:30")
	if !dc.HandleAction(ActionDetailSave) {
		t.Fatal("ActionDetailSave returned false")
	}

	got := tikiStore.GetTiki("TIKIDT1")
	if got == nil {
		t.Fatal("tiki disappeared from store after save")
	}
	stored, _, _ := got.TimeField("dueBy")
	if stored.IsZero() {
		t.Fatal("dueBy not persisted (silent-drop)")
	}
	if formatted := value.FormatDateTime(stored); formatted != "2026-07-08 14:30" {
		t.Fatalf("dueBy persisted as %q, want %q", formatted, "2026-07-08 14:30")
	}
}
