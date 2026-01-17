package controller

import (
	"github.com/boolean-maybe/tiki/model"
)

// ViewEntry represents a view on the navigation stack with optional parameters
type ViewEntry struct {
	ViewID model.ViewID
	Params map[string]interface{}
}

// viewStack maintains the view stack for Esc-back behavior
type viewStack struct {
	stack []ViewEntry
}

// newViewStack creates a new view stack
func newViewStack() *viewStack {
	return &viewStack{
		stack: make([]ViewEntry, 0),
	}
}

// push adds a view to the stack
func (n *viewStack) push(viewID model.ViewID, params map[string]interface{}) {
	n.stack = append(n.stack, ViewEntry{
		ViewID: viewID,
		Params: params,
	})
}

// replaceTopView replaces the current (top) view with a new one
func (n *viewStack) replaceTopView(viewID model.ViewID, params map[string]interface{}) bool {
	if len(n.stack) == 0 {
		return false
	}
	n.stack[len(n.stack)-1] = ViewEntry{
		ViewID: viewID,
		Params: params,
	}
	return true
}

// pop removes and returns the top view, returns nil if stack is empty
func (n *viewStack) pop() *ViewEntry {
	if len(n.stack) == 0 {
		return nil
	}
	last := n.stack[len(n.stack)-1]
	n.stack = n.stack[:len(n.stack)-1]
	return &last
}

// currentView returns the current (top) view without removing it
func (n *viewStack) currentView() *ViewEntry {
	if len(n.stack) == 0 {
		return nil
	}
	entry := n.stack[len(n.stack)-1]
	return &entry
}

// currentViewID returns just the view ID of the current view
func (n *viewStack) currentViewID() model.ViewID {
	if len(n.stack) == 0 {
		return ""
	}
	return n.stack[len(n.stack)-1].ViewID
}

// previousView returns the view below the current one (for preview purposes)
func (n *viewStack) previousView() *ViewEntry {
	if len(n.stack) < 2 {
		return nil
	}
	entry := n.stack[len(n.stack)-2]
	return &entry
}

// depth returns the current stack depth
func (n *viewStack) depth() int {
	return len(n.stack)
}

// canGoBack returns true if there's a view to go back to
func (n *viewStack) canGoBack() bool {
	return len(n.stack) > 1
}

// clear empties the navigation stack
func (n *viewStack) clear() {
	n.stack = n.stack[:0]
}
