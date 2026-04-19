package model

import "sync"

// ViewContext holds the active view's identity and action DTOs.
// Subscribed to by HeaderWidget and ActionPalette. Written by RootLayout
// via syncViewContextFromView when the active view or its actions change.
type ViewContext struct {
	mu sync.RWMutex

	viewID          ViewID
	viewName        string
	viewDescription string
	viewActions     []HeaderAction
	pluginActions   []HeaderAction

	listeners    map[int]func()
	nextListener int
}

func NewViewContext() *ViewContext {
	return &ViewContext{
		listeners:    make(map[int]func()),
		nextListener: 1,
	}
}

// SetFromView atomically updates all view-context fields and fires exactly one notification.
func (vc *ViewContext) SetFromView(id ViewID, name, description string, viewActions, pluginActions []HeaderAction) {
	vc.mu.Lock()
	vc.viewID = id
	vc.viewName = name
	vc.viewDescription = description
	vc.viewActions = viewActions
	vc.pluginActions = pluginActions
	vc.mu.Unlock()
	vc.notifyListeners()
}

func (vc *ViewContext) GetViewID() ViewID {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.viewID
}

func (vc *ViewContext) GetViewName() string {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.viewName
}

func (vc *ViewContext) GetViewDescription() string {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.viewDescription
}

func (vc *ViewContext) GetViewActions() []HeaderAction {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.viewActions
}

func (vc *ViewContext) GetPluginActions() []HeaderAction {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.pluginActions
}

func (vc *ViewContext) AddListener(listener func()) int {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	id := vc.nextListener
	vc.nextListener++
	vc.listeners[id] = listener
	return id
}

func (vc *ViewContext) RemoveListener(id int) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	delete(vc.listeners, id)
}

func (vc *ViewContext) notifyListeners() {
	vc.mu.RLock()
	listeners := make([]func(), 0, len(vc.listeners))
	for _, listener := range vc.listeners {
		listeners = append(listeners, listener)
	}
	vc.mu.RUnlock()

	for _, listener := range listeners {
		listener()
	}
}
