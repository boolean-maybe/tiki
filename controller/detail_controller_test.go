package controller

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/boolean-maybe/tiki/gridlayout"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// newTestDraftTiki returns a fresh in-memory draft tiki for use in
// ApplyDetailMode tests. The draft is not persisted to any store.
func newTestDraftTiki(id string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = id
	return tk
}

// newTestDetailPlugin builds a DetailPlugin fixture with the given metadata
// and per-view actions.
func newTestDetailPlugin(metadata []string, actions []plugin.PluginAction) *plugin.DetailPlugin {
	var spec gridlayout.GridSpec
	if len(metadata) > 0 {
		s, err := gridlayout.ParseGrid([][]string{metadata})
		if err != nil {
			panic(fmt.Sprintf("newTestDetailPlugin: invalid metadata %v: %v", metadata, err))
		}
		spec = s
	}
	return &plugin.DetailPlugin{
		BasePlugin: plugin.BasePlugin{
			Name:        "Detail",
			Kind:        plugin.KindDetail,
			ConfigIndex: 0,
		},
		Layout:  spec,
		Actions: actions,
	}
}

// TestDetailController_RegistryHasFullscreenAndStubEdit asserts the built-in
// detail action registry surfaces the actions Phase 1 promises (Fullscreen
// always available; the 'e' binding reserved). Phase 2 replaced the stub
// with a real ActionDetailEdit; the assertion still anchors on the 'e'
// binding being present.
func TestDetailController_RegistryHasFullscreenAndStubEdit(t *testing.T) {
	pluginDef := newTestDetailPlugin([]string{"status"}, nil)
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	nav := newMockNavigationController()

	dc := NewDetailController(pluginDef, nav, nil, tikiStore, gate, rukiRuntime.NewSchema(), nil)

	r := dc.GetActionRegistry()
	if r.MatchBinding(tcell.KeyRune, 'f', 0) == nil {
		t.Error("expected Fullscreen action on 'f'")
	}
	editAct := r.MatchBinding(tcell.KeyRune, 'e', 0)
	if editAct == nil {
		t.Fatal("expected Edit action on 'e'")
	}
	if editAct.ID != ActionDetailEdit {
		t.Errorf("expected ActionDetailEdit, got %q", editAct.ID)
	}
}

// TestDetailController_SurfacesPerViewActions asserts the controller registers
// configured per-view actions onto the registry.
func TestDetailController_SurfacesPerViewActions(t *testing.T) {
	actions := []plugin.PluginAction{
		{
			Key:        tcell.KeyRune,
			Rune:       'a',
			KeyStr:     "a",
			Label:      "Assign me",
			Kind:       plugin.ActionKindView,
			TargetView: "Backlog",
		},
	}
	pluginDef := newTestDetailPlugin([]string{"status"}, actions)
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	nav := newMockNavigationController()

	dc := NewDetailController(pluginDef, nav, nil, tikiStore, gate, rukiRuntime.NewSchema(), nil)

	if act := dc.GetActionRegistry().MatchBinding(tcell.KeyRune, 'a', 0); act == nil {
		t.Fatal("expected per-view action to be registered on 'a'")
	} else if act.Label != "Assign me" {
		t.Errorf("Label = %q, want %q", act.Label, "Assign me")
	}
}

// TestDetailController_SetSelectedTikiID exercises the SelectableView contract
// — the selection set on the controller must round-trip through GetSelectedID.
func TestDetailController_SetSelectedTikiID(t *testing.T) {
	pluginDef := newTestDetailPlugin([]string{"status"}, nil)
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	nav := newMockNavigationController()

	dc := NewDetailController(pluginDef, nav, nil, tikiStore, gate, rukiRuntime.NewSchema(), nil)

	dc.SetSelectedTikiID("TIKI001")
	if dc.selectedTikiID != "TIKI001" {
		t.Errorf("selectedTikiID = %q, want %q", dc.selectedTikiID, "TIKI001")
	}
}

// TestDetailController_ShowNavigationFalse asserts detail views don't surface
// plugin nav arrow keys.
func TestDetailController_ShowNavigationFalse(t *testing.T) {
	pluginDef := newTestDetailPlugin(nil, nil)
	dc := NewDetailController(pluginDef, newMockNavigationController(), nil, nil, nil, nil, nil)
	if dc.ShowNavigation() {
		t.Error("ShowNavigation() = true, want false for kind: detail")
	}
}

// TestApplyDetailMode_View_NoOp pins that view mode (and the empty default)
// leaves the view in read-only mode and just returns true.
func TestApplyDetailMode_View_NoOp(t *testing.T) {
	dc, view, _, _ := newDetailEditTestRig(t)
	if !dc.ApplyDetailMode(plugin.DetailModeView, "", nil) {
		t.Fatal("ApplyDetailMode returned false for view mode")
	}
	if view.editing {
		t.Error("view mode should not enter edit mode")
	}
}

// TestApplyDetailMode_Edit_EntersEditMode pins that edit mode flips the
// bound view into in-place edit mode against the carried selection.
func TestApplyDetailMode_Edit_EntersEditMode(t *testing.T) {
	dc, view, _, _ := newDetailEditTestRig(t)
	if !dc.ApplyDetailMode(plugin.DetailModeEdit, "", nil) {
		t.Fatal("ApplyDetailMode returned false for edit mode")
	}
	if !view.editing {
		t.Error("edit mode should enter edit mode")
	}
}

// TestApplyDetailMode_New_RequiresDraft pins that new mode without a draft
// is rejected — the dispatcher is responsible for synthesizing one before
// pushing the view.
func TestApplyDetailMode_New_RequiresDraft(t *testing.T) {
	dc, _, _, _ := newDetailEditTestRig(t)
	if dc.ApplyDetailMode(plugin.DetailModeNew, "", nil) {
		t.Fatal("ApplyDetailMode should refuse new mode without a draft")
	}
}

// TestApplyDetailMode_New_SetsDraftAndFocusesTitle pins that new mode
// adopts the carried draft into the edit session and enters edit mode
// with title focused.
func TestApplyDetailMode_New_SetsDraftAndFocusesTitle(t *testing.T) {
	dc, view, session, _ := newDetailEditTestRig(t)
	draft := newTestDraftTiki("DRAFT1")
	if !dc.ApplyDetailMode(plugin.DetailModeNew, "", draft) {
		t.Fatal("ApplyDetailMode returned false for new mode")
	}
	if got := session.GetDraftTiki(); got == nil || got.ID != "DRAFT1" {
		t.Errorf("draft not set on session: got %+v", got)
	}
	if !view.editing {
		t.Error("new mode should enter edit mode")
	}
	if view.focusField != model.EditFieldTitle {
		t.Errorf("focus = %q, want %q", view.focusField, model.EditFieldTitle)
	}
}

// TestApplyDetailMode_EditDesc_InstallsDescRegistry pins that edit-desc
// mode swaps in the description-only registry on the view.
func TestApplyDetailMode_EditDesc_InstallsDescRegistry(t *testing.T) {
	dc, view, _, _ := newDetailEditTestRig(t)
	if !dc.ApplyDetailMode(plugin.DetailModeEditDesc, "", nil) {
		t.Fatal("ApplyDetailMode returned false for edit-desc")
	}
	if !view.editing {
		t.Error("edit-desc should enter edit mode")
	}
	if view.registry == nil {
		t.Fatal("edit-desc did not install a registry on the view")
	}
	if view.registry.GetByID(ActionSaveTiki) == nil {
		t.Error("edit-desc registry should contain ActionSaveTiki")
	}
	if view.registry.GetByID(ActionNextField) != nil {
		t.Error("edit-desc registry should NOT contain ActionNextField (no field nav)")
	}
	if view.focusField != model.EditFieldDescription {
		t.Errorf("focus = %q, want %q", view.focusField, model.EditFieldDescription)
	}
}

// TestApplyDetailMode_EditTags_InstallsTagsRegistry pins that edit-tags
// mode swaps in the tags-only registry on the view.
func TestApplyDetailMode_EditTags_InstallsTagsRegistry(t *testing.T) {
	dc, view, _, _ := newDetailEditTestRig(t)
	if !dc.ApplyDetailMode(plugin.DetailModeEditTags, "", nil) {
		t.Fatal("ApplyDetailMode returned false for edit-tags")
	}
	if !view.editing {
		t.Error("edit-tags should enter edit mode")
	}
	if view.registry == nil {
		t.Fatal("edit-tags did not install a registry on the view")
	}
	if view.registry.GetByID(ActionSaveTiki) == nil {
		t.Error("edit-tags registry should contain ActionSaveTiki")
	}
	if view.registry.GetByID(ActionNextField) != nil {
		t.Error("edit-tags registry should NOT contain ActionNextField (no field nav)")
	}
	if view.focusField != model.EditFieldTags {
		t.Errorf("focus = %q, want %q", view.focusField, model.EditFieldTags)
	}
}

// TestDetailController_RukiChooseActionRoundTrip exercises the full
// choose() pipeline for a kind: ruki action declared on a kind: detail
// view. Such actions used to be silently dropped at registration; this
// test pins that they (a) register, (b) advertise themselves to the input
// router as choose-driven, (c) evaluate their choose filter, and (d)
// commit the executor result through the mutation gate when handed a
// chosen tiki id.
func TestDetailController_RukiChooseActionRoundTrip(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	// the project that will receive the dependency
	project := tikipkg.New()
	project.ID = "PROJ01"
	project.Title = "Sample Project"
	project.Set("type", "project")
	project.Set("status", "ready")
	if err := tikiStore.CreateTiki(project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	// a candidate tiki that the choose filter should surface
	seedTiki(t, tikiStore, "TASK01", "Candidate Task", "ready", 0)

	stmt := mustParseStmt(t,
		`update where id = id() set dependsOn = dependsOn + choose(select where has(type) and type != "project" and id not in outer.dependsOn)`)
	if !stmt.UsesChooseBuiltin() {
		t.Fatal("test fixture statement does not use choose() — adjust the ruki text")
	}

	action := plugin.PluginAction{
		Key:          tcell.KeyRune,
		Rune:         'a',
		KeyStr:       "a",
		Label:        "Add to project",
		Kind:         plugin.ActionKindRuki,
		Action:       stmt,
		HasChoose:    true,
		ChooseFilter: stmt.ChooseFilter(),
	}
	pluginDef := newTestDetailPlugin([]string{"status"}, []plugin.PluginAction{action})

	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	nav := newMockNavigationController()
	dc := NewDetailController(pluginDef, nav, nil, tikiStore, gate, rukiRuntime.NewSchema(), nil)
	dc.SetSelectedTikiID("PROJ01")

	// (a) registered
	if dc.GetActionRegistry().MatchBinding(tcell.KeyRune, 'a', 0) == nil {
		t.Fatal("expected HasChoose ruki action to register on 'a'")
	}

	actionID := pluginActionID("a")

	// (b) recognised as choose-driven by the input router contract
	label, ok := dc.GetActionChooseSpec(actionID)
	if !ok {
		t.Fatal("GetActionChooseSpec returned false for HasChoose ruki action")
	}
	if label != "Add to project" {
		t.Errorf("label = %q, want %q", label, "Add to project")
	}

	// (c) candidate set comes from the choose filter
	_, candidates, ok := dc.CanStartActionChoose(actionID)
	if !ok {
		t.Fatal("CanStartActionChoose returned false")
	}
	foundCandidate := false
	for _, c := range candidates {
		if c.ID == "TASK01" {
			foundCandidate = true
			break
		}
	}
	if !foundCandidate {
		t.Errorf("expected TASK01 in choose candidates, got %d candidates", len(candidates))
	}

	// (d) handing back the chosen id mutates the project's dependsOn
	if !dc.HandleActionChoose(actionID, "TASK01") {
		t.Fatal("HandleActionChoose returned false")
	}
	updated := tikiStore.GetTiki("PROJ01")
	if updated == nil {
		t.Fatal("re-read project: nil")
	}
	deps, ok, _ := updated.StringSliceField(tikipkg.FieldDependsOn)
	if !ok {
		t.Fatal("project.dependsOn not present after HandleActionChoose")
	}
	if len(deps) != 1 || deps[0] != "TASK01" {
		t.Errorf("project.dependsOn = %v, want [TASK01]", deps)
	}
}
