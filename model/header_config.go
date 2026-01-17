package model

import (
	"maps"
	"sync"

	"github.com/boolean-maybe/tiki/store"

	"github.com/gdamore/tcell/v2"
)

// HeaderAction is a controller-free DTO representing an action for the header.
// Used to avoid import cycles between model and controller packages.
type HeaderAction struct {
	ID           string
	Key          tcell.Key
	Rune         rune
	Label        string
	Modifier     tcell.ModMask
	ShowInHeader bool
}

// StatValue represents a single stat entry for the header
type StatValue struct {
	Value    string
	Priority int
}

// HeaderConfig manages ALL header state - both content AND visibility.
// Thread-safe model that notifies listeners when state changes.
type HeaderConfig struct {
	mu sync.RWMutex

	// Content state
	viewActions   []HeaderAction
	pluginActions []HeaderAction
	baseStats     map[string]StatValue // global stats (version, store, user, branch, etc.)
	viewStats     map[string]StatValue // view-specific stats (e.g., board "Total")
	burndown      []store.BurndownPoint

	// Visibility state
	visible        bool // current header visibility (may be overridden by fullscreen view)
	userPreference bool // user's preferred visibility (persisted, used when not fullscreen)

	// Listener management
	listeners    map[int]func()
	nextListener int
}

// NewHeaderConfig creates a new header config with default state
func NewHeaderConfig() *HeaderConfig {
	return &HeaderConfig{
		baseStats:      make(map[string]StatValue),
		viewStats:      make(map[string]StatValue),
		visible:        true,
		userPreference: true,
		listeners:      make(map[int]func()),
		nextListener:   1,
	}
}

// SetViewActions updates the view-specific header actions
func (hc *HeaderConfig) SetViewActions(actions []HeaderAction) {
	hc.mu.Lock()
	hc.viewActions = actions
	hc.mu.Unlock()
	hc.notifyListeners()
}

// GetViewActions returns the current view's header actions
func (hc *HeaderConfig) GetViewActions() []HeaderAction {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.viewActions
}

// SetPluginActions updates the plugin navigation header actions
func (hc *HeaderConfig) SetPluginActions(actions []HeaderAction) {
	hc.mu.Lock()
	hc.pluginActions = actions
	hc.mu.Unlock()
	hc.notifyListeners()
}

// GetPluginActions returns the plugin navigation header actions
func (hc *HeaderConfig) GetPluginActions() []HeaderAction {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.pluginActions
}

// SetBaseStat sets a global stat (displayed in all views)
func (hc *HeaderConfig) SetBaseStat(key, value string, priority int) {
	hc.mu.Lock()
	hc.baseStats[key] = StatValue{Value: value, Priority: priority}
	hc.mu.Unlock()
	hc.notifyListeners()
}

// SetViewStat sets a view-specific stat
func (hc *HeaderConfig) SetViewStat(key, value string, priority int) {
	hc.mu.Lock()
	hc.viewStats[key] = StatValue{Value: value, Priority: priority}
	hc.mu.Unlock()
	hc.notifyListeners()
}

// ClearViewStats clears all view-specific stats
func (hc *HeaderConfig) ClearViewStats() {
	hc.mu.Lock()
	hc.viewStats = make(map[string]StatValue)
	hc.mu.Unlock()
	hc.notifyListeners()
}

// GetStats returns all stats (base + view) merged together
func (hc *HeaderConfig) GetStats() map[string]StatValue {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make(map[string]StatValue)
	maps.Copy(result, hc.baseStats)
	maps.Copy(result, hc.viewStats)
	return result
}

// SetBurndown updates the burndown chart data
func (hc *HeaderConfig) SetBurndown(points []store.BurndownPoint) {
	hc.mu.Lock()
	hc.burndown = points
	hc.mu.Unlock()
	hc.notifyListeners()
}

// GetBurndown returns the current burndown chart data
func (hc *HeaderConfig) GetBurndown() []store.BurndownPoint {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.burndown
}

// SetVisible sets the current header visibility
func (hc *HeaderConfig) SetVisible(visible bool) {
	hc.mu.Lock()
	changed := hc.visible != visible
	hc.visible = visible
	hc.mu.Unlock()
	if changed {
		hc.notifyListeners()
	}
}

// IsVisible returns whether the header is currently visible
func (hc *HeaderConfig) IsVisible() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.visible
}

// SetUserPreference sets the user's preferred visibility
func (hc *HeaderConfig) SetUserPreference(preference bool) {
	hc.mu.Lock()
	hc.userPreference = preference
	hc.mu.Unlock()
}

// GetUserPreference returns the user's preferred visibility
func (hc *HeaderConfig) GetUserPreference() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.userPreference
}

// ToggleUserPreference toggles the user preference and updates visible state
func (hc *HeaderConfig) ToggleUserPreference() {
	hc.mu.Lock()
	hc.userPreference = !hc.userPreference
	hc.visible = hc.userPreference
	hc.mu.Unlock()
	hc.notifyListeners()
}

// AddListener registers a callback for header config changes.
// Returns a listener ID that can be used to remove the listener.
func (hc *HeaderConfig) AddListener(listener func()) int {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	id := hc.nextListener
	hc.nextListener++
	hc.listeners[id] = listener
	return id
}

// RemoveListener removes a previously registered listener by ID
func (hc *HeaderConfig) RemoveListener(id int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.listeners, id)
}

// notifyListeners calls all registered listeners
func (hc *HeaderConfig) notifyListeners() {
	hc.mu.RLock()
	listeners := make([]func(), 0, len(hc.listeners))
	for _, listener := range hc.listeners {
		listeners = append(listeners, listener)
	}
	hc.mu.RUnlock()

	for _, listener := range listeners {
		listener()
	}
}
