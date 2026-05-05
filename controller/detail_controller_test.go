package controller

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
)

// newTestDetailPlugin builds a DetailPlugin fixture with the given metadata
// and per-view actions.
func newTestDetailPlugin(metadata []string, actions []plugin.PluginAction) *plugin.DetailPlugin {
	return &plugin.DetailPlugin{
		BasePlugin: plugin.BasePlugin{
			Name:        "Detail",
			Kind:        plugin.KindDetail,
			ConfigIndex: -1,
		},
		Metadata: metadata,
		Actions:  actions,
	}
}

// TestDetailController_RegistryHasFullscreenAndStubEdit asserts the built-in
// detail action registry surfaces the actions Phase 1 promises: Fullscreen
// always available, Edit registered as a stub so the keybinding is reserved.
func TestDetailController_RegistryHasFullscreenAndStubEdit(t *testing.T) {
	pluginDef := newTestDetailPlugin([]string{"status"}, nil)
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	nav := newMockNavigationController()

	dc := NewDetailController(pluginDef, nav, nil, taskStore, gate, rukiRuntime.NewSchema())

	r := dc.GetActionRegistry()
	if r.MatchBinding(tcell.KeyRune, 'f', 0) == nil {
		t.Error("expected Fullscreen action on 'f'")
	}
	if r.MatchBinding(tcell.KeyRune, 'e', 0) == nil {
		t.Error("expected Edit stub action on 'e'")
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
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	nav := newMockNavigationController()

	dc := NewDetailController(pluginDef, nav, nil, taskStore, gate, rukiRuntime.NewSchema())

	if act := dc.GetActionRegistry().MatchBinding(tcell.KeyRune, 'a', 0); act == nil {
		t.Fatal("expected per-view action to be registered on 'a'")
	} else if act.Label != "Assign me" {
		t.Errorf("Label = %q, want %q", act.Label, "Assign me")
	}
}

// TestDetailController_SetSelectedTaskID exercises the SelectableView contract
// — the selection set on the controller must round-trip through GetSelectedID.
func TestDetailController_SetSelectedTaskID(t *testing.T) {
	pluginDef := newTestDetailPlugin([]string{"status"}, nil)
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	nav := newMockNavigationController()

	dc := NewDetailController(pluginDef, nav, nil, taskStore, gate, rukiRuntime.NewSchema())

	dc.SetSelectedTaskID("TIKI001")
	if dc.selectedTaskID != "TIKI001" {
		t.Errorf("selectedTaskID = %q, want %q", dc.selectedTaskID, "TIKI001")
	}
}

// TestDetailController_ShowNavigationFalse asserts detail views don't surface
// plugin nav arrow keys.
func TestDetailController_ShowNavigationFalse(t *testing.T) {
	pluginDef := newTestDetailPlugin(nil, nil)
	dc := NewDetailController(pluginDef, newMockNavigationController(), nil, nil, nil, nil)
	if dc.ShowNavigation() {
		t.Error("ShowNavigation() = true, want false for kind: detail")
	}
}
