package controller

import (
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"

	"github.com/rivo/tview"
)

// View and ViewFactory interfaces decouple controllers from view implementations.

// FocusSettable is implemented by views that need focus management for their subcomponents.
// This is used to wire up tview focus changes when the view needs to transfer focus to
// different primitives (e.g., edit fields, select lists).
type FocusSettable interface {
	SetFocusSetter(setter func(p tview.Primitive))
}

// View represents a renderable view with its action registry
type View interface {
	// GetPrimitive returns the tview primitive for this view
	GetPrimitive() tview.Primitive

	// GetActionRegistry returns the actions available in this view
	GetActionRegistry() *ActionRegistry

	// GetViewID returns the identifier for this view type
	GetViewID() model.ViewID

	// OnFocus is called when the view becomes active
	OnFocus()

	// OnBlur is called when the view becomes inactive
	OnBlur()
}

// ViewFactory creates views on demand
type ViewFactory interface {
	// CreateView instantiates a view by ID with optional parameters
	CreateView(viewID model.ViewID, params map[string]interface{}) View
}

// SelectableView is a view that tracks selection state
type SelectableView interface {
	View

	// GetSelectedID returns the ID of the currently selected item
	GetSelectedID() string

	// SetSelectedID sets the selection to a specific item
	SetSelectedID(id string)
}

// SearchableView is a view that supports search functionality
type SearchableView interface {
	View

	// ShowSearch displays the search box and returns the primitive to focus
	ShowSearch() tview.Primitive

	// HideSearch hides the search box
	HideSearch()

	// IsSearchVisible returns whether the search box is currently visible
	IsSearchVisible() bool

	// IsSearchBoxFocused returns whether the search box currently has focus
	IsSearchBoxFocused() bool

	// SetSearchSubmitHandler sets the callback for when search is submitted
	SetSearchSubmitHandler(handler func(text string))

	// SetFocusSetter sets the callback for requesting focus changes
	SetFocusSetter(setter func(p tview.Primitive))
}

// FullscreenView is a view that can toggle fullscreen rendering
type FullscreenView interface {
	View

	// EnterFullscreen switches the view into fullscreen mode
	EnterFullscreen()

	// ExitFullscreen returns the view to its normal layout
	ExitFullscreen()

	// IsFullscreen reports whether the view is currently fullscreen
	IsFullscreen() bool
}

// FullscreenChangeNotifier is a view that notifies when fullscreen state changes
type FullscreenChangeNotifier interface {
	// SetFullscreenChangeHandler sets the callback for when fullscreen state changes
	SetFullscreenChangeHandler(handler func(isFullscreen bool))
}

// DescriptionEditableView is a view that supports description editing functionality
type DescriptionEditableView interface {
	View

	// ShowDescriptionEditor displays the description text area and returns the primitive to focus
	ShowDescriptionEditor() tview.Primitive

	// HideDescriptionEditor hides the description text area
	HideDescriptionEditor()

	// IsDescriptionEditing returns whether the description is currently being edited
	IsDescriptionEditing() bool

	// IsDescriptionTextAreaFocused returns whether the description text area currently has focus
	IsDescriptionTextAreaFocused() bool

	// SetDescriptionSaveHandler sets the callback for when description is saved
	SetDescriptionSaveHandler(handler func(string))

	// SetDescriptionCancelHandler sets the callback for when description editing is cancelled
	SetDescriptionCancelHandler(handler func())

	// SetFocusSetter sets the callback for requesting focus changes
	SetFocusSetter(setter func(p tview.Primitive))
}

// TitleEditableView is a view that supports title editing functionality
type TitleEditableView interface {
	View

	// ShowTitleEditor displays the title input field and returns the primitive to focus
	ShowTitleEditor() tview.Primitive

	// HideTitleEditor hides the title input field
	HideTitleEditor()

	// IsTitleEditing returns whether the title is currently being edited
	IsTitleEditing() bool

	// IsTitleInputFocused returns whether the title input currently has focus
	IsTitleInputFocused() bool

	// SetTitleSaveHandler sets the callback for when title is saved (explicit save via Enter)
	SetTitleSaveHandler(handler func(string))

	// SetTitleChangeHandler sets the callback for when title changes (auto-save on keystroke)
	SetTitleChangeHandler(handler func(string))

	// SetTitleCancelHandler sets the callback for when title editing is cancelled
	SetTitleCancelHandler(handler func())

	// SetFocusSetter sets the callback for requesting focus changes
	SetFocusSetter(setter func(p tview.Primitive))
}

// TaskEditView exposes edited task fields for save operations
type TaskEditView interface {
	View

	// GetEditedTitle returns the current title text in the editor
	GetEditedTitle() string

	// GetEditedDescription returns the current description text in the editor
	GetEditedDescription() string
}

// FieldFocusableView is a view that supports field-level focus in edit mode
type FieldFocusableView interface {
	View

	// SetFocusedField changes the focused field and re-renders
	SetFocusedField(field model.EditField)

	// GetFocusedField returns the currently focused field
	GetFocusedField() model.EditField

	// FocusNextField advances to the next field in edit order
	FocusNextField() bool

	// FocusPrevField moves to the previous field in edit order
	FocusPrevField() bool

	// IsEditFieldFocused returns whether any editable field has tview focus
	IsEditFieldFocused() bool
}

// ValueCyclableView is a view that supports cycling through field values with arrow keys
type ValueCyclableView interface {
	View

	// CycleFieldValueUp cycles the currently focused field's value upward
	CycleFieldValueUp() bool

	// CycleFieldValueDown cycles the currently focused field's value downward
	CycleFieldValueDown() bool
}

// StatusEditableView is a view that supports status editing functionality
type StatusEditableView interface {
	View

	// SetStatusSaveHandler sets the callback for when status is saved
	SetStatusSaveHandler(handler func(string))
}

// TypeEditableView is a view that supports type editing functionality
type TypeEditableView interface {
	View

	// SetTypeSaveHandler sets the callback for when type is saved
	SetTypeSaveHandler(handler func(string))
}

// PriorityEditableView is a view that supports priority editing functionality
type PriorityEditableView interface {
	View

	// SetPrioritySaveHandler sets the callback for when priority is saved
	SetPrioritySaveHandler(handler func(int))
}

// AssigneeEditableView is a view that supports assignee editing functionality
type AssigneeEditableView interface {
	View

	// SetAssigneeSaveHandler sets the callback for when assignee is saved
	SetAssigneeSaveHandler(handler func(string))
}

// PointsEditableView is a view that supports story points editing functionality
type PointsEditableView interface {
	View

	// SetPointsSaveHandler sets the callback for when story points is saved
	SetPointsSaveHandler(handler func(int))
}

// StatsProvider is a view that provides statistics for the header
type StatsProvider interface {
	// GetStats returns stats to display in the header for this view
	GetStats() []store.Stat
}
