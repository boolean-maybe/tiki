package model

import "sync"

// QuickSelectConfig manages the visibility and callbacks for the QuickSelect overlay.
type QuickSelectConfig struct {
	mu sync.RWMutex

	visible  bool
	onSelect func(taskID string)
	onCancel func()

	listeners    map[int]func()
	nextListener int
}

// NewQuickSelectConfig creates a new config (hidden by default).
func NewQuickSelectConfig() *QuickSelectConfig {
	return &QuickSelectConfig{
		listeners:    make(map[int]func()),
		nextListener: 1,
	}
}

func (qc *QuickSelectConfig) IsVisible() bool {
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	return qc.visible
}

func (qc *QuickSelectConfig) SetVisible(visible bool) {
	qc.mu.Lock()
	changed := qc.visible != visible
	qc.visible = visible
	qc.mu.Unlock()
	if changed {
		qc.notifyListeners()
	}
}

func (qc *QuickSelectConfig) SetOnSelect(fn func(taskID string)) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.onSelect = fn
}

func (qc *QuickSelectConfig) SetOnCancel(fn func()) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.onCancel = fn
}

// Select invokes the select callback with the chosen task ID, then hides.
func (qc *QuickSelectConfig) Select(taskID string) {
	qc.mu.RLock()
	fn := qc.onSelect
	qc.mu.RUnlock()
	if fn != nil {
		fn(taskID)
	}
	qc.SetVisible(false)
}

// Cancel invokes the cancel callback, then hides.
func (qc *QuickSelectConfig) Cancel() {
	qc.mu.RLock()
	fn := qc.onCancel
	qc.mu.RUnlock()
	if fn != nil {
		fn()
	}
	qc.SetVisible(false)
}

func (qc *QuickSelectConfig) AddListener(listener func()) int {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	id := qc.nextListener
	qc.nextListener++
	qc.listeners[id] = listener
	return id
}

func (qc *QuickSelectConfig) RemoveListener(id int) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	delete(qc.listeners, id)
}

func (qc *QuickSelectConfig) notifyListeners() {
	qc.mu.RLock()
	listeners := make([]func(), 0, len(qc.listeners))
	for _, l := range qc.listeners {
		listeners = append(listeners, l)
	}
	qc.mu.RUnlock()

	for _, l := range listeners {
		l()
	}
}
