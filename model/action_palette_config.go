package model

import "sync"

// ActionPaletteConfig manages the visibility state of the action palette overlay.
// The palette reads view metadata from ViewContext and action rows from live
// controller registries — this config only tracks open/close state.
type ActionPaletteConfig struct {
	mu sync.RWMutex

	visible bool

	listeners    map[int]func()
	nextListener int
}

// NewActionPaletteConfig creates a new palette config (hidden by default).
func NewActionPaletteConfig() *ActionPaletteConfig {
	return &ActionPaletteConfig{
		listeners:    make(map[int]func()),
		nextListener: 1,
	}
}

// IsVisible returns whether the palette is currently visible.
func (pc *ActionPaletteConfig) IsVisible() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.visible
}

// SetVisible sets the palette visibility and notifies listeners on change.
func (pc *ActionPaletteConfig) SetVisible(visible bool) {
	pc.mu.Lock()
	changed := pc.visible != visible
	pc.visible = visible
	pc.mu.Unlock()
	if changed {
		pc.notifyListeners()
	}
}

// ToggleVisible toggles the palette visibility.
func (pc *ActionPaletteConfig) ToggleVisible() {
	pc.mu.Lock()
	pc.visible = !pc.visible
	pc.mu.Unlock()
	pc.notifyListeners()
}

// AddListener registers a callback for palette config changes.
// Returns a listener ID for removal.
func (pc *ActionPaletteConfig) AddListener(listener func()) int {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	id := pc.nextListener
	pc.nextListener++
	pc.listeners[id] = listener
	return id
}

// RemoveListener removes a previously registered listener by ID.
func (pc *ActionPaletteConfig) RemoveListener(id int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.listeners, id)
}

func (pc *ActionPaletteConfig) notifyListeners() {
	pc.mu.RLock()
	listeners := make([]func(), 0, len(pc.listeners))
	for _, listener := range pc.listeners {
		listeners = append(listeners, listener)
	}
	pc.mu.RUnlock()

	for _, listener := range listeners {
		listener()
	}
}
