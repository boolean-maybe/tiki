package model

import (
	"sync"
	"testing"
)

func TestNewStatuslineConfig(t *testing.T) {
	sc := NewStatuslineConfig()

	if sc == nil {
		t.Fatal("NewStatuslineConfig() returned nil")
	}

	if !sc.IsVisible() {
		t.Error("initial IsVisible() = false, want true")
	}

	if len(sc.GetLeftStats()) != 0 {
		t.Error("initial GetLeftStats() should be empty")
	}

	msg, level, autoHide := sc.GetMessage()
	if msg != "" {
		t.Errorf("initial message = %q, want empty", msg)
	}
	if level != MessageLevelInfo {
		t.Errorf("initial level = %q, want %q", level, MessageLevelInfo)
	}
	if autoHide {
		t.Error("initial autoHide = true, want false")
	}
}

func TestStatuslineConfig_LeftStats(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetLeftStat("Branch", "main", 3)
	sc.SetLeftStat("User", "vbobrov", 4)

	stats := sc.GetLeftStats()
	if len(stats) != 2 {
		t.Errorf("len(GetLeftStats()) = %d, want 2", len(stats))
	}

	if stats["Branch"].Value != "main" {
		t.Errorf("stats[Branch].Value = %q, want %q", stats["Branch"].Value, "main")
	}
	if stats["Branch"].Priority != 3 {
		t.Errorf("stats[Branch].Priority = %d, want 3", stats["Branch"].Priority)
	}
	if stats["User"].Value != "vbobrov" {
		t.Errorf("stats[User].Value = %q, want %q", stats["User"].Value, "vbobrov")
	}
}

func TestStatuslineConfig_ViewStats(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetLeftStat("Branch", "main", 3)
	sc.SetViewStat("Total", "42", 5)

	stats := sc.GetLeftStats()
	if len(stats) != 2 {
		t.Errorf("len(GetLeftStats()) = %d, want 2", len(stats))
	}
	if stats["Total"].Value != "42" {
		t.Errorf("stats[Total].Value = %q, want %q", stats["Total"].Value, "42")
	}
}

func TestStatuslineConfig_ViewStatsOverrideBase(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetLeftStat("Branch", "main", 3)
	sc.SetViewStat("Branch", "feature", 3)

	stats := sc.GetLeftStats()
	if stats["Branch"].Value != "feature" {
		t.Errorf("view stat should override base: got %q, want %q", stats["Branch"].Value, "feature")
	}
}

func TestStatuslineConfig_ClearViewStats(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetLeftStat("Branch", "main", 3)
	sc.SetViewStat("Total", "42", 5)

	sc.ClearViewStats()

	stats := sc.GetLeftStats()
	if len(stats) != 1 {
		t.Errorf("len(GetLeftStats()) after clear = %d, want 1", len(stats))
	}
	if _, ok := stats["Total"]; ok {
		t.Error("view stat should be cleared")
	}
	if stats["Branch"].Value != "main" {
		t.Error("base stat should remain after ClearViewStats")
	}
}

func TestStatuslineConfig_Message(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetMessage("error occurred", MessageLevelError, false)

	msg, level, autoHide := sc.GetMessage()
	if msg != "error occurred" {
		t.Errorf("message = %q, want %q", msg, "error occurred")
	}
	if level != MessageLevelError {
		t.Errorf("level = %q, want %q", level, MessageLevelError)
	}
	if autoHide {
		t.Error("autoHide = true, want false")
	}
}

func TestStatuslineConfig_MessageAutoHide(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetMessage("transient error", MessageLevelInfo, true)

	msg, _, autoHide := sc.GetMessage()
	if msg != "transient error" {
		t.Errorf("message = %q, want %q", msg, "transient error")
	}
	if !autoHide {
		t.Error("autoHide = false, want true")
	}
}

func TestStatuslineConfig_MessageMakesVisible(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetVisible(false)
	if sc.IsVisible() {
		t.Fatal("precondition: should be invisible")
	}

	sc.SetMessage("hello", MessageLevelInfo, false)
	if !sc.IsVisible() {
		t.Error("SetMessage should make statusline visible")
	}
}

func TestStatuslineConfig_ClearMessage(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetMessage("error", MessageLevelError, true)
	sc.ClearMessage()

	msg, level, autoHide := sc.GetMessage()
	if msg != "" {
		t.Errorf("message after clear = %q, want empty", msg)
	}
	if level != MessageLevelInfo {
		t.Errorf("level after clear = %q, want %q", level, MessageLevelInfo)
	}
	if autoHide {
		t.Error("autoHide should be false after ClearMessage")
	}
}

func TestStatuslineConfig_ClearMessageNoChangeNoNotify(t *testing.T) {
	sc := NewStatuslineConfig()

	callCount := 0
	sc.AddListener(func() { callCount++ })

	// clearing already-empty message should not notify
	sc.ClearMessage()

	if callCount != 0 {
		t.Errorf("callCount = %d, want 0 (no-op clear)", callCount)
	}
}

func TestStatuslineConfig_DismissAutoHide(t *testing.T) {
	sc := NewStatuslineConfig()

	// no auto-hide active → should return false
	if sc.DismissAutoHide() {
		t.Error("DismissAutoHide() = true when no auto-hide active")
	}

	// set auto-hide message
	sc.SetMessage("will hide", MessageLevelInfo, true)
	if !sc.IsVisible() {
		t.Fatal("precondition: should be visible after SetMessage")
	}

	// dismiss
	if !sc.DismissAutoHide() {
		t.Error("DismissAutoHide() = false, want true")
	}

	// statusline should remain visible (only the message is dismissed)
	if !sc.IsVisible() {
		t.Error("statusline should remain visible after DismissAutoHide")
	}

	// message should be cleared
	msg, level, autoHide := sc.GetMessage()
	if msg != "" {
		t.Errorf("message after dismiss = %q, want empty", msg)
	}
	if level != MessageLevelInfo {
		t.Errorf("level after dismiss = %q, want %q", level, MessageLevelInfo)
	}
	if autoHide {
		t.Error("autoHide should be false after dismiss")
	}

	// second dismiss should be no-op
	if sc.DismissAutoHide() {
		t.Error("second DismissAutoHide() = true, want false")
	}
}

func TestStatuslineConfig_DismissAutoHideNotifiesListeners(t *testing.T) {
	sc := NewStatuslineConfig()

	callCount := 0
	sc.AddListener(func() { callCount++ })

	sc.SetMessage("hide me", MessageLevelInfo, true)
	callCount = 0 // reset after SetMessage notification

	sc.DismissAutoHide()

	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 after DismissAutoHide", callCount)
	}
}

func TestStatuslineConfig_Visibility(t *testing.T) {
	sc := NewStatuslineConfig()

	if !sc.IsVisible() {
		t.Error("default IsVisible() = false, want true")
	}

	sc.SetVisible(false)
	if sc.IsVisible() {
		t.Error("IsVisible() after SetVisible(false) = true, want false")
	}

	sc.SetVisible(true)
	if !sc.IsVisible() {
		t.Error("IsVisible() after SetVisible(true) = false, want true")
	}
}

func TestStatuslineConfig_SetVisibleNoChangeNoNotify(t *testing.T) {
	sc := NewStatuslineConfig()

	callCount := 0
	sc.AddListener(func() { callCount++ })

	sc.SetVisible(true) // already true

	if callCount != 0 {
		t.Error("listener called when visibility didn't change")
	}

	sc.SetVisible(false)

	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 after actual change", callCount)
	}
}

func TestStatuslineConfig_ListenerNotification(t *testing.T) {
	sc := NewStatuslineConfig()

	called := false
	listenerID := sc.AddListener(func() { called = true })

	tests := []struct {
		name   string
		action func()
	}{
		{"SetLeftStat", func() { sc.SetLeftStat("k", "v", 1) }},
		{"SetViewStat", func() { sc.SetViewStat("k", "v", 1) }},
		{"ClearViewStats", func() { sc.ClearViewStats() }},
		{"SetMessage", func() { sc.SetMessage("msg", MessageLevelInfo, false) }},
		{"SetVisible", func() { sc.SetVisible(false); sc.SetVisible(true) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			tt.action()

			if !called {
				t.Errorf("listener not called after %s", tt.name)
			}
		})
	}

	sc.RemoveListener(listenerID)

	called = false
	sc.SetLeftStat("x", "y", 1)

	if called {
		t.Error("listener called after RemoveListener")
	}
}

func TestStatuslineConfig_RightViewStats(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetRightViewStat("Total", "42", 5)

	stats := sc.GetRightViewStats()
	if len(stats) != 1 {
		t.Errorf("len(GetRightViewStats()) = %d, want 1", len(stats))
	}
	if stats["Total"].Value != "42" {
		t.Errorf("stats[Total].Value = %q, want %q", stats["Total"].Value, "42")
	}
	if stats["Total"].Priority != 5 {
		t.Errorf("stats[Total].Priority = %d, want 5", stats["Total"].Priority)
	}
}

func TestStatuslineConfig_ClearRightViewStats(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetRightViewStat("Total", "42", 5)
	sc.SetRightViewStat("Done", "10", 6)

	sc.ClearRightViewStats()

	stats := sc.GetRightViewStats()
	if len(stats) != 0 {
		t.Errorf("len(GetRightViewStats()) after clear = %d, want 0", len(stats))
	}
}

func TestStatuslineConfig_RightViewStatsNotifyListeners(t *testing.T) {
	sc := NewStatuslineConfig()

	called := false
	sc.AddListener(func() { called = true })

	sc.SetRightViewStat("Total", "42", 5)

	if !called {
		t.Error("listener not called after SetRightViewStat")
	}

	called = false
	sc.ClearRightViewStats()

	if !called {
		t.Error("listener not called after ClearRightViewStats")
	}
}

func TestStatuslineConfig_ConcurrentAccess(t *testing.T) {
	sc := NewStatuslineConfig()

	done := make(chan bool)

	go func() {
		for i := range 50 {
			sc.SetLeftStat("key", "value", i)
			sc.SetViewStat("vkey", "vvalue", i)
			if i%10 == 0 {
				sc.ClearViewStats()
			}
		}
		done <- true
	}()

	go func() {
		for i := range 50 {
			sc.SetMessage("msg", MessageLevelInfo, i%2 == 0)
			if i%5 == 0 {
				sc.ClearMessage()
			}
			sc.DismissAutoHide()
		}
		done <- true
	}()

	go func() {
		for i := range 50 {
			sc.SetVisible(i%2 == 0)
		}
		done <- true
	}()

	go func() {
		for i := range 50 {
			sc.SetRightViewStat("Total", "42", i)
			if i%10 == 0 {
				sc.ClearRightViewStats()
			}
		}
		done <- true
	}()

	go func() {
		for range 100 {
			_ = sc.GetLeftStats()
			_ = sc.GetRightViewStats()
			_, _, _ = sc.GetMessage()
			_ = sc.IsVisible()
		}
		done <- true
	}()

	for range 5 {
		<-done
	}
}

func TestStatuslineConfig_MultipleListeners(t *testing.T) {
	sc := NewStatuslineConfig()

	var mu sync.Mutex
	callCounts := make(map[int]int)

	id1 := sc.AddListener(func() {
		mu.Lock()
		callCounts[1]++
		mu.Unlock()
	})
	id2 := sc.AddListener(func() {
		mu.Lock()
		callCounts[2]++
		mu.Unlock()
	})

	sc.SetLeftStat("test", "value", 1)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 1 {
		t.Errorf("callCounts = %v, want both 1", callCounts)
	}
	mu.Unlock()

	sc.RemoveListener(id1)
	sc.SetLeftStat("test2", "value2", 2)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 2 {
		t.Errorf("callCounts after remove = %v, want {1:1, 2:2}", callCounts)
	}
	mu.Unlock()

	sc.RemoveListener(id2)
}

func TestStatuslineConfig_SetRightViewStats(t *testing.T) {
	sc := NewStatuslineConfig()

	stats := map[string]StatValue{
		"Total": {Value: "42", Priority: 5},
		"Done":  {Value: "10", Priority: 6},
	}

	sc.SetRightViewStats(stats)

	got := sc.GetRightViewStats()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got["Total"].Value != "42" {
		t.Errorf("Total = %q, want %q", got["Total"].Value, "42")
	}
	if got["Done"].Value != "10" {
		t.Errorf("Done = %q, want %q", got["Done"].Value, "10")
	}
}

func TestStatuslineConfig_SetRightViewStats_replacesExisting(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetRightViewStats(map[string]StatValue{
		"Old": {Value: "1", Priority: 1},
	})

	sc.SetRightViewStats(map[string]StatValue{
		"New": {Value: "2", Priority: 2},
	})

	got := sc.GetRightViewStats()
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if _, ok := got["Old"]; ok {
		t.Error("old stat should be replaced")
	}
	if got["New"].Value != "2" {
		t.Errorf("New = %q, want %q", got["New"].Value, "2")
	}
}

func TestStatuslineConfig_SetRightViewStats_singleNotification(t *testing.T) {
	sc := NewStatuslineConfig()

	callCount := 0
	sc.AddListener(func() { callCount++ })

	sc.SetRightViewStats(map[string]StatValue{
		"A": {Value: "1", Priority: 1},
		"B": {Value: "2", Priority: 2},
		"C": {Value: "3", Priority: 3},
	})

	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (single notification for bulk set)", callCount)
	}
}

func TestStatuslineConfig_SetViewStats(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetLeftStat("Branch", "main", 1)

	sc.SetViewStats(map[string]StatValue{
		"Filter": {Value: "active", Priority: 10},
	})

	got := sc.GetLeftStats()
	if got["Branch"].Value != "main" {
		t.Error("base stat should be preserved")
	}
	if got["Filter"].Value != "active" {
		t.Errorf("Filter = %q, want %q", got["Filter"].Value, "active")
	}
}

func TestStatuslineConfig_SetViewStats_replacesExisting(t *testing.T) {
	sc := NewStatuslineConfig()

	sc.SetViewStats(map[string]StatValue{
		"Old": {Value: "1", Priority: 1},
	})

	sc.SetViewStats(map[string]StatValue{
		"New": {Value: "2", Priority: 2},
	})

	got := sc.GetLeftStats()
	if _, ok := got["Old"]; ok {
		t.Error("old view stat should be replaced")
	}
	if got["New"].Value != "2" {
		t.Errorf("New = %q, want %q", got["New"].Value, "2")
	}
}

func TestStatuslineConfig_SetViewStats_singleNotification(t *testing.T) {
	sc := NewStatuslineConfig()

	callCount := 0
	sc.AddListener(func() { callCount++ })

	sc.SetViewStats(map[string]StatValue{
		"A": {Value: "1", Priority: 1},
		"B": {Value: "2", Priority: 2},
	})

	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
}
