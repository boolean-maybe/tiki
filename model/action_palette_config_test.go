package model

import (
	"testing"
)

func TestActionPaletteConfig_DefaultHidden(t *testing.T) {
	pc := NewActionPaletteConfig()
	if pc.IsVisible() {
		t.Error("palette should be hidden by default")
	}
}

func TestActionPaletteConfig_SetVisible(t *testing.T) {
	pc := NewActionPaletteConfig()
	pc.SetVisible(true)
	if !pc.IsVisible() {
		t.Error("palette should be visible after SetVisible(true)")
	}
	pc.SetVisible(false)
	if pc.IsVisible() {
		t.Error("palette should be hidden after SetVisible(false)")
	}
}

func TestActionPaletteConfig_ToggleVisible(t *testing.T) {
	pc := NewActionPaletteConfig()
	pc.ToggleVisible()
	if !pc.IsVisible() {
		t.Error("palette should be visible after first toggle")
	}
	pc.ToggleVisible()
	if pc.IsVisible() {
		t.Error("palette should be hidden after second toggle")
	}
}

func TestActionPaletteConfig_ListenerNotifiedOnChange(t *testing.T) {
	pc := NewActionPaletteConfig()
	called := 0
	pc.AddListener(func() { called++ })

	pc.SetVisible(true)
	if called != 1 {
		t.Errorf("listener should be called once on change, got %d", called)
	}

	// no-op (already visible)
	pc.SetVisible(true)
	if called != 1 {
		t.Errorf("listener should not be called on no-op SetVisible, got %d", called)
	}

	pc.SetVisible(false)
	if called != 2 {
		t.Errorf("listener should be called on hide, got %d", called)
	}
}

func TestActionPaletteConfig_ToggleAlwaysNotifies(t *testing.T) {
	pc := NewActionPaletteConfig()
	called := 0
	pc.AddListener(func() { called++ })

	pc.ToggleVisible()
	pc.ToggleVisible()
	if called != 2 {
		t.Errorf("expected 2 notifications from toggle, got %d", called)
	}
}

func TestActionPaletteConfig_RemoveListener(t *testing.T) {
	pc := NewActionPaletteConfig()
	called := 0
	id := pc.AddListener(func() { called++ })

	pc.SetVisible(true)
	if called != 1 {
		t.Errorf("expected 1 call, got %d", called)
	}

	pc.RemoveListener(id)
	pc.SetVisible(false)
	if called != 1 {
		t.Errorf("expected no more calls after removal, got %d", called)
	}
}
