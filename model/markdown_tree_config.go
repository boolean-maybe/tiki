package model

import "sync"

// MarkdownTreeConfig manages visibility and callbacks for the markdown-file
// tree overlay.
type MarkdownTreeConfig struct {
	mu sync.RWMutex

	visible  bool
	onSelect func(relPath string)
	onCancel func()

	listeners    map[int]func()
	nextListener int
}

// NewMarkdownTreeConfig creates a new config (hidden by default).
func NewMarkdownTreeConfig() *MarkdownTreeConfig {
	return &MarkdownTreeConfig{
		listeners:    make(map[int]func()),
		nextListener: 1,
	}
}

func (mc *MarkdownTreeConfig) IsVisible() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.visible
}

func (mc *MarkdownTreeConfig) SetVisible(visible bool) {
	mc.mu.Lock()
	changed := mc.visible != visible
	mc.visible = visible
	mc.mu.Unlock()
	if changed {
		mc.notifyListeners()
	}
}

func (mc *MarkdownTreeConfig) SetOnSelect(fn func(relPath string)) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.onSelect = fn
}

func (mc *MarkdownTreeConfig) SetOnCancel(fn func()) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.onCancel = fn
}

// Select hides the overlay, then invokes the select callback with the chosen
// relative path. Hiding first is deliberate: the visibility listener restores
// the pre-overlay focus, so it must run before onSelect pushes a new view and
// focuses it — otherwise the restore would clobber the pushed view's focus.
func (mc *MarkdownTreeConfig) Select(relPath string) {
	mc.mu.RLock()
	fn := mc.onSelect
	mc.mu.RUnlock()
	mc.SetVisible(false)
	if fn != nil {
		fn(relPath)
	}
}

// Cancel invokes the cancel callback, then hides.
func (mc *MarkdownTreeConfig) Cancel() {
	mc.mu.RLock()
	fn := mc.onCancel
	mc.mu.RUnlock()
	if fn != nil {
		fn()
	}
	mc.SetVisible(false)
}

func (mc *MarkdownTreeConfig) AddListener(listener func()) int {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	id := mc.nextListener
	mc.nextListener++
	mc.listeners[id] = listener
	return id
}

func (mc *MarkdownTreeConfig) RemoveListener(id int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	delete(mc.listeners, id)
}

func (mc *MarkdownTreeConfig) notifyListeners() {
	mc.mu.RLock()
	listeners := make([]func(), 0, len(mc.listeners))
	for _, l := range mc.listeners {
		listeners = append(listeners, l)
	}
	mc.mu.RUnlock()

	for _, l := range listeners {
		l()
	}
}
