package controller

import (
	"github.com/boolean-maybe/tiki/model"

	"github.com/gdamore/tcell/v2"
)

// ActionRegistry maps keyboard shortcuts to actions and matches key events.

// ActionID identifies a specific action
type ActionID string

// ActionID values for global actions (available in all views).
const (
	ActionBack           ActionID = "back"
	ActionQuit           ActionID = "quit"
	ActionRefresh        ActionID = "refresh"
	ActionToggleViewMode ActionID = "toggle_view_mode"
	ActionToggleHeader   ActionID = "toggle_header"
)

// ActionID values for task navigation and manipulation (used by plugins).
const (
	ActionOpenTask      ActionID = "open_task"
	ActionMoveTask      ActionID = "move_task"
	ActionMoveTaskLeft  ActionID = "move_task_left"
	ActionMoveTaskRight ActionID = "move_task_right"
	ActionNewTask       ActionID = "new_task"
	ActionDeleteTask    ActionID = "delete_task"
	ActionNavLeft       ActionID = "nav_left"
	ActionNavRight      ActionID = "nav_right"
	ActionNavUp         ActionID = "nav_up"
	ActionNavDown       ActionID = "nav_down"
)

// ActionID values for task detail view actions.
const (
	ActionEditTitle  ActionID = "edit_title"
	ActionEditSource ActionID = "edit_source"
	ActionFullscreen ActionID = "fullscreen"
	ActionCloneTask  ActionID = "clone_task"
)

// ActionID values for task edit view actions.
const (
	ActionSaveTask  ActionID = "save_task"
	ActionQuickSave ActionID = "quick_save"
	ActionNextField ActionID = "next_field"
	ActionPrevField ActionID = "prev_field"
	ActionNextValue ActionID = "next_value" // Navigate to next value in a picker (down arrow)
	ActionPrevValue ActionID = "prev_value" // Navigate to previous value in a picker (up arrow)
)

// ActionID values for search.
const (
	ActionSearch ActionID = "search"
)

// ActionID values for plugin view actions.
const (
	ActionOpenFromPlugin ActionID = "open_from_plugin"
)

// ActionID values for doki plugin (markdown navigation) actions.
const (
	ActionNavigateBack    ActionID = "navigate_back"
	ActionNavigateForward ActionID = "navigate_forward"
)

// PluginInfo provides the minimal info needed to register plugin actions.
// Avoids import cycle between controller and plugin packages.
type PluginInfo struct {
	Name     string
	Key      tcell.Key
	Rune     rune
	Modifier tcell.ModMask
}

// pluginActionRegistry holds plugin navigation actions (populated at init time)
var pluginActionRegistry *ActionRegistry

// InitPluginActions creates the plugin action registry from loaded plugins.
// Called once during app initialization after plugins are loaded.
func InitPluginActions(plugins []PluginInfo) {
	pluginActionRegistry = NewActionRegistry()
	for _, p := range plugins {
		if p.Key == 0 && p.Rune == 0 {
			continue // skip plugins without key binding
		}
		pluginActionRegistry.Register(Action{
			ID:           ActionID("plugin:" + p.Name),
			Key:          p.Key,
			Rune:         p.Rune,
			Modifier:     p.Modifier,
			Label:        p.Name,
			ShowInHeader: true,
		})
	}
}

// GetPluginActions returns the plugin action registry
func GetPluginActions() *ActionRegistry {
	if pluginActionRegistry == nil {
		return NewActionRegistry() // empty if not initialized
	}
	return pluginActionRegistry
}

// GetPluginNameFromAction extracts the plugin name from a plugin action ID.
// Returns empty string if the action is not a plugin action.
func GetPluginNameFromAction(id ActionID) string {
	const prefix = "plugin:"
	s := string(id)
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return ""
}

// Action represents a keyboard shortcut binding
type Action struct {
	ID           ActionID
	Key          tcell.Key
	Rune         rune // for letter keys (when Key == tcell.KeyRune)
	Label        string
	Modifier     tcell.ModMask
	ShowInHeader bool // whether to display in header bar
}

// ActionRegistry holds the available actions for a view.
// Uses a space-time tradeoff: stores actions in 3 places for different purposes:
// - actions slice preserves registration order (needed for header display)
// - byKey/byRune maps provide O(1) lookups for keyboard matching (vs O(n) linear search)
type ActionRegistry struct {
	actions []Action             // All registered actions in order
	byKey   map[tcell.Key]Action // Fast lookup for special keys (arrow keys, function keys, etc.)
	byRune  map[rune]Action      // Fast lookup for character keys (letters, symbols)
}

// NewActionRegistry creates a new action registry
func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		actions: make([]Action, 0),
		byKey:   make(map[tcell.Key]Action),
		byRune:  make(map[rune]Action),
	}
}

// Register adds an action to the registry
func (r *ActionRegistry) Register(action Action) {
	r.actions = append(r.actions, action)
	if action.Key == tcell.KeyRune {
		r.byRune[action.Rune] = action
	} else {
		r.byKey[action.Key] = action
	}
}

// Merge adds all actions from another registry into this one.
// Actions from the other registry are appended to preserve order.
// If there are key conflicts, the other registry's actions take precedence.
func (r *ActionRegistry) Merge(other *ActionRegistry) {
	for _, action := range other.actions {
		r.Register(action)
	}
}

// MergePluginActions adds all plugin activation actions to this registry.
// Called after plugins are loaded to add dynamic plugin keys to view registries.
func (r *ActionRegistry) MergePluginActions() {
	if pluginActionRegistry != nil {
		r.Merge(pluginActionRegistry)
	}
}

// GetActions returns all registered actions
func (r *ActionRegistry) GetActions() []Action {
	return r.actions
}

// Match finds an action matching the given key event
func (r *ActionRegistry) Match(event *tcell.EventKey) *Action {
	// normalize modifier (ignore caps lock, num lock, etc.)
	mod := event.Modifiers() & (tcell.ModShift | tcell.ModCtrl | tcell.ModAlt | tcell.ModMeta)

	for i := range r.actions {
		action := &r.actions[i]

		if event.Key() == tcell.KeyRune {
			// for printable characters, match by rune first
			if action.Key == tcell.KeyRune && action.Rune == event.Rune() {
				// if action has explicit modifiers, require exact match
				if action.Modifier != 0 && action.Modifier != mod {
					continue // modifier mismatch, try next action
				}
				return action
			}
		} else {
			// for special keys, require exact modifier match
			if action.Key == event.Key() && action.Modifier == mod {
				return action
			}
			// Handle Ctrl+letter: tcell sends key='A'-'Z' with ModCtrl,
			// but actions may register KeyCtrlA-KeyCtrlZ (1-26)
			if mod == tcell.ModCtrl && action.Modifier == tcell.ModCtrl {
				var ctrlKeyCode tcell.Key
				if event.Key() >= 'A' && event.Key() <= 'Z' {
					ctrlKeyCode = event.Key() - 'A' + 1
				} else if event.Key() >= 'a' && event.Key() <= 'z' {
					ctrlKeyCode = event.Key() - 'a' + 1
				}
				if ctrlKeyCode != 0 && ctrlKeyCode == action.Key {
					return action
				}
			}
		}
	}
	return nil
}

// DefaultGlobalActions returns common actions available in all views
func DefaultGlobalActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionBack, Key: tcell.KeyEscape, Label: "Back", ShowInHeader: true})
	r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit", ShowInHeader: true})
	r.Register(Action{ID: ActionRefresh, Key: tcell.KeyRune, Rune: 'r', Label: "Refresh", ShowInHeader: true})
	r.Register(Action{ID: ActionToggleHeader, Key: tcell.KeyF10, Label: "Hide Header", ShowInHeader: true})
	return r
}

// GetHeaderActions returns only actions marked for header display
func (r *ActionRegistry) GetHeaderActions() []Action {
	var result []Action
	for _, a := range r.actions {
		if a.ShowInHeader {
			result = append(result, a)
		}
	}
	return result
}

// TaskDetailViewActions returns the canonical action registry for the task detail view.
// Single source of truth for both input handling and header display.
func TaskDetailViewActions() *ActionRegistry {
	r := NewActionRegistry()

	r.Register(Action{ID: ActionEditTitle, Key: tcell.KeyRune, Rune: 'e', Label: "Edit", ShowInHeader: true})
	r.Register(Action{ID: ActionEditSource, Key: tcell.KeyRune, Rune: 's', Label: "Edit source", ShowInHeader: true})
	r.Register(Action{ID: ActionFullscreen, Key: tcell.KeyRune, Rune: 'f', Label: "Full screen", ShowInHeader: true})
	// Clone action removed - not yet implemented

	return r
}

// TaskEditViewActions returns the canonical action registry for the task edit view.
// Separate registry so view/edit modes can diverge while sharing rendering helpers.
func TaskEditViewActions() *ActionRegistry {
	r := NewActionRegistry()

	r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
	r.Register(Action{ID: ActionNextField, Key: tcell.KeyTab, Label: "Next", ShowInHeader: true})
	r.Register(Action{ID: ActionPrevField, Key: tcell.KeyBacktab, Label: "Prev", ShowInHeader: true})

	return r
}

// CommonFieldNavigationActions returns actions available in all field editors (Tab/Shift-Tab navigation)
func CommonFieldNavigationActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionNextField, Key: tcell.KeyTab, Label: "Next field", ShowInHeader: true})
	r.Register(Action{ID: ActionPrevField, Key: tcell.KeyBacktab, Label: "Prev field", ShowInHeader: true})
	return r
}

// TaskEditTitleActions returns actions available when editing the title field
func TaskEditTitleActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionQuickSave, Key: tcell.KeyEnter, Label: "Quick Save", ShowInHeader: true})
	r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
	r.Merge(CommonFieldNavigationActions())
	return r
}

// TaskEditStatusActions returns actions available when editing the status field
func TaskEditStatusActions() *ActionRegistry {
	r := CommonFieldNavigationActions()
	r.Register(Action{ID: ActionNextValue, Key: tcell.KeyDown, Label: "Next ↓", ShowInHeader: true})
	r.Register(Action{ID: ActionPrevValue, Key: tcell.KeyUp, Label: "Prev ↑", ShowInHeader: true})
	return r
}

// TaskEditTypeActions returns actions available when editing the type field
func TaskEditTypeActions() *ActionRegistry {
	r := CommonFieldNavigationActions()
	r.Register(Action{ID: ActionNextValue, Key: tcell.KeyDown, Label: "Next ↓", ShowInHeader: true})
	r.Register(Action{ID: ActionPrevValue, Key: tcell.KeyUp, Label: "Prev ↑", ShowInHeader: true})
	return r
}

// TaskEditPriorityActions returns actions available when editing the priority field
func TaskEditPriorityActions() *ActionRegistry {
	r := CommonFieldNavigationActions()
	// Future: Add ActionChangePriority when priority editor is implemented
	return r
}

// TaskEditAssigneeActions returns actions available when editing the assignee field
func TaskEditAssigneeActions() *ActionRegistry {
	r := CommonFieldNavigationActions()
	r.Register(Action{ID: ActionNextValue, Key: tcell.KeyDown, Label: "Next ↓", ShowInHeader: true})
	r.Register(Action{ID: ActionPrevValue, Key: tcell.KeyUp, Label: "Prev ↑", ShowInHeader: true})
	return r
}

// TaskEditPointsActions returns actions available when editing the story points field
func TaskEditPointsActions() *ActionRegistry {
	r := CommonFieldNavigationActions()
	// Future: Add ActionChangePoints when points editor is implemented
	return r
}

// TaskEditDescriptionActions returns actions available when editing the description field
func TaskEditDescriptionActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
	r.Merge(CommonFieldNavigationActions())
	return r
}

// GetActionsForField returns the appropriate action registry for the given edit field
func GetActionsForField(field model.EditField) *ActionRegistry {
	switch field {
	case model.EditFieldTitle:
		return TaskEditTitleActions()
	case model.EditFieldStatus:
		return TaskEditStatusActions()
	case model.EditFieldType:
		return TaskEditTypeActions()
	case model.EditFieldPriority:
		return TaskEditPriorityActions()
	case model.EditFieldAssignee:
		return TaskEditAssigneeActions()
	case model.EditFieldPoints:
		return TaskEditPointsActions()
	case model.EditFieldDescription:
		return TaskEditDescriptionActions()
	default:
		// default to title actions if field is unknown
		return TaskEditTitleActions()
	}
}

// PluginViewActions returns the canonical action registry for plugin views.
// Similar to backlog view but without sprint-specific actions.
func PluginViewActions() *ActionRegistry {
	r := NewActionRegistry()

	// navigation (not shown in header)
	r.Register(Action{ID: ActionNavUp, Key: tcell.KeyUp, Label: "↑"})
	r.Register(Action{ID: ActionNavDown, Key: tcell.KeyDown, Label: "↓"})
	r.Register(Action{ID: ActionNavLeft, Key: tcell.KeyLeft, Label: "←"})
	r.Register(Action{ID: ActionNavRight, Key: tcell.KeyRight, Label: "→"})
	r.Register(Action{ID: ActionNavUp, Key: tcell.KeyRune, Rune: 'k', Label: "↑"})
	r.Register(Action{ID: ActionNavDown, Key: tcell.KeyRune, Rune: 'j', Label: "↓"})
	r.Register(Action{ID: ActionNavLeft, Key: tcell.KeyRune, Rune: 'h', Label: "←"})
	r.Register(Action{ID: ActionNavRight, Key: tcell.KeyRune, Rune: 'l', Label: "→"})

	// plugin actions (shown in header)
	r.Register(Action{ID: ActionOpenFromPlugin, Key: tcell.KeyEnter, Label: "Open", ShowInHeader: true})
	r.Register(Action{ID: ActionMoveTaskLeft, Key: tcell.KeyLeft, Modifier: tcell.ModShift, Label: "Move ←", ShowInHeader: true})
	r.Register(Action{ID: ActionMoveTaskRight, Key: tcell.KeyRight, Modifier: tcell.ModShift, Label: "Move →", ShowInHeader: true})
	r.Register(Action{ID: ActionNewTask, Key: tcell.KeyRune, Rune: 'n', Label: "New", ShowInHeader: true})
	r.Register(Action{ID: ActionDeleteTask, Key: tcell.KeyRune, Rune: 'd', Label: "Delete", ShowInHeader: true})
	r.Register(Action{ID: ActionSearch, Key: tcell.KeyRune, Rune: '/', Label: "Search", ShowInHeader: true})
	r.Register(Action{ID: ActionToggleViewMode, Key: tcell.KeyRune, Rune: 'v', Label: "View mode", ShowInHeader: true})

	// plugin activation keys are merged dynamically after plugins load
	r.MergePluginActions()

	return r
}

// DokiViewActions returns the action registry for doki (documentation) plugin views.
// Doki views primarily handle navigation through the NavigableMarkdown component.
func DokiViewActions() *ActionRegistry {
	r := NewActionRegistry()

	// Navigation actions (handled by the NavigableMarkdown component in the view)
	// These are registered here for consistency, but actual handling is in the view
	// Note: The navidown component supports both plain Left/Right and Alt+Left/Right
	// We register plain arrows since they're simpler and context-sensitive (no conflicts)
	r.Register(Action{ID: ActionNavigateBack, Key: tcell.KeyLeft, Label: "← Back", ShowInHeader: true})
	r.Register(Action{ID: ActionNavigateForward, Key: tcell.KeyRight, Label: "Forward →", ShowInHeader: true})

	// plugin activation keys are merged dynamically after plugins load
	r.MergePluginActions()

	return r
}
