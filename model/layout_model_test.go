package model

import (
	"sync"
	"testing"
	"time"
)

func TestNewLayoutModel(t *testing.T) {
	lm := NewLayoutModel()

	if lm == nil {
		t.Fatal("NewLayoutModel() returned nil")
	}

	// Initial state should be zero values
	if lm.GetContentViewID() != "" {
		t.Errorf("initial GetContentViewID() = %q, want empty", lm.GetContentViewID())
	}

	if lm.GetContentParams() != nil {
		t.Error("initial GetContentParams() should be nil")
	}

	if lm.GetRevision() != 0 {
		t.Errorf("initial GetRevision() = %d, want 0", lm.GetRevision())
	}
}

func TestLayoutModel_SetContent(t *testing.T) {
	lm := NewLayoutModel()

	// Set content with nil params
	lm.SetContent(TaskDetailViewID, nil)

	if lm.GetContentViewID() != TaskDetailViewID {
		t.Errorf("GetContentViewID() = %q, want %q", lm.GetContentViewID(), TaskDetailViewID)
	}

	if lm.GetContentParams() != nil {
		t.Error("GetContentParams() should be nil")
	}

	if lm.GetRevision() != 1 {
		t.Errorf("GetRevision() = %d, want 1", lm.GetRevision())
	}

	// Set content with params
	params := map[string]any{"taskID": "TIKI-1", "index": 42}
	lm.SetContent(TaskDetailViewID, params)

	if lm.GetContentViewID() != TaskDetailViewID {
		t.Errorf("GetContentViewID() = %q, want %q", lm.GetContentViewID(), TaskDetailViewID)
	}

	gotParams := lm.GetContentParams()
	if gotParams == nil {
		t.Fatal("GetContentParams() returned nil")
	}

	if gotParams["taskID"] != "TIKI-1" {
		t.Errorf("params[taskID] = %v, want TIKI-1", gotParams["taskID"])
	}

	if gotParams["index"] != 42 {
		t.Errorf("params[index] = %v, want 42", gotParams["index"])
	}

	if lm.GetRevision() != 2 {
		t.Errorf("GetRevision() = %d, want 2 after second SetContent", lm.GetRevision())
	}
}

func TestLayoutModel_Touch(t *testing.T) {
	lm := NewLayoutModel()

	// Set initial content
	lm.SetContent(TaskDetailViewID, map[string]any{"foo": "bar"})
	initialRevision := lm.GetRevision()

	// Touch should increment revision without changing content
	lm.Touch()

	if lm.GetRevision() != initialRevision+1 {
		t.Errorf("GetRevision() after Touch = %d, want %d", lm.GetRevision(), initialRevision+1)
	}

	// ViewID and params should be unchanged
	if lm.GetContentViewID() != TaskDetailViewID {
		t.Errorf("GetContentViewID() changed after Touch = %q, want %q",
			lm.GetContentViewID(), TaskDetailViewID)
	}

	gotParams := lm.GetContentParams()
	if gotParams == nil || gotParams["foo"] != "bar" {
		t.Error("GetContentParams() changed after Touch")
	}

	// Multiple touches should keep incrementing
	lm.Touch()
	lm.Touch()

	if lm.GetRevision() != initialRevision+3 {
		t.Errorf("GetRevision() after 3 touches = %d, want %d", lm.GetRevision(), initialRevision+3)
	}
}

func TestLayoutModel_ListenerNotification(t *testing.T) {
	lm := NewLayoutModel()

	// Track listener calls
	called := false
	listener := func() {
		called = true
	}

	// Add listener
	listenerID := lm.AddListener(listener)

	// SetContent should notify
	lm.SetContent(TaskDetailViewID, nil)

	// Give listener time to execute
	time.Sleep(10 * time.Millisecond)

	if !called {
		t.Error("listener not called after SetContent")
	}

	// Reset and test Touch
	called = false
	lm.Touch()

	time.Sleep(10 * time.Millisecond)

	if !called {
		t.Error("listener not called after Touch")
	}

	// Remove listener
	lm.RemoveListener(listenerID)

	// Should not be called anymore
	called = false
	lm.SetContent(TaskDetailViewID, nil)

	time.Sleep(10 * time.Millisecond)

	if called {
		t.Error("listener called after RemoveListener")
	}
}

func TestLayoutModel_MultipleListeners(t *testing.T) {
	lm := NewLayoutModel()

	// Track calls
	var mu sync.Mutex
	callCounts := make(map[int]int)

	// Add multiple listeners
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
	listener3 := func() {
		mu.Lock()
		callCounts[3]++
		mu.Unlock()
	}

	id1 := lm.AddListener(listener1)
	id2 := lm.AddListener(listener2)
	id3 := lm.AddListener(listener3)

	// All should be notified
	lm.SetContent(TaskDetailViewID, nil)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 1 || callCounts[2] != 1 || callCounts[3] != 1 {
		t.Errorf("call counts = %v, want all 1", callCounts)
	}
	mu.Unlock()

	// Remove middle listener
	lm.RemoveListener(id2)

	// Only 1 and 3 should be notified
	lm.Touch()

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 2 || callCounts[2] != 1 || callCounts[3] != 2 {
		t.Errorf("call counts after remove = %v, want {1:2, 2:1, 3:2}", callCounts)
	}
	mu.Unlock()

	// Remove all
	lm.RemoveListener(id1)
	lm.RemoveListener(id3)

	// None should be notified
	lm.SetContent(TaskEditViewID, nil)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCounts[1] != 2 || callCounts[2] != 1 || callCounts[3] != 2 {
		t.Errorf("call counts after remove all = %v, want unchanged", callCounts)
	}
	mu.Unlock()
}

func TestLayoutModel_RevisionMonotonicity(t *testing.T) {
	lm := NewLayoutModel()

	// Revision should always increase
	revisions := make([]uint64, 0, 100)

	for range 100 {
		lm.SetContent(TaskDetailViewID, nil)
		revisions = append(revisions, lm.GetRevision())
	}

	// Verify monotonic increase
	for i := 1; i < len(revisions); i++ {
		if revisions[i] <= revisions[i-1] {
			t.Errorf("revision[%d] = %d not greater than revision[%d] = %d",
				i, revisions[i], i-1, revisions[i-1])
		}
	}
}

func TestLayoutModel_ParamsIsolation(t *testing.T) {
	lm := NewLayoutModel()

	// Set content with params
	originalParams := map[string]any{
		"key1": "value1",
		"key2": 42,
	}
	lm.SetContent(TaskDetailViewID, originalParams)

	// Get params
	gotParams := lm.GetContentParams()

	// Modify the returned params
	gotParams["key1"] = "modified"
	gotParams["key3"] = "new"

	// Original params should be unchanged (if implementation copies)
	// Note: Current implementation doesn't copy, so this tests the current behavior
	// If implementation changes to copy, this test should be updated
	secondGet := lm.GetContentParams()
	if secondGet["key1"] != "modified" {
		// If this fails, implementation is copying params (which is fine)
		t.Logf("Implementation copies params - this is OK")
	}
}

func TestLayoutModel_ConcurrentAccess(t *testing.T) {
	lm := NewLayoutModel()

	// This test verifies no panics under concurrent access
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := range 100 {
			params := map[string]any{"index": i}
			lm.SetContent(TaskDetailViewID, params)
			lm.Touch()
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for range 100 {
			_ = lm.GetContentViewID()
			_ = lm.GetContentParams()
			_ = lm.GetRevision()
		}
		done <- true
	}()

	// Listener management goroutine
	go func() {
		ids := make([]int, 0, 10)
		for i := range 10 {
			id := lm.AddListener(func() {})
			ids = append(ids, id)
			if i%3 == 0 && len(ids) > 0 {
				lm.RemoveListener(ids[0])
				ids = ids[1:]
			}
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// If we get here without panic, test passes
}

func TestLayoutModel_EmptyViewID(t *testing.T) {
	lm := NewLayoutModel()

	// Should be able to set empty ViewID
	lm.SetContent("", nil)

	if lm.GetContentViewID() != "" {
		t.Errorf("GetContentViewID() = %q, want empty", lm.GetContentViewID())
	}

	if lm.GetRevision() != 1 {
		t.Error("SetContent with empty ViewID should still increment revision")
	}
}

func TestLayoutModel_ListenerIDUniqueness(t *testing.T) {
	lm := NewLayoutModel()

	// Add many listeners and verify IDs are unique
	ids := make(map[int]bool)
	for range 100 {
		id := lm.AddListener(func() {})
		if ids[id] {
			t.Errorf("duplicate listener ID: %d", id)
		}
		ids[id] = true
	}

	if len(ids) != 100 {
		t.Errorf("expected 100 unique IDs, got %d", len(ids))
	}
}

func TestLayoutModel_RemoveNonexistentListener(t *testing.T) {
	lm := NewLayoutModel()

	// Removing non-existent listener should not panic
	lm.RemoveListener(999)
	lm.RemoveListener(-1)
	lm.RemoveListener(0)

	// Should still work normally after
	lm.SetContent(TaskDetailViewID, nil)
	if lm.GetRevision() != 1 {
		t.Error("model not working after removing non-existent listeners")
	}
}
