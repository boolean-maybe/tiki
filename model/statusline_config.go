package model

import (
	"maps"
	"sync"
)

// StatuslineConfig manages statusline state — left stats, right message, and visibility.
// Thread-safe model that notifies listeners when state changes.
type StatuslineConfig struct {
	mu sync.RWMutex

	// left section: base stats (branch, user) + view-specific stats
	leftStats map[string]StatValue
	viewStats map[string]StatValue

	// right section: view-specific stats (right-aligned, e.g. task count)
	rightViewStats map[string]StatValue

	// right section: transient message
	message  string
	level    MessageLevel
	autoHide bool // if true, message is dismissed on next keypress

	// visibility
	visible bool

	// listener management
	listeners    map[int]func()
	nextListener int
}

// NewStatuslineConfig creates a new statusline config with default state
func NewStatuslineConfig() *StatuslineConfig {
	return &StatuslineConfig{
		leftStats:      make(map[string]StatValue),
		viewStats:      make(map[string]StatValue),
		rightViewStats: make(map[string]StatValue),
		level:          MessageLevelInfo,
		visible:        true,
		listeners:      make(map[int]func()),
		nextListener:   1,
	}
}

// SetLeftStat sets a base stat (displayed in all views)
func (sc *StatuslineConfig) SetLeftStat(key, value string, priority int) {
	sc.mu.Lock()
	sc.leftStats[key] = StatValue{Value: value, Priority: priority}
	sc.mu.Unlock()
	sc.notifyListeners()
}

// GetLeftStats returns all left stats (base + view) merged together
func (sc *StatuslineConfig) GetLeftStats() map[string]StatValue {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result := make(map[string]StatValue)
	maps.Copy(result, sc.leftStats)
	maps.Copy(result, sc.viewStats)
	return result
}

// SetViewStat sets a view-specific stat
func (sc *StatuslineConfig) SetViewStat(key, value string, priority int) {
	sc.mu.Lock()
	sc.viewStats[key] = StatValue{Value: value, Priority: priority}
	sc.mu.Unlock()
	sc.notifyListeners()
}

// ClearViewStats clears all view-specific stats
func (sc *StatuslineConfig) ClearViewStats() {
	sc.mu.Lock()
	sc.viewStats = make(map[string]StatValue)
	sc.mu.Unlock()
	sc.notifyListeners()
}

// SetRightViewStat sets a right-aligned view-specific stat (e.g. task count)
func (sc *StatuslineConfig) SetRightViewStat(key, value string, priority int) {
	sc.mu.Lock()
	sc.rightViewStats[key] = StatValue{Value: value, Priority: priority}
	sc.mu.Unlock()
	sc.notifyListeners()
}

// GetRightViewStats returns all right-aligned view stats
func (sc *StatuslineConfig) GetRightViewStats() map[string]StatValue {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result := make(map[string]StatValue, len(sc.rightViewStats))
	maps.Copy(result, sc.rightViewStats)
	return result
}

// ClearRightViewStats clears all right-aligned view-specific stats
func (sc *StatuslineConfig) ClearRightViewStats() {
	sc.mu.Lock()
	sc.rightViewStats = make(map[string]StatValue)
	sc.mu.Unlock()
	sc.notifyListeners()
}

// SetRightViewStats replaces all right view stats in a single notification
func (sc *StatuslineConfig) SetRightViewStats(stats map[string]StatValue) {
	sc.mu.Lock()
	sc.rightViewStats = make(map[string]StatValue, len(stats))
	maps.Copy(sc.rightViewStats, stats)
	sc.mu.Unlock()
	sc.notifyListeners()
}

// SetViewStats replaces all view-specific left stats in a single notification
func (sc *StatuslineConfig) SetViewStats(stats map[string]StatValue) {
	sc.mu.Lock()
	sc.viewStats = make(map[string]StatValue, len(stats))
	maps.Copy(sc.viewStats, stats)
	sc.mu.Unlock()
	sc.notifyListeners()
}

// SetMessage sets the right-section message with a severity level. If autoHide
// is true, the message is dismissed on the next keypress (via DismissAutoHide).
// Setting a message makes the statusline visible.
func (sc *StatuslineConfig) SetMessage(text string, level MessageLevel, autoHide bool) {
	sc.mu.Lock()
	sc.message = text
	sc.level = level
	sc.autoHide = autoHide
	sc.visible = true
	sc.mu.Unlock()
	sc.notifyListeners()
}

// GetMessage returns the current message, its level, and whether auto-hide is active
func (sc *StatuslineConfig) GetMessage() (string, MessageLevel, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.message, sc.level, sc.autoHide
}

// ClearMessage clears the right-section message
func (sc *StatuslineConfig) ClearMessage() {
	sc.mu.Lock()
	changed := sc.message != ""
	sc.message = ""
	sc.level = MessageLevelInfo
	sc.autoHide = false
	sc.mu.Unlock()
	if changed {
		sc.notifyListeners()
	}
}

// DismissAutoHide is called on every keypress. If auto-hide is active,
// it clears the message but keeps the statusline bar visible. Returns true
// if a message was dismissed.
func (sc *StatuslineConfig) DismissAutoHide() bool {
	sc.mu.Lock()
	if !sc.autoHide {
		sc.mu.Unlock()
		return false
	}
	sc.autoHide = false
	sc.message = ""
	sc.level = MessageLevelInfo
	sc.mu.Unlock()
	sc.notifyListeners()
	return true
}

// IsVisible returns whether the statusline is currently visible
func (sc *StatuslineConfig) IsVisible() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.visible
}

// SetVisible sets the statusline visibility
func (sc *StatuslineConfig) SetVisible(visible bool) {
	sc.mu.Lock()
	changed := sc.visible != visible
	sc.visible = visible
	sc.mu.Unlock()
	if changed {
		sc.notifyListeners()
	}
}

// AddListener registers a callback for statusline config changes.
// Returns a listener ID that can be used to remove the listener.
func (sc *StatuslineConfig) AddListener(listener func()) int {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	id := sc.nextListener
	sc.nextListener++
	sc.listeners[id] = listener
	return id
}

// RemoveListener removes a previously registered listener by ID
func (sc *StatuslineConfig) RemoveListener(id int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.listeners, id)
}

// notifyListeners calls all registered listeners.
// listeners are called outside the lock, so they may safely re-enter
// StatuslineConfig methods (e.g., SetLeftStat, SetVisible) without deadlocking.
func (sc *StatuslineConfig) notifyListeners() {
	sc.mu.RLock()
	listeners := make([]func(), 0, len(sc.listeners))
	for _, listener := range sc.listeners {
		listeners = append(listeners, listener)
	}
	sc.mu.RUnlock()

	for _, listener := range listeners {
		listener()
	}
}
