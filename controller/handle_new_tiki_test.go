package controller

import (
	"reflect"
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
)

func TestHandleNewTiki_PassesDetailSpecInParams(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)

	spec := gridlayout.GridSpec{
		Rows: 1, Cols: 1,
		Anchors:   []gridlayout.Anchor{{Name: "title", Row: 0, Col: 0, RowSpan: 1, ColSpan: 1}},
		Stretcher: []bool{false},
		Cells:     [][]gridlayout.Cell{{gridlayout.FieldCell{Name: "title"}}},
	}
	SetDetailSpecSource(func() (gridlayout.GridSpec, bool) { return spec, true })
	defer SetDetailSpecSource(nil)

	pluginDef := &plugin.WorkflowPlugin{BasePlugin: plugin.BasePlugin{Name: "TestPlugin"}}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	nav := newMockNavigationController()
	pc := NewPluginController(tikiStore, gate, pluginConfig, pluginDef, nav, nil, rukiRuntime.NewSchema())

	if !pc.handleNewTiki() {
		t.Fatal("handleNewTiki returned false")
	}
	top := nav.CurrentView()
	if top == nil || top.ViewID != model.TikiEditViewID {
		t.Fatalf("want TikiEditViewID on stack, got %+v", top)
	}
	out := model.DecodeTikiEditParams(top.Params)
	if !reflect.DeepEqual(out.Spec, spec) {
		t.Fatalf("spec not propagated: %+v", out.Spec)
	}
}

func TestHandleNewTiki_NoDetailSpecReturnsFalse(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)

	SetDetailSpecSource(nil) // default returns (zero, false)

	pluginDef := &plugin.WorkflowPlugin{BasePlugin: plugin.BasePlugin{Name: "TestPlugin"}}
	pluginConfig := model.NewPluginConfig("TestPlugin")
	pc := NewPluginController(tikiStore, gate, pluginConfig, pluginDef, newMockNavigationController(), nil, rukiRuntime.NewSchema())

	if pc.handleNewTiki() {
		t.Fatal("handleNewTiki should return false when no Detail spec is registered")
	}
}
