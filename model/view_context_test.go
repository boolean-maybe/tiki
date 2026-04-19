package model

import (
	"sync/atomic"
	"testing"
)

func TestViewContext_SetFromView_SingleNotification(t *testing.T) {
	vc := NewViewContext()
	var count int32
	vc.AddListener(func() { atomic.AddInt32(&count, 1) })

	vc.SetFromView("plugin:Kanban", "Kanban", "desc", nil, nil)

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Errorf("expected exactly 1 notification, got %d", got)
	}
}

func TestViewContext_Getters(t *testing.T) {
	vc := NewViewContext()

	viewActions := []HeaderAction{{ID: "edit", Label: "Edit"}}
	pluginActions := []HeaderAction{{ID: "plugin:Kanban", Label: "Kanban"}}

	vc.SetFromView(TaskDetailViewID, "Tiki Detail", "desc", viewActions, pluginActions)

	if vc.GetViewID() != TaskDetailViewID {
		t.Errorf("expected %v, got %v", TaskDetailViewID, vc.GetViewID())
	}
	if vc.GetViewName() != "Tiki Detail" {
		t.Errorf("expected 'Tiki Detail', got %q", vc.GetViewName())
	}
	if vc.GetViewDescription() != "desc" {
		t.Errorf("expected 'desc', got %q", vc.GetViewDescription())
	}
	if len(vc.GetViewActions()) != 1 || vc.GetViewActions()[0].ID != "edit" {
		t.Errorf("unexpected view actions: %v", vc.GetViewActions())
	}
	if len(vc.GetPluginActions()) != 1 || vc.GetPluginActions()[0].ID != "plugin:Kanban" {
		t.Errorf("unexpected plugin actions: %v", vc.GetPluginActions())
	}
}

func TestViewContext_RemoveListener(t *testing.T) {
	vc := NewViewContext()
	var count int32
	id := vc.AddListener(func() { atomic.AddInt32(&count, 1) })

	vc.SetFromView("v1", "n", "d", nil, nil)
	if atomic.LoadInt32(&count) != 1 {
		t.Fatal("listener should have fired once")
	}

	vc.RemoveListener(id)
	vc.SetFromView("v2", "n", "d", nil, nil)
	if atomic.LoadInt32(&count) != 1 {
		t.Error("listener should not fire after removal")
	}
}

func TestViewContext_MultipleListeners(t *testing.T) {
	vc := NewViewContext()
	var a, b int32
	vc.AddListener(func() { atomic.AddInt32(&a, 1) })
	vc.AddListener(func() { atomic.AddInt32(&b, 1) })

	vc.SetFromView("v1", "n", "d", nil, nil)

	if atomic.LoadInt32(&a) != 1 {
		t.Errorf("listener A: expected 1, got %d", atomic.LoadInt32(&a))
	}
	if atomic.LoadInt32(&b) != 1 {
		t.Errorf("listener B: expected 1, got %d", atomic.LoadInt32(&b))
	}
}

func TestViewContext_ZeroValueIsEmpty(t *testing.T) {
	vc := NewViewContext()

	if vc.GetViewID() != "" {
		t.Errorf("expected empty view ID, got %v", vc.GetViewID())
	}
	if vc.GetViewName() != "" {
		t.Errorf("expected empty view name, got %q", vc.GetViewName())
	}
	if vc.GetViewActions() != nil {
		t.Errorf("expected nil view actions, got %v", vc.GetViewActions())
	}
	if vc.GetPluginActions() != nil {
		t.Errorf("expected nil plugin actions, got %v", vc.GetPluginActions())
	}
}
