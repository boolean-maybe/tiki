package model

import (
	"sync"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/store"

	"github.com/gdamore/tcell/v2"
)

func TestNewHeaderConfig(t *testing.T) {
	hc := NewHeaderConfig()

	if hc == nil {
		t.Fatal("NewHeaderConfig() returned nil")
	}

	// Initial visibility should be true
	if !hc.IsVisible() {
		t.Error("initial IsVisible() = false, want true")
	}

	if !hc.GetUserPreference() {
		t.Error("initial GetUserPreference() = false, want true")
	}

	// Initial collections should be empty
	if len(hc.GetViewActions()) != 0 {
		t.Error("initial GetViewActions() should be empty")
	}

	if len(hc.GetPluginActions()) != 0 {
		t.Error("initial GetPluginActions() should be empty")
	}

	if hc.GetViewName() != "" {
		t.Error("initial GetViewName() should be empty")
	}

	if hc.GetViewDescription() != "" {
		t.Error("initial GetViewDescription() should be empty")
	}

	if len(hc.GetBurndown()) != 0 {
		t.Error("initial GetBurndown() should be empty")
	}
}

func TestHeaderConfig_ViewActions(t *testing.T) {
	hc := NewHeaderConfig()

	actions := []HeaderAction{
		{
			ID:           "action1",
			Key:          tcell.KeyEnter,
			Label:        "Enter",
			ShowInHeader: true,
		},
		{
			ID:           "action2",
			Key:          tcell.KeyEscape,
			Label:        "Esc",
			ShowInHeader: true,
		},
	}

	hc.SetViewActions(actions)

	got := hc.GetViewActions()
	if len(got) != 2 {
		t.Errorf("len(GetViewActions()) = %d, want 2", len(got))
	}

	if got[0].ID != "action1" {
		t.Errorf("ViewActions[0].ID = %q, want %q", got[0].ID, "action1")
	}

	if got[1].ID != "action2" {
		t.Errorf("ViewActions[1].ID = %q, want %q", got[1].ID, "action2")
	}
}

func TestHeaderConfig_PluginActions(t *testing.T) {
	hc := NewHeaderConfig()

	actions := []HeaderAction{
		{
			ID:           "plugin1",
			Rune:         '1',
			Label:        "Plugin 1",
			ShowInHeader: true,
		},
	}

	hc.SetPluginActions(actions)

	got := hc.GetPluginActions()
	if len(got) != 1 {
		t.Errorf("len(GetPluginActions()) = %d, want 1", len(got))
	}

	if got[0].ID != "plugin1" {
		t.Errorf("PluginActions[0].ID = %q, want %q", got[0].ID, "plugin1")
	}
}

func TestHeaderConfig_ViewInfo(t *testing.T) {
	hc := NewHeaderConfig()

	hc.SetViewInfo("Kanban", "Tasks moving through stages")

	if got := hc.GetViewName(); got != "Kanban" {
		t.Errorf("GetViewName() = %q, want %q", got, "Kanban")
	}

	if got := hc.GetViewDescription(); got != "Tasks moving through stages" {
		t.Errorf("GetViewDescription() = %q, want %q", got, "Tasks moving through stages")
	}

	// update overwrites
	hc.SetViewInfo("Backlog", "Upcoming tasks")

	if got := hc.GetViewName(); got != "Backlog" {
		t.Errorf("GetViewName() after update = %q, want %q", got, "Backlog")
	}

	if got := hc.GetViewDescription(); got != "Upcoming tasks" {
		t.Errorf("GetViewDescription() after update = %q, want %q", got, "Upcoming tasks")
	}
}

func TestHeaderConfig_ViewInfoEmptyDescription(t *testing.T) {
	hc := NewHeaderConfig()

	hc.SetViewInfo("Task Detail", "")

	if got := hc.GetViewName(); got != "Task Detail" {
		t.Errorf("GetViewName() = %q, want %q", got, "Task Detail")
	}

	if got := hc.GetViewDescription(); got != "" {
		t.Errorf("GetViewDescription() = %q, want empty", got)
	}
}

func TestHeaderConfig_Burndown(t *testing.T) {
	hc := NewHeaderConfig()

	date1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	date3 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	points := []store.BurndownPoint{
		{Date: date1, Remaining: 100},
		{Date: date2, Remaining: 80},
		{Date: date3, Remaining: 60},
	}

	hc.SetBurndown(points)

	got := hc.GetBurndown()
	if len(got) != 3 {
		t.Errorf("len(GetBurndown()) = %d, want 3", len(got))
	}

	if !got[0].Date.Equal(date1) {
		t.Errorf("Burndown[0].Date = %v, want %v", got[0].Date, date1)
	}

	if got[0].Remaining != 100 {
		t.Errorf("Burndown[0].Remaining = %d, want 100", got[0].Remaining)
	}
}

func TestHeaderConfig_Visibility(t *testing.T) {
	hc := NewHeaderConfig()

	// Default should be visible
	if !hc.IsVisible() {
		t.Error("default IsVisible() = false, want true")
	}

	// Set invisible
	hc.SetVisible(false)
	if hc.IsVisible() {
		t.Error("IsVisible() after SetVisible(false) = true, want false")
	}

	// Set visible again
	hc.SetVisible(true)
	if !hc.IsVisible() {
		t.Error("IsVisible() after SetVisible(true) = false, want true")
	}
}

func TestHeaderConfig_UserPreference(t *testing.T) {
	hc := NewHeaderConfig()

	// Default preference should be true
	if !hc.GetUserPreference() {
		t.Error("default GetUserPreference() = false, want true")
	}

	// Set preference
	hc.SetUserPreference(false)
	if hc.GetUserPreference() {
		t.Error("GetUserPreference() after SetUserPreference(false) = true, want false")
	}

	hc.SetUserPreference(true)
	if !hc.GetUserPreference() {
		t.Error("GetUserPreference() after SetUserPreference(true) = false, want true")
	}
}

func TestHeaderConfig_ToggleUserPreference(t *testing.T) {
	hc := NewHeaderConfig()

	// Initial state
	initialPref := hc.GetUserPreference()
	initialVisible := hc.IsVisible()

	// Toggle
	hc.ToggleUserPreference()

	// Preference should be toggled
	if hc.GetUserPreference() == initialPref {
		t.Error("ToggleUserPreference() did not toggle preference")
	}

	// Visible should match new preference
	if hc.IsVisible() != hc.GetUserPreference() {
		t.Error("visible state should match preference after toggle")
	}

	// Toggle back
	hc.ToggleUserPreference()

	// Should return to initial state
	if hc.GetUserPreference() != initialPref {
		t.Error("ToggleUserPreference() twice did not return to initial state")
	}

	if hc.IsVisible() != initialVisible {
		t.Error("visible state should return to initial after double toggle")
	}
}

func TestHeaderConfig_ListenerNotification(t *testing.T) {
	hc := NewHeaderConfig()

	called := false
	listener := func() {
		called = true
	}

	listenerID := hc.AddListener(listener)

	// Test various operations trigger notification
	tests := []struct {
		name   string
		action func()
	}{
		{"SetViewActions", func() { hc.SetViewActions([]HeaderAction{{ID: "test"}}) }},
		{"SetPluginActions", func() { hc.SetPluginActions([]HeaderAction{{ID: "test"}}) }},
		{"SetViewInfo", func() { hc.SetViewInfo("Test", "desc") }},
		{"SetBurndown", func() { hc.SetBurndown([]store.BurndownPoint{{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}}) }},
		{"SetVisible", func() { hc.SetVisible(false); hc.SetVisible(true) }},
		{"ToggleUserPreference", func() { hc.ToggleUserPreference() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			tt.action()

			time.Sleep(10 * time.Millisecond)

			if !called {
				t.Errorf("listener not called after %s", tt.name)
			}
		})
	}

	// Remove listener
	hc.RemoveListener(listenerID)

	called = false
	hc.SetViewActions([]HeaderAction{{ID: "test2"}})

	time.Sleep(10 * time.Millisecond)

	if called {
		t.Error("listener called after RemoveListener")
	}
}

func TestHeaderConfig_SetVisibleNoChangeNoNotify(t *testing.T) {
	hc := NewHeaderConfig()

	callCount := 0
	hc.AddListener(func() {
		callCount++
	})

	// Set to current value (no change)
	hc.SetVisible(true) // Already true by default

	time.Sleep(10 * time.Millisecond)

	if callCount > 0 {
		t.Error("listener called when visibility didn't change")
	}

	// Now change it
	hc.SetVisible(false)

	time.Sleep(10 * time.Millisecond)

	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 after actual change", callCount)
	}
}

func TestHeaderConfig_MultipleListeners(t *testing.T) {
	hc := NewHeaderConfig()

	var mu sync.Mutex
	callCounts := make(map[int]int)

	listener1 := func() {
		mu.Lock()
		callCounts[1]++
		mu.Unlock()
	}
	listener2 := func() {
		mu.Lock()
		callCounts[2]++
		mu.Unlock()
	}

	id1 := hc.AddListener(listener1)
	id2 := hc.AddListener(listener2)

	// Both should be notified
	hc.SetViewInfo("Test", "desc")

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 1 {
		t.Errorf("callCounts = %v, want both 1", callCounts)
	}
	mu.Unlock()

	// Remove one
	hc.RemoveListener(id1)

	hc.SetViewInfo("Test2", "desc2")

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 2 {
		t.Errorf("callCounts after remove = %v, want {1:1, 2:2}", callCounts)
	}
	mu.Unlock()

	// Remove second
	hc.RemoveListener(id2)

	hc.SetViewInfo("Test3", "desc3")

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 2 {
		t.Error("callCounts changed after both listeners removed")
	}
	mu.Unlock()
}

func TestHeaderConfig_ConcurrentAccess(t *testing.T) {
	hc := NewHeaderConfig()

	done := make(chan bool)

	// Writer goroutine - actions
	go func() {
		for i := range 50 {
			hc.SetViewActions([]HeaderAction{{ID: string(rune('a' + i%26))}})
			hc.SetPluginActions([]HeaderAction{{ID: string(rune('A' + i%26))}})
		}
		done <- true
	}()

	// Writer goroutine - view info
	go func() {
		for i := range 50 {
			hc.SetViewInfo("View"+string(rune('a'+i%26)), "desc")
		}
		done <- true
	}()

	// Writer goroutine - visibility
	go func() {
		for i := range 50 {
			hc.SetVisible(i%2 == 0)
			if i%5 == 0 {
				hc.ToggleUserPreference()
			}
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for range 100 {
			_ = hc.GetViewActions()
			_ = hc.GetPluginActions()
			_ = hc.GetViewName()
			_ = hc.GetViewDescription()
			_ = hc.GetBurndown()
			_ = hc.IsVisible()
			_ = hc.GetUserPreference()
		}
		done <- true
	}()

	// Wait for all
	for range 4 {
		<-done
	}

	// If we get here without panic, test passes
}

func TestHeaderConfig_EmptyCollections(t *testing.T) {
	hc := NewHeaderConfig()

	// Set empty actions
	hc.SetViewActions([]HeaderAction{})
	if len(hc.GetViewActions()) != 0 {
		t.Error("GetViewActions() should return empty slice")
	}

	// Set nil actions
	hc.SetPluginActions(nil)
	if len(hc.GetPluginActions()) != 0 {
		t.Error("GetPluginActions() with nil input should return empty slice")
	}

	// Set empty burndown
	hc.SetBurndown([]store.BurndownPoint{})
	if len(hc.GetBurndown()) != 0 {
		t.Error("GetBurndown() should return empty slice")
	}
}

func TestHeaderConfig_ListenerIDUniqueness(t *testing.T) {
	hc := NewHeaderConfig()

	ids := make(map[int]bool)
	for range 100 {
		id := hc.AddListener(func() {})
		if ids[id] {
			t.Errorf("duplicate listener ID: %d", id)
		}
		ids[id] = true
	}

	if len(ids) != 100 {
		t.Errorf("expected 100 unique IDs, got %d", len(ids))
	}
}
