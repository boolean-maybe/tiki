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

	if len(hc.GetStats()) != 0 {
		t.Error("initial GetStats() should be empty")
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

func TestHeaderConfig_BaseStats(t *testing.T) {
	hc := NewHeaderConfig()

	// Set base stats
	hc.SetBaseStat("version", "v1.0.0", 100)
	hc.SetBaseStat("user", "testuser", 90)

	stats := hc.GetStats()

	if len(stats) != 2 {
		t.Errorf("len(GetStats()) = %d, want 2", len(stats))
	}

	if stats["version"].Value != "v1.0.0" {
		t.Errorf("stats[version].Value = %q, want %q", stats["version"].Value, "v1.0.0")
	}

	if stats["version"].Priority != 100 {
		t.Errorf("stats[version].Priority = %d, want 100", stats["version"].Priority)
	}

	if stats["user"].Value != "testuser" {
		t.Errorf("stats[user].Value = %q, want %q", stats["user"].Value, "testuser")
	}
}

func TestHeaderConfig_ViewStats(t *testing.T) {
	hc := NewHeaderConfig()

	// Set view stats
	hc.SetViewStat("total", "42", 50)
	hc.SetViewStat("selected", "5", 60)

	stats := hc.GetStats()

	if len(stats) != 2 {
		t.Errorf("len(GetStats()) = %d, want 2", len(stats))
	}

	if stats["total"].Value != "42" {
		t.Errorf("stats[total].Value = %q, want %q", stats["total"].Value, "42")
	}

	if stats["selected"].Value != "5" {
		t.Errorf("stats[selected].Value = %q, want %q", stats["selected"].Value, "5")
	}
}

func TestHeaderConfig_StatsMerging(t *testing.T) {
	hc := NewHeaderConfig()

	// Set base stats
	hc.SetBaseStat("version", "v1.0.0", 100)
	hc.SetBaseStat("user", "testuser", 90)

	// Set view stats (including one that overrides base)
	hc.SetViewStat("total", "42", 50)
	hc.SetViewStat("user", "viewuser", 95) // Override base stat

	stats := hc.GetStats()

	// Should have 3 unique keys (version, user, total)
	if len(stats) != 3 {
		t.Errorf("len(GetStats()) = %d, want 3", len(stats))
	}

	// View stat should override base stat
	if stats["user"].Value != "viewuser" {
		t.Errorf("stats[user].Value = %q, want %q (view should override base)",
			stats["user"].Value, "viewuser")
	}

	if stats["user"].Priority != 95 {
		t.Errorf("stats[user].Priority = %d, want 95", stats["user"].Priority)
	}

	// Base stats should still be present
	if stats["version"].Value != "v1.0.0" {
		t.Error("base stat 'version' missing after merge")
	}

	// View stat should be present
	if stats["total"].Value != "42" {
		t.Error("view stat 'total' missing after merge")
	}
}

func TestHeaderConfig_ClearViewStats(t *testing.T) {
	hc := NewHeaderConfig()

	// Set both base and view stats
	hc.SetBaseStat("version", "v1.0.0", 100)
	hc.SetViewStat("total", "42", 50)
	hc.SetViewStat("selected", "5", 60)

	stats := hc.GetStats()
	if len(stats) != 3 {
		t.Errorf("len(GetStats()) before clear = %d, want 3", len(stats))
	}

	// Clear view stats
	hc.ClearViewStats()

	stats = hc.GetStats()

	// Should only have base stats now
	if len(stats) != 1 {
		t.Errorf("len(GetStats()) after clear = %d, want 1", len(stats))
	}

	if stats["version"].Value != "v1.0.0" {
		t.Error("base stats should remain after ClearViewStats")
	}

	if _, ok := stats["total"]; ok {
		t.Error("view stats should be cleared")
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
		{"SetBaseStat", func() { hc.SetBaseStat("key", "value", 1) }},
		{"SetViewStat", func() { hc.SetViewStat("key", "value", 1) }},
		{"ClearViewStats", func() { hc.ClearViewStats() }},
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
	hc.SetBaseStat("test", "value", 1)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 1 {
		t.Errorf("callCounts = %v, want both 1", callCounts)
	}
	mu.Unlock()

	// Remove one
	hc.RemoveListener(id1)

	hc.SetViewStat("another", "test", 1)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 2 {
		t.Errorf("callCounts after remove = %v, want {1:1, 2:2}", callCounts)
	}
	mu.Unlock()

	// Remove second
	hc.RemoveListener(id2)

	hc.ClearViewStats()

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

	// Writer goroutine - stats
	go func() {
		for i := range 50 {
			hc.SetBaseStat("key", "value", i)
			hc.SetViewStat("viewkey", "viewvalue", i)
			if i%10 == 0 {
				hc.ClearViewStats()
			}
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
			_ = hc.GetStats()
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

func TestHeaderConfig_StatPriorityOrdering(t *testing.T) {
	hc := NewHeaderConfig()

	// Set stats with different priorities
	hc.SetBaseStat("low", "value", 10)
	hc.SetBaseStat("high", "value", 100)
	hc.SetBaseStat("medium", "value", 50)

	stats := hc.GetStats()

	// Verify all stats are present (priority doesn't filter, just orders)
	if len(stats) != 3 {
		t.Errorf("len(stats) = %d, want 3", len(stats))
	}

	// Verify priorities are preserved
	if stats["low"].Priority != 10 {
		t.Errorf("stats[low].Priority = %d, want 10", stats["low"].Priority)
	}

	if stats["high"].Priority != 100 {
		t.Errorf("stats[high].Priority = %d, want 100", stats["high"].Priority)
	}

	if stats["medium"].Priority != 50 {
		t.Errorf("stats[medium].Priority = %d, want 50", stats["medium"].Priority)
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
