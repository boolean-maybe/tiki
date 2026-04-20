package view

import (
	"strings"

	"github.com/boolean-maybe/tiki/controller"

	"github.com/rivo/tview"
)

type inputMode int

const (
	inputModeClosed        inputMode = iota
	inputModeSearchEditing           // search box focused, user typing
	inputModeSearchPassive           // search applied, box visible but unfocused
	inputModeActionInput             // action-input box focused, user typing
)

// InputHelper provides reusable input box integration to eliminate duplication across views.
// It tracks an explicit mode state machine:
//
//	closed → searchEditing (on search open)
//	searchEditing → searchPassive (on non-empty Enter)
//	searchEditing → closed (on Esc — clears search)
//	searchPassive → closed (on Esc — clears search)
//	searchPassive → actionInput (on action-input open — temporarily replaces)
//	actionInput → searchPassive (on Enter/Esc if search was passive before)
//	actionInput → closed (on Enter/Esc if no prior search)
type InputHelper struct {
	inputBox         *InputBox
	mode             inputMode
	savedSearchQuery string // saved when action-input temporarily replaces passive search
	onSubmit         func(text string) controller.InputSubmitResult
	onCancel         func()
	onClose          func()             // called when the helper needs the view to rebuild layout (remove widget)
	onRestorePassive func(query string) // called when action-input ends and passive search should be restored
	focusSetter      func(p tview.Primitive)
	contentPrimitive tview.Primitive
}

// NewInputHelper creates a new input helper with an initialized input box
func NewInputHelper(contentPrimitive tview.Primitive) *InputHelper {
	helper := &InputHelper{
		inputBox:         NewInputBox(),
		contentPrimitive: contentPrimitive,
	}

	helper.inputBox.SetSubmitHandler(func(text string) controller.InputSubmitResult {
		if helper.onSubmit == nil {
			return controller.InputClose
		}
		result := helper.onSubmit(text)
		switch result {
		case controller.InputShowPassive:
			helper.mode = inputModeSearchPassive
			helper.inputBox.SetText(strings.TrimSpace(text))
			if helper.focusSetter != nil {
				helper.focusSetter(contentPrimitive)
			}
		case controller.InputClose:
			helper.finishInput()
		}
		return result
	})

	helper.inputBox.SetCancelHandler(func() {
		if helper.onCancel != nil {
			helper.onCancel()
		}
	})

	return helper
}

// finishInput handles InputClose: restores passive search or fully closes.
func (ih *InputHelper) finishInput() {
	if ih.mode == inputModeActionInput && ih.savedSearchQuery != "" {
		query := ih.savedSearchQuery
		ih.savedSearchQuery = ""
		ih.mode = inputModeSearchPassive
		ih.inputBox.SetPrompt("> ")
		ih.inputBox.SetText(query)
		if ih.focusSetter != nil {
			ih.focusSetter(ih.contentPrimitive)
		}
		if ih.onRestorePassive != nil {
			ih.onRestorePassive(query)
		}
		return
	}
	ih.savedSearchQuery = ""
	ih.mode = inputModeClosed
	ih.inputBox.Clear()
	if ih.focusSetter != nil {
		ih.focusSetter(ih.contentPrimitive)
	}
	if ih.onClose != nil {
		ih.onClose()
	}
}

// SetSubmitHandler sets the handler called when user submits input (Enter key).
func (ih *InputHelper) SetSubmitHandler(handler func(text string) controller.InputSubmitResult) {
	ih.onSubmit = handler
}

// SetCancelHandler sets the handler called when user cancels input (Escape key)
func (ih *InputHelper) SetCancelHandler(handler func()) {
	ih.onCancel = handler
}

// SetCloseHandler sets the callback for when the input box should be removed from layout.
func (ih *InputHelper) SetCloseHandler(handler func()) {
	ih.onClose = handler
}

// SetRestorePassiveHandler sets the callback for when passive search should be restored
// after action-input ends (layout may need prompt text update).
func (ih *InputHelper) SetRestorePassiveHandler(handler func(query string)) {
	ih.onRestorePassive = handler
}

// SetFocusSetter sets the function used to change focus between primitives.
func (ih *InputHelper) SetFocusSetter(setter func(p tview.Primitive)) {
	ih.focusSetter = setter
}

// Show makes the input box visible with the given prompt and initial text.
// If the box is in search-passive mode, it saves the search query and
// transitions to action-input mode (temporarily replacing the passive indicator).
func (ih *InputHelper) Show(prompt, initialText string, mode inputMode) tview.Primitive {
	if ih.mode == inputModeSearchPassive && mode == inputModeActionInput {
		ih.savedSearchQuery = ih.inputBox.GetText()
	}
	ih.mode = mode
	ih.inputBox.SetPrompt(prompt)
	ih.inputBox.SetText(initialText)
	return ih.inputBox
}

// ShowSearch makes the input box visible in search-editing mode.
func (ih *InputHelper) ShowSearch(currentQuery string) tview.Primitive {
	return ih.Show("> ", currentQuery, inputModeSearchEditing)
}

// Hide hides the input box and clears its text. Generic teardown only.
func (ih *InputHelper) Hide() {
	ih.mode = inputModeClosed
	ih.savedSearchQuery = ""
	ih.inputBox.Clear()
}

// IsVisible returns true if the input box is currently visible (any mode except closed)
func (ih *InputHelper) IsVisible() bool {
	return ih.mode != inputModeClosed
}

// IsEditing returns true if the input box is in an active editing mode (focused)
func (ih *InputHelper) IsEditing() bool {
	return ih.mode == inputModeSearchEditing || ih.mode == inputModeActionInput
}

// IsSearchPassive returns true if the input box is in search-passive mode
func (ih *InputHelper) IsSearchPassive() bool {
	return ih.mode == inputModeSearchPassive
}

// Mode returns the current input mode
func (ih *InputHelper) Mode() inputMode {
	return ih.mode
}

// HasFocus returns true if the input box currently has focus
func (ih *InputHelper) HasFocus() bool {
	return ih.IsVisible() && ih.inputBox.HasFocus()
}

// GetFocusSetter returns the focus setter function
func (ih *InputHelper) GetFocusSetter() func(p tview.Primitive) {
	return ih.focusSetter
}

// GetInputBox returns the underlying input box primitive for layout building
func (ih *InputHelper) GetInputBox() *InputBox {
	return ih.inputBox
}
