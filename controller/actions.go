package controller

import (
	"github.com/boolean-maybe/tiki/config"
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
	ActionOpenPalette    ActionID = "open_palette"
)

// ActionID values for task navigation and manipulation (used by plugins).
const (
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
	ActionEditDesc   ActionID = "edit_desc"
	ActionFullscreen ActionID = "fullscreen"
	ActionCloneTask  ActionID = "clone_task"
	ActionEditDeps   ActionID = "edit_deps"
	ActionEditTags   ActionID = "edit_tags"
	ActionChat       ActionID = "chat"
)

// ActionID values for task edit view actions.
const (
	ActionSaveTask   ActionID = "save_task"
	ActionQuickSave  ActionID = "quick_save"
	ActionNextField  ActionID = "next_field"
	ActionPrevField  ActionID = "prev_field"
	ActionNextValue  ActionID = "next_value"  // Navigate to next value in a picker (down arrow)
	ActionPrevValue  ActionID = "prev_value"  // Navigate to previous value in a picker (up arrow)
	ActionClearField ActionID = "clear_field" // Clear the current field value
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
		pluginViewID := model.MakePluginViewID(p.Name)
		pluginActionRegistry.Register(Action{
			ID:           ActionID("plugin:" + p.Name),
			Key:          p.Key,
			Rune:         p.Rune,
			Modifier:     p.Modifier,
			Label:        p.Name,
			ShowInHeader: true,
			IsEnabled: func(view *ViewEntry, _ View) bool {
				if view == nil {
					return true
				}
				return view.ViewID != pluginViewID
			},
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
	ID              ActionID
	Key             tcell.Key
	Rune            rune // for letter keys (when Key == tcell.KeyRune)
	Label           string
	Modifier        tcell.ModMask
	ShowInHeader    bool // whether to display in header bar
	HideFromPalette bool // when true, action is excluded from the action palette (zero value = visible)
	IsEnabled       func(view *ViewEntry, activeView View) bool
}

// keyWithMod is a composite map key for special-key lookups, disambiguating
// the same key registered with different modifiers (e.g. Left vs Shift+Left).
type keyWithMod struct {
	key tcell.Key
	mod tcell.ModMask
}

// runeWithMod is a composite map key for rune lookups, disambiguating
// the same rune registered with different modifiers (e.g. 'M' vs Alt+'M').
type runeWithMod struct {
	ch  rune
	mod tcell.ModMask
}

// ActionRegistry holds the available actions for a view.
// - actions slice preserves registration order (needed for header display)
// - byKey/byRune maps provide O(1) lookups for keyboard matching
type ActionRegistry struct {
	actions []Action               // all registered actions in order
	byKey   map[keyWithMod]Action  // fast lookup for special keys (arrow keys, function keys, etc.)
	byRune  map[runeWithMod]Action // fast lookup for character keys (letters, symbols)
}

// NewActionRegistry creates a new action registry
func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		actions: make([]Action, 0),
		byKey:   make(map[keyWithMod]Action),
		byRune:  make(map[runeWithMod]Action),
	}
}

// Register adds an action to the registry
func (r *ActionRegistry) Register(action Action) {
	r.actions = append(r.actions, action)
	mod := action.Modifier & (tcell.ModShift | tcell.ModCtrl | tcell.ModAlt | tcell.ModMeta)
	if action.Key == tcell.KeyRune {
		r.byRune[runeWithMod{action.Rune, mod}] = action
	} else {
		r.byKey[keyWithMod{action.Key, mod}] = action
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

// LookupRune returns the action registered for the given rune (with no modifier), if any.
func (r *ActionRegistry) LookupRune(ch rune) (Action, bool) {
	a, ok := r.byRune[runeWithMod{ch, 0}]
	return a, ok
}

// Match finds an action matching the given key event using O(1) map lookups.
func (r *ActionRegistry) Match(event *tcell.EventKey) *Action {
	// normalize modifier (ignore caps lock, num lock, etc.)
	mod := event.Modifiers() & (tcell.ModShift | tcell.ModCtrl | tcell.ModAlt | tcell.ModMeta)

	if event.Key() == tcell.KeyRune {
		// exact rune+modifier lookup
		if a, ok := r.byRune[runeWithMod{event.Rune(), mod}]; ok {
			return &a
		}
		// rune actions registered without a modifier match any modifier
		if mod != 0 {
			if a, ok := r.byRune[runeWithMod{event.Rune(), 0}]; ok && a.Modifier == 0 {
				return &a
			}
		}
		return nil
	}

	// special keys — exact key+modifier lookup
	if a, ok := r.byKey[keyWithMod{event.Key(), mod}]; ok {
		return &a
	}

	// Ctrl+letter fallback: tcell may send Key='A'-'Z' with ModCtrl,
	// but actions may register KeyCtrlA-KeyCtrlZ (1-26)
	if mod == tcell.ModCtrl {
		var ctrlKeyCode tcell.Key
		if event.Key() >= 'A' && event.Key() <= 'Z' {
			ctrlKeyCode = event.Key() - 'A' + 1
		} else if event.Key() >= 'a' && event.Key() <= 'z' {
			ctrlKeyCode = event.Key() - 'a' + 1
		}
		if ctrlKeyCode != 0 {
			if a, ok := r.byKey[keyWithMod{ctrlKeyCode, tcell.ModCtrl}]; ok {
				return &a
			}
		}
	}

	return nil
}

// selectionRequired is an IsEnabled predicate that returns true only when
// the active view has a non-empty selection (for use with plugin/deps actions).
func selectionRequired(_ *ViewEntry, activeView View) bool {
	if sv, ok := activeView.(SelectableView); ok {
		return sv.GetSelectedID() != ""
	}
	return false
}

// DefaultGlobalActions returns common actions available in all views
func DefaultGlobalActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionBack, Key: tcell.KeyEscape, Label: "Back", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit", ShowInHeader: true})
	r.Register(Action{ID: ActionRefresh, Key: tcell.KeyRune, Rune: 'r', Label: "Refresh", ShowInHeader: true})
	r.Register(Action{ID: ActionToggleHeader, Key: tcell.KeyF10, Label: "Toggle Header", ShowInHeader: true})
	r.Register(Action{ID: ActionOpenPalette, Key: tcell.KeyCtrlA, Modifier: tcell.ModCtrl, Label: "All", ShowInHeader: true, HideFromPalette: true})
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

// GetPaletteActions returns palette-visible actions, deduped by ActionID (first registration wins).
func (r *ActionRegistry) GetPaletteActions() []Action {
	if r == nil {
		return nil
	}
	seen := make(map[ActionID]bool)
	var result []Action
	for _, a := range r.actions {
		if a.HideFromPalette {
			continue
		}
		if seen[a.ID] {
			continue
		}
		seen[a.ID] = true
		result = append(result, a)
	}
	return result
}

// ContainsID returns true if the registry has an action with the given ID.
func (r *ActionRegistry) ContainsID(id ActionID) bool {
	if r == nil {
		return false
	}
	for _, a := range r.actions {
		if a.ID == id {
			return true
		}
	}
	return false
}

// ToHeaderActions converts the registry's header actions to model.HeaderAction slice.
// This bridges the controller→model boundary without requiring callers to do the mapping.
func (r *ActionRegistry) ToHeaderActions() []model.HeaderAction {
	if r == nil {
		return nil
	}
	actions := r.GetHeaderActions()
	result := make([]model.HeaderAction, len(actions))
	for i, a := range actions {
		result[i] = model.HeaderAction{
			ID:           string(a.ID),
			Key:          a.Key,
			Rune:         a.Rune,
			Label:        a.Label,
			Modifier:     a.Modifier,
			ShowInHeader: a.ShowInHeader,
		}
	}
	return result
}

// TaskDetailViewActions returns the canonical action registry for the task detail view.
// Single source of truth for both input handling and header display.
func TaskDetailViewActions() *ActionRegistry {
	r := NewActionRegistry()

	taskDetailEnabled := func(view *ViewEntry, _ View) bool {
		if view == nil || view.ViewID != model.TaskDetailViewID {
			return false
		}
		return model.DecodeTaskDetailParams(view.Params).TaskID != ""
	}

	r.Register(Action{ID: ActionEditTitle, Key: tcell.KeyRune, Rune: 'e', Label: "Edit", ShowInHeader: true, IsEnabled: taskDetailEnabled})
	r.Register(Action{ID: ActionEditDesc, Key: tcell.KeyRune, Rune: 'D', Label: "Edit desc", ShowInHeader: true, IsEnabled: taskDetailEnabled})
	r.Register(Action{ID: ActionEditSource, Key: tcell.KeyRune, Rune: 's', Label: "Edit source", ShowInHeader: true, IsEnabled: taskDetailEnabled})
	r.Register(Action{ID: ActionFullscreen, Key: tcell.KeyRune, Rune: 'f', Label: "Full screen", ShowInHeader: true})
	r.Register(Action{ID: ActionEditDeps, Key: tcell.KeyCtrlD, Modifier: tcell.ModCtrl, Label: "Dependencies", ShowInHeader: true, IsEnabled: taskDetailEnabled})
	r.Register(Action{ID: ActionEditTags, Key: tcell.KeyRune, Rune: 'T', Label: "Edit tags", ShowInHeader: true, IsEnabled: taskDetailEnabled})

	if config.GetAIAgent() != "" {
		r.Register(Action{ID: ActionChat, Key: tcell.KeyRune, Rune: 'c', Label: "Chat", ShowInHeader: true, IsEnabled: taskDetailEnabled})
	}

	return r
}

// ReadonlyTaskDetailViewActions returns a reduced registry for readonly task detail views.
// Only fullscreen toggle is available — no editing actions.
func ReadonlyTaskDetailViewActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionFullscreen, Key: tcell.KeyRune, Rune: 'f', Label: "Full screen", ShowInHeader: true})
	return r
}

// TaskEditViewActions returns the canonical action registry for the task edit view.
// Separate registry so view/edit modes can diverge while sharing rendering helpers.
func TaskEditViewActions() *ActionRegistry {
	r := NewActionRegistry()

	r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
	r.Register(Action{ID: ActionNextField, Key: tcell.KeyTab, Label: "Next", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionPrevField, Key: tcell.KeyBacktab, Label: "Prev", ShowInHeader: true, HideFromPalette: true})

	return r
}

// CommonFieldNavigationActions returns actions available in all field editors (Tab/Shift-Tab navigation)
func CommonFieldNavigationActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionNextField, Key: tcell.KeyTab, Label: "Next field", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionPrevField, Key: tcell.KeyBacktab, Label: "Prev field", ShowInHeader: true, HideFromPalette: true})
	return r
}

// TaskEditTitleActions returns actions available when editing the title field
func TaskEditTitleActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionQuickSave, Key: tcell.KeyEnter, Label: "Quick Save", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
	r.Merge(CommonFieldNavigationActions())
	return r
}

// TaskEditStatusActions returns actions available when editing the status field
func TaskEditStatusActions() *ActionRegistry {
	r := CommonFieldNavigationActions()
	r.Register(Action{ID: ActionNextValue, Key: tcell.KeyDown, Label: "Next ↓", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionPrevValue, Key: tcell.KeyUp, Label: "Prev ↑", ShowInHeader: true, HideFromPalette: true})
	return r
}

// TaskEditTypeActions returns actions available when editing the type field
func TaskEditTypeActions() *ActionRegistry {
	r := CommonFieldNavigationActions()
	r.Register(Action{ID: ActionNextValue, Key: tcell.KeyDown, Label: "Next ↓", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionPrevValue, Key: tcell.KeyUp, Label: "Prev ↑", ShowInHeader: true, HideFromPalette: true})
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
	r.Register(Action{ID: ActionNextValue, Key: tcell.KeyDown, Label: "Next ↓", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionPrevValue, Key: tcell.KeyUp, Label: "Prev ↑", ShowInHeader: true, HideFromPalette: true})
	return r
}

// TaskEditPointsActions returns actions available when editing the story points field
func TaskEditPointsActions() *ActionRegistry {
	r := CommonFieldNavigationActions()
	// Future: Add ActionChangePoints when points editor is implemented
	return r
}

// TaskEditDueActions returns actions available when editing the due date field
func TaskEditDueActions() *ActionRegistry {
	r := CommonFieldNavigationActions()
	r.Register(Action{ID: ActionNextValue, Key: tcell.KeyDown, Label: "Next ↓", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionPrevValue, Key: tcell.KeyUp, Label: "Prev ↑", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionClearField, Key: tcell.KeyCtrlU, Label: "Clear", ShowInHeader: true, HideFromPalette: true})
	return r
}

// TaskEditRecurrenceActions returns actions available when editing the recurrence field
func TaskEditRecurrenceActions() *ActionRegistry {
	r := CommonFieldNavigationActions()
	r.Register(Action{ID: ActionNextValue, Key: tcell.KeyDown, Label: "Next ↓", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionPrevValue, Key: tcell.KeyUp, Label: "Prev ↑", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionNavLeft, Key: tcell.KeyLeft, Label: "← Part", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionNavRight, Key: tcell.KeyRight, Label: "Part →", ShowInHeader: true, HideFromPalette: true})
	return r
}

// TaskEditDescriptionActions returns actions available when editing the description field
func TaskEditDescriptionActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
	r.Merge(CommonFieldNavigationActions())
	return r
}

// DescOnlyEditActions returns actions for description-only edit mode (no field navigation).
func DescOnlyEditActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
	return r
}

// TagsOnlyEditActions returns actions for tags-only edit mode (no field navigation).
func TagsOnlyEditActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
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
	case model.EditFieldDue:
		return TaskEditDueActions()
	case model.EditFieldRecurrence:
		return TaskEditRecurrenceActions()
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

	// navigation (not shown in header, hidden from palette)
	r.Register(Action{ID: ActionNavUp, Key: tcell.KeyUp, Label: "↑", HideFromPalette: true})
	r.Register(Action{ID: ActionNavDown, Key: tcell.KeyDown, Label: "↓", HideFromPalette: true})
	r.Register(Action{ID: ActionNavLeft, Key: tcell.KeyLeft, Label: "←", HideFromPalette: true})
	r.Register(Action{ID: ActionNavRight, Key: tcell.KeyRight, Label: "→", HideFromPalette: true})
	r.Register(Action{ID: ActionNavUp, Key: tcell.KeyRune, Rune: 'k', Label: "↑", HideFromPalette: true})
	r.Register(Action{ID: ActionNavDown, Key: tcell.KeyRune, Rune: 'j', Label: "↓", HideFromPalette: true})
	r.Register(Action{ID: ActionNavLeft, Key: tcell.KeyRune, Rune: 'h', Label: "←", HideFromPalette: true})
	r.Register(Action{ID: ActionNavRight, Key: tcell.KeyRune, Rune: 'l', Label: "→", HideFromPalette: true})

	// plugin actions (shown in header)
	r.Register(Action{ID: ActionOpenFromPlugin, Key: tcell.KeyEnter, Label: "Open", ShowInHeader: true, IsEnabled: selectionRequired})
	r.Register(Action{ID: ActionMoveTaskLeft, Key: tcell.KeyLeft, Modifier: tcell.ModShift, Label: "Move ←", ShowInHeader: true, IsEnabled: selectionRequired})
	r.Register(Action{ID: ActionMoveTaskRight, Key: tcell.KeyRight, Modifier: tcell.ModShift, Label: "Move →", ShowInHeader: true, IsEnabled: selectionRequired})
	r.Register(Action{ID: ActionNewTask, Key: tcell.KeyRune, Rune: 'n', Label: "New", ShowInHeader: true})
	r.Register(Action{ID: ActionDeleteTask, Key: tcell.KeyRune, Rune: 'd', Label: "Delete", ShowInHeader: true, IsEnabled: selectionRequired})
	r.Register(Action{ID: ActionSearch, Key: tcell.KeyRune, Rune: '/', Label: "Search", ShowInHeader: true})
	r.Register(Action{ID: ActionToggleViewMode, Key: tcell.KeyRune, Rune: 'v', Label: "View mode", ShowInHeader: true})

	// plugin activation keys are merged dynamically after plugins load
	r.MergePluginActions()

	return r
}

// DepsViewActions returns the action registry for the dependency editor view.
func DepsViewActions() *ActionRegistry {
	r := NewActionRegistry()

	// navigation (not shown in header, hidden from palette)
	r.Register(Action{ID: ActionNavUp, Key: tcell.KeyUp, Label: "↑", HideFromPalette: true})
	r.Register(Action{ID: ActionNavDown, Key: tcell.KeyDown, Label: "↓", HideFromPalette: true})
	r.Register(Action{ID: ActionNavLeft, Key: tcell.KeyLeft, Label: "←", HideFromPalette: true})
	r.Register(Action{ID: ActionNavRight, Key: tcell.KeyRight, Label: "→", HideFromPalette: true})
	r.Register(Action{ID: ActionNavUp, Key: tcell.KeyRune, Rune: 'k', Label: "↑", HideFromPalette: true})
	r.Register(Action{ID: ActionNavDown, Key: tcell.KeyRune, Rune: 'j', Label: "↓", HideFromPalette: true})
	r.Register(Action{ID: ActionNavLeft, Key: tcell.KeyRune, Rune: 'h', Label: "←", HideFromPalette: true})
	r.Register(Action{ID: ActionNavRight, Key: tcell.KeyRune, Rune: 'l', Label: "→", HideFromPalette: true})

	// move task between lanes (shown in header)
	r.Register(Action{ID: ActionMoveTaskLeft, Key: tcell.KeyLeft, Modifier: tcell.ModShift, Label: "Move ←", ShowInHeader: true, IsEnabled: selectionRequired})
	r.Register(Action{ID: ActionMoveTaskRight, Key: tcell.KeyRight, Modifier: tcell.ModShift, Label: "Move →", ShowInHeader: true, IsEnabled: selectionRequired})

	// task actions
	r.Register(Action{ID: ActionOpenFromPlugin, Key: tcell.KeyEnter, Label: "Open", ShowInHeader: true, IsEnabled: selectionRequired})
	r.Register(Action{ID: ActionNewTask, Key: tcell.KeyRune, Rune: 'n', Label: "New", ShowInHeader: true})
	r.Register(Action{ID: ActionDeleteTask, Key: tcell.KeyRune, Rune: 'd', Label: "Delete", ShowInHeader: true, IsEnabled: selectionRequired})

	// view mode and search
	r.Register(Action{ID: ActionSearch, Key: tcell.KeyRune, Rune: '/', Label: "Search", ShowInHeader: true})
	r.Register(Action{ID: ActionToggleViewMode, Key: tcell.KeyRune, Rune: 'v', Label: "View mode", ShowInHeader: true})

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
