package model

import (
	"sync"
	"testing"
	"time"
)

func TestNewHeaderConfig(t *testing.T) {
	hc := NewHeaderConfig()

	if hc == nil {
		t.Fatal("NewHeaderConfig() returned nil")
	}

	if !hc.IsVisible() {
		t.Error("initial IsVisible() = false, want true")
	}

	if !hc.GetUserPreference() {
		t.Error("initial GetUserPreference() = false, want true")
	}
}

func TestHeaderConfig_Visibility(t *testing.T) {
	hc := NewHeaderConfig()

	if !hc.IsVisible() {
		t.Error("default IsVisible() = false, want true")
	}

	hc.SetVisible(false)
	if hc.IsVisible() {
		t.Error("IsVisible() after SetVisible(false) = true, want false")
	}

	hc.SetVisible(true)
	if !hc.IsVisible() {
		t.Error("IsVisible() after SetVisible(true) = false, want true")
	}
}

func TestHeaderConfig_UserPreference(t *testing.T) {
	hc := NewHeaderConfig()

	if !hc.GetUserPreference() {
		t.Error("default GetUserPreference() = false, want true")
	}

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

	initialPref := hc.GetUserPreference()
	initialVisible := hc.IsVisible()

	hc.ToggleUserPreference()

	if hc.GetUserPreference() == initialPref {
		t.Error("ToggleUserPreference() did not toggle preference")
	}

	if hc.IsVisible() != hc.GetUserPreference() {
		t.Error("visible state should match preference after toggle")
	}

	hc.ToggleUserPreference()

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

	tests := []struct {
		name   string
		action func()
	}{
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

	hc.RemoveListener(listenerID)

	called = false
	hc.SetVisible(false)

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

	hc.SetVisible(true) // already true by default

	time.Sleep(10 * time.Millisecond)

	if callCount > 0 {
		t.Error("listener called when visibility didn't change")
	}

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

	hc.SetVisible(false)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 1 {
		t.Errorf("callCounts = %v, want both 1", callCounts)
	}
	mu.Unlock()

	hc.RemoveListener(id1)

	hc.SetVisible(true)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 2 {
		t.Errorf("callCounts after remove = %v, want {1:1, 2:2}", callCounts)
	}
	mu.Unlock()

	hc.RemoveListener(id2)

	hc.SetVisible(false)

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

	go func() {
		for i := range 50 {
			hc.SetVisible(i%2 == 0)
			if i%5 == 0 {
				hc.ToggleUserPreference()
			}
		}
		done <- true
	}()

	go func() {
		for range 100 {
			_ = hc.IsVisible()
			_ = hc.GetUserPreference()
		}
		done <- true
	}()

	for range 2 {
		<-done
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
