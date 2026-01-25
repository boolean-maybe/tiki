package controller

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"
)

func TestNavigationState_PushPop(t *testing.T) {
	nav := newViewStack()

	// Push first view
	nav.push(model.TaskDetailViewID, nil)

	// Verify depth
	if nav.depth() != 1 {
		t.Errorf("depth = %d, want 1", nav.depth())
	}

	// Push second view with params
	params := model.EncodeTaskDetailParams(model.TaskDetailParams{TaskID: "TIKI-1"})
	nav.push(model.TaskDetailViewID, params)

	// Verify depth
	if nav.depth() != 2 {
		t.Errorf("depth = %d, want 2", nav.depth())
	}

	// Pop should return task detail view
	entry := nav.pop()
	if entry == nil {
		t.Fatal("pop() returned nil, want ViewEntry")
	}
	if entry.ViewID != model.TaskDetailViewID {
		t.Errorf("ViewID = %v, want %v", entry.ViewID, model.TaskDetailViewID)
	}
	if model.DecodeTaskDetailParams(entry.Params).TaskID != "TIKI-1" {
		t.Errorf("taskID param = %v, want TIKI-1", model.DecodeTaskDetailParams(entry.Params).TaskID)
	}

	// Verify depth decreased
	if nav.depth() != 1 {
		t.Errorf("depth after pop = %d, want 1", nav.depth())
	}

	// Pop should return board view
	entry = nav.pop()
	if entry == nil {
		t.Fatal("pop() returned nil, want ViewEntry")
	}
	if entry.ViewID != model.TaskDetailViewID {
		t.Errorf("ViewID = %v, want %v", entry.ViewID, model.TaskDetailViewID)
	}

	// Stack should be empty
	if nav.depth() != 0 {
		t.Errorf("depth after second pop = %d, want 0", nav.depth())
	}

	// Pop on empty stack should return nil
	entry = nav.pop()
	if entry != nil {
		t.Error("pop() on empty stack should return nil")
	}
}

func TestNavigationState_CurrentView(t *testing.T) {
	nav := newViewStack()

	// CurrentView on empty stack should return nil
	entry := nav.currentView()
	if entry != nil {
		t.Error("currentView() on empty stack should return nil")
	}

	// Push views
	nav.push(model.TaskDetailViewID, nil)
	nav.push(model.TaskEditViewID, nil)

	// CurrentView should return task edit (top) without removing it
	entry = nav.currentView()
	if entry == nil {
		t.Fatal("currentView() returned nil")
	}
	if entry.ViewID != model.TaskEditViewID {
		t.Errorf("ViewID = %v, want %v", entry.ViewID, model.TaskEditViewID)
	}

	// Depth should not change
	if nav.depth() != 2 {
		t.Error("currentView() should not modify stack")
	}

	// Calling CurrentView again should return same view
	entry2 := nav.currentView()
	if entry2.ViewID != model.TaskEditViewID {
		t.Error("currentView() should consistently return top view")
	}
}

func TestNavigationState_CurrentViewID(t *testing.T) {
	nav := newViewStack()

	// Empty stack
	if nav.currentViewID() != "" {
		t.Errorf("currentViewID() on empty stack = %v, want empty string", nav.currentViewID())
	}

	// With views
	nav.push(model.TaskDetailViewID, nil)
	if nav.currentViewID() != model.TaskDetailViewID {
		t.Errorf("currentViewID() = %v, want %v", nav.currentViewID(), model.TaskDetailViewID)
	}

	nav.push(model.TaskDetailViewID, nil)
	if nav.currentViewID() != model.TaskDetailViewID {
		t.Errorf("currentViewID() = %v, want %v", nav.currentViewID(), model.TaskDetailViewID)
	}
}

func TestNavigationState_PreviousView(t *testing.T) {
	nav := newViewStack()

	// Empty stack
	entry := nav.previousView()
	if entry != nil {
		t.Error("previousView() on empty stack should return nil")
	}

	// Single view - no previous
	nav.push(model.TaskDetailViewID, nil)
	entry = nav.previousView()
	if entry != nil {
		t.Error("previousView() with depth 1 should return nil")
	}

	// Two views - should return first
	nav.push(model.TaskDetailViewID, nil)
	entry = nav.previousView()
	if entry == nil {
		t.Fatal("previousView() returned nil, want ViewEntry")
	}
	if entry.ViewID != model.TaskDetailViewID {
		t.Errorf("previousView() ViewID = %v, want %v", entry.ViewID, model.TaskDetailViewID)
	}

	// Three views - should return second
	nav.push(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{TaskID: "TIKI-5"}))
	entry = nav.previousView()
	if entry == nil {
		t.Fatal("previousView() returned nil")
	}
	if entry.ViewID != model.TaskDetailViewID {
		t.Errorf("previousView() ViewID = %v, want %v", entry.ViewID, model.TaskDetailViewID)
	}

	// Stack should not be modified
	if nav.depth() != 3 {
		t.Error("previousView() should not modify stack")
	}
}

func TestNavigationState_CanGoBack(t *testing.T) {
	nav := newViewStack()

	// Empty stack - cannot go back
	if nav.canGoBack() {
		t.Error("canGoBack() on empty stack should return false")
	}

	// Single view - cannot go back
	nav.push(model.TaskDetailViewID, nil)
	if nav.canGoBack() {
		t.Error("canGoBack() with depth 1 should return false")
	}

	// Two views - can go back
	nav.push(model.TaskDetailViewID, nil)
	if !nav.canGoBack() {
		t.Error("canGoBack() with depth 2 should return true")
	}

	// After pop - cannot go back
	nav.pop()
	if nav.canGoBack() {
		t.Error("canGoBack() after pop to depth 1 should return false")
	}
}

func TestNavigationState_Depth(t *testing.T) {
	nav := newViewStack()

	tests := []struct {
		name          string
		action        func()
		expectedDepth int
	}{
		{
			name:          "initial empty",
			action:        func() {},
			expectedDepth: 0,
		},
		{
			name:          "after first push",
			action:        func() { nav.push(model.TaskDetailViewID, nil) },
			expectedDepth: 1,
		},
		{
			name:          "after second push",
			action:        func() { nav.push(model.TaskDetailViewID, nil) },
			expectedDepth: 2,
		},
		{
			name:          "after third push",
			action:        func() { nav.push(model.TaskDetailViewID, nil) },
			expectedDepth: 3,
		},
		{
			name:          "after one pop",
			action:        func() { nav.pop() },
			expectedDepth: 2,
		},
		{
			name:          "after two pops",
			action:        func() { nav.pop(); nav.pop() },
			expectedDepth: 0,
		},
		{
			name:          "pop on empty stays zero",
			action:        func() { nav.pop() },
			expectedDepth: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.action()
			if nav.depth() != tt.expectedDepth {
				t.Errorf("depth = %d, want %d", nav.depth(), tt.expectedDepth)
			}
		})
	}
}

func TestNavigationState_Clear(t *testing.T) {
	nav := newViewStack()

	// Push multiple views
	nav.push(model.TaskDetailViewID, nil)
	nav.push(model.TaskDetailViewID, nil)
	nav.push(model.TaskEditViewID, nil)

	// Verify stack has items
	if nav.depth() != 3 {
		t.Fatalf("depth = %d, want 3", nav.depth())
	}

	// Clear stack
	nav.clear()

	// Verify empty
	if nav.depth() != 0 {
		t.Errorf("depth after clear() = %d, want 0", nav.depth())
	}

	// Verify operations on cleared stack work correctly
	if nav.currentView() != nil {
		t.Error("currentView() after clear() should return nil")
	}
	if nav.canGoBack() {
		t.Error("canGoBack() after clear() should return false")
	}

	// Should be able to push again
	nav.push(model.TaskDetailViewID, nil)
	if nav.depth() != 1 {
		t.Errorf("depth after push on cleared stack = %d, want 1", nav.depth())
	}
}

func TestNavigationState_ParameterPassing(t *testing.T) {
	nav := newViewStack()

	// Push view with nil params
	nav.push(model.TaskDetailViewID, nil)
	entry := nav.currentView()
	if entry.Params != nil {
		t.Error("nil params should remain nil")
	}

	// Push view with empty params
	nav.push(model.TaskDetailViewID, map[string]interface{}{})
	entry = nav.currentView()
	if entry.Params == nil {
		t.Error("empty params map should not be nil")
	}
	if len(entry.Params) != 0 {
		t.Errorf("empty params length = %d, want 0", len(entry.Params))
	}

	// Push view with multiple params
	params := model.EncodeTaskDetailParams(model.TaskDetailParams{TaskID: "TIKI-42"})
	params["readOnly"] = true
	params["index"] = 123
	nav.push(model.TaskEditViewID, params)
	entry = nav.currentView()

	if model.DecodeTaskDetailParams(entry.Params).TaskID != "TIKI-42" {
		t.Errorf("taskID param = %v, want TIKI-42", model.DecodeTaskDetailParams(entry.Params).TaskID)
	}
	if entry.Params["readOnly"] != true {
		t.Errorf("readOnly param = %v, want true", entry.Params["readOnly"])
	}
	if entry.Params["index"] != 123 {
		t.Errorf("index param = %v, want 123", entry.Params["index"])
	}

	// Pop and verify params are preserved
	entry = nav.pop()
	if model.DecodeTaskDetailParams(entry.Params).TaskID != "TIKI-42" {
		t.Error("params should be preserved through pop()")
	}
}

func TestNavigationState_ComplexNavigationFlow(t *testing.T) {
	nav := newViewStack()

	// Simulate: Board -> open task -> back to board -> edit task -> back to board
	nav.push(model.TaskDetailViewID, nil)
	if nav.currentViewID() != model.TaskDetailViewID {
		t.Fatal("should start on board")
	}

	// Open task from board
	nav.push(model.TaskDetailViewID, model.EncodeTaskDetailParams(model.TaskDetailParams{TaskID: "TIKI-1"}))
	if nav.depth() != 2 {
		t.Error("should have 2 views after opening task")
	}
	if !nav.canGoBack() {
		t.Error("should be able to go back")
	}

	// Back to board
	entry := nav.pop()
	if entry.ViewID != model.TaskDetailViewID {
		t.Error("should pop task detail")
	}
	if nav.currentViewID() != model.TaskDetailViewID {
		t.Error("should return to board")
	}

	// Switch to task edit
	nav.push(model.TaskEditViewID, nil)
	if nav.currentViewID() != model.TaskEditViewID {
		t.Error("should be on task edit")
	}

	// Back to board again
	nav.pop()
	if nav.currentViewID() != model.TaskDetailViewID {
		t.Error("should return to board again")
	}

	// Final state
	if nav.depth() != 1 {
		t.Errorf("final depth = %d, want 1", nav.depth())
	}
	if nav.canGoBack() {
		t.Error("should not be able to go back from single view")
	}
}
