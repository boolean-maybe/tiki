package view

import (
	"github.com/rivo/tview"
)

// SearchHelper provides reusable search box integration to eliminate duplication across views
type SearchHelper struct {
	searchBox      *SearchBox
	searchVisible  bool
	onSearchSubmit func(text string)
	focusSetter    func(p tview.Primitive)
}

// NewSearchHelper creates a new search helper with an initialized search box
func NewSearchHelper(contentPrimitive tview.Primitive) *SearchHelper {
	helper := &SearchHelper{
		searchBox: NewSearchBox(),
	}

	// Wire up internal handlers - the search box will call these
	helper.searchBox.SetSubmitHandler(func(text string) {
		if helper.onSearchSubmit != nil {
			helper.onSearchSubmit(text)
		}
		// Transfer focus back to content after search
		if helper.focusSetter != nil {
			helper.focusSetter(contentPrimitive)
		}
	})

	return helper
}

// SetSubmitHandler sets the handler called when user submits a search query
// This is typically wired to the controller's HandleSearch method
func (sh *SearchHelper) SetSubmitHandler(handler func(text string)) {
	sh.onSearchSubmit = handler
}

// SetCancelHandler sets the handler called when user cancels search (Escape key)
// This is typically wired to the view's HideSearch method
func (sh *SearchHelper) SetCancelHandler(handler func()) {
	sh.searchBox.SetCancelHandler(handler)
}

// SetFocusSetter sets the function used to change focus between primitives
// This is typically app.SetFocus and is provided by the InputRouter
func (sh *SearchHelper) SetFocusSetter(setter func(p tview.Primitive)) {
	sh.focusSetter = setter
}

// ShowSearch makes the search box visible and returns it for focus management
// currentQuery: the query text to restore (e.g., when returning from task detail)
func (sh *SearchHelper) ShowSearch(currentQuery string) tview.Primitive {
	sh.searchVisible = true
	sh.searchBox.SetText(currentQuery)
	return sh.searchBox
}

// HideSearch clears and hides the search box
func (sh *SearchHelper) HideSearch() {
	sh.searchVisible = false
	sh.searchBox.Clear()
}

// IsVisible returns true if the search box is currently visible
func (sh *SearchHelper) IsVisible() bool {
	return sh.searchVisible
}

// HasFocus returns true if the search box currently has focus
func (sh *SearchHelper) HasFocus() bool {
	return sh.searchVisible && sh.searchBox.HasFocus()
}

// GetSearchBox returns the underlying search box primitive for layout building
func (sh *SearchHelper) GetSearchBox() *SearchBox {
	return sh.searchBox
}
