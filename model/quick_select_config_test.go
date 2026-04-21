package model

import "testing"

func TestQuickSelectConfig_VisibilityToggles(t *testing.T) {
	qc := NewQuickSelectConfig()
	if qc.IsVisible() {
		t.Fatal("should start hidden")
	}
	qc.SetVisible(true)
	if !qc.IsVisible() {
		t.Fatal("should be visible after SetVisible(true)")
	}
	qc.SetVisible(false)
	if qc.IsVisible() {
		t.Fatal("should be hidden after SetVisible(false)")
	}
}

func TestQuickSelectConfig_ListenerNotified(t *testing.T) {
	qc := NewQuickSelectConfig()
	called := 0
	qc.AddListener(func() { called++ })
	qc.SetVisible(true)
	if called != 1 {
		t.Fatalf("expected 1 call, got %d", called)
	}
	qc.SetVisible(false)
	if called != 2 {
		t.Fatalf("expected 2 calls, got %d", called)
	}
	// no-op (already false)
	qc.SetVisible(false)
	if called != 2 {
		t.Fatalf("expected 2 calls (no change), got %d", called)
	}
}

func TestQuickSelectConfig_SelectInvokesCallbackAndHides(t *testing.T) {
	qc := NewQuickSelectConfig()
	var selectedID string
	qc.SetOnSelect(func(id string) { selectedID = id })
	qc.SetVisible(true)

	qc.Select("TIKI-000001")
	if selectedID != "TIKI-000001" {
		t.Fatalf("expected TIKI-000001, got %q", selectedID)
	}
	if qc.IsVisible() {
		t.Fatal("should be hidden after Select")
	}
}

func TestQuickSelectConfig_CancelInvokesCallbackAndHides(t *testing.T) {
	qc := NewQuickSelectConfig()
	cancelled := false
	qc.SetOnCancel(func() { cancelled = true })
	qc.SetVisible(true)

	qc.Cancel()
	if !cancelled {
		t.Fatal("cancel callback not invoked")
	}
	if qc.IsVisible() {
		t.Fatal("should be hidden after Cancel")
	}
}

func TestQuickSelectConfig_RemoveListener(t *testing.T) {
	qc := NewQuickSelectConfig()
	called := 0
	id := qc.AddListener(func() { called++ })
	qc.SetVisible(true)
	if called != 1 {
		t.Fatalf("expected 1, got %d", called)
	}
	qc.RemoveListener(id)
	qc.SetVisible(false)
	if called != 1 {
		t.Fatalf("expected still 1 after remove, got %d", called)
	}
}
