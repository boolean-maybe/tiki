package model

import "sync"

// LayoutModel manages the screen layout state - what content view is displayed.
// Thread-safe model that notifies listeners when content changes.
type LayoutModel struct {
	mu            sync.RWMutex
	contentViewID ViewID
	contentParams map[string]any
	revision      uint64 // monotonically increasing counter for change notifications
	listeners     map[int]func()
	nextListener  int
}

// NewLayoutModel creates a new layout model with default state
func NewLayoutModel() *LayoutModel {
	return &LayoutModel{
		listeners:    make(map[int]func()),
		nextListener: 1,
	}
}

// SetContent updates the current content view and notifies listeners
func (lm *LayoutModel) SetContent(viewID ViewID, params map[string]any) {
	lm.mu.Lock()
	lm.contentViewID = viewID
	lm.contentParams = params
	lm.revision++
	lm.mu.Unlock()
	lm.notifyListeners()
}

// Touch increments the revision and notifies listeners without changing viewID/params.
// Use when the current view's internal UI state changes and RootLayout must recompute
// derived layout (e.g., header visibility after fullscreen toggle).
func (lm *LayoutModel) Touch() {
	lm.mu.Lock()
	lm.revision++
	lm.mu.Unlock()
	lm.notifyListeners()
}

// GetContentViewID returns the current content view identifier
func (lm *LayoutModel) GetContentViewID() ViewID {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.contentViewID
}

// GetContentParams returns the current content view parameters
func (lm *LayoutModel) GetContentParams() map[string]any {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.contentParams
}

// GetRevision returns the current revision counter
func (lm *LayoutModel) GetRevision() uint64 {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.revision
}

// AddListener registers a callback for layout changes.
// Returns a listener ID that can be used to remove the listener.
func (lm *LayoutModel) AddListener(listener func()) int {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	id := lm.nextListener
	lm.nextListener++
	lm.listeners[id] = listener
	return id
}

// RemoveListener removes a previously registered listener by ID
func (lm *LayoutModel) RemoveListener(id int) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	delete(lm.listeners, id)
}

// notifyListeners calls all registered listeners
func (lm *LayoutModel) notifyListeners() {
	lm.mu.RLock()
	listeners := make([]func(), 0, len(lm.listeners))
	for _, listener := range lm.listeners {
		listeners = append(listeners, listener)
	}
	lm.mu.RUnlock()

	for _, listener := range listeners {
		listener()
	}
}
