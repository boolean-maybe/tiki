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
	ActionEditWorkflow   ActionID = "edit_workflow"
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

	// ActionDetailEditStub: registered on configurable detail views so the
	// Edit keybinding stays reserved during Phase 1. Phase 2 replaces the
	// no-op handler with the in-place edit-mode toggle. The constant is
	// kept for tests and palette callers that referenced it; new code
	// should prefer ActionDetailEdit.
	ActionDetailEditStub ActionID = "detail_edit_stub"

	// ActionDetailEdit: enters in-place edit mode on a configurable detail
	// view. Phase 2 wiring.
	ActionDetailEdit ActionID = "detail_edit"

	// ActionDetailSave: commits edits in in-place detail edit mode.
	ActionDetailSave ActionID = "detail_save"

	// ActionDetailCancel: cancels edits in in-place detail edit mode.
	ActionDetailCancel ActionID = "detail_cancel"
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
	ActionExecute        ActionID = "execute"
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
	// Require carries the view's own `require:` list from workflow.yaml
	// (BasePlugin.Require). 6B.15: the direct activation-key action for
	// this view must respect the view's declared requirements so a
	// detail view with `require: ["selection:one"]` cannot be opened
	// with no selection, regardless of how the navigation was triggered.
	Require []string
}

// pluginActionRegistry holds plugin navigation actions (populated at init time)
var pluginActionRegistry *ActionRegistry

// pluginViewRequires maps plugin view name → the view's own require: list
// from workflow.yaml. Populated alongside the activation-key registry so
// `kind: view` action dispatchers can consult the target view's
// requirements without depending on the plugin package.
var pluginViewRequires map[string][]string

// InitPluginActions creates the plugin action registry from loaded plugins.
// Called once during app initialization after plugins are loaded.
//
// The activation action's Require list is the union of:
//   - `!view:<id>` — prevents re-opening the view we're already on.
//   - the view's own `require:` from workflow.yaml, EXCLUDING `view:*`
//     tokens. View-family requirements describe *target*-scoped state
//     ("require that the current view be X") and must not be evaluated
//     against the source view's id by the source-scoped UI enablement
//     gate. They are instead honored by the target-scoped
//     TargetViewEnabled check that runs inside activateTargetView at
//     dispatch time, matching what `kind: view` action dispatch does
//     (6B.22 / 6B.24).
//
// Source-safe requirements (selection cardinality, `ai`, custom tokens)
// remain in the merged list so the UI correctly greys out the activation
// key when the source view can't satisfy them — no round-trip through
// the dispatcher needed for those.
func InitPluginActions(plugins []PluginInfo) {
	pluginActionRegistry = NewActionRegistry()
	pluginViewRequires = make(map[string][]string, len(plugins))
	for _, p := range plugins {
		if len(p.Require) > 0 {
			pluginViewRequires[p.Name] = p.Require
		}
		if p.Key == 0 && p.Rune == 0 {
			continue // skip plugins without key binding
		}
		pluginViewID := model.MakePluginViewID(p.Name)
		require := []Requirement{Requirement("!view:" + string(pluginViewID))}
		for _, r := range p.Require {
			if isViewScopedRequirement(r) {
				continue
			}
			require = append(require, Requirement(r))
		}
		pluginActionRegistry.Register(Action{
			ID:           ActionID("plugin:" + p.Name),
			Key:          p.Key,
			Rune:         p.Rune,
			Modifier:     p.Modifier,
			Label:        p.Name,
			ShowInHeader: true,
			Require:      require,
		})
	}
}

// isViewScopedRequirement reports whether a require token names the
// `view:*` family (positive or negated). Those tokens are target-scoped
// when attached to a view's require list and must not be evaluated at the
// source-scoped UI enablement gate — see InitPluginActions comment.
func isViewScopedRequirement(token string) bool {
	t := token
	if len(t) > 0 && t[0] == '!' {
		t = t[1:]
	}
	return len(t) >= len("view:") && t[:len("view:")] == "view:"
}

// PluginViewRequire returns the view-level require: list for the named view.
// Returns nil when the view is unknown or declares no requirements.
// Consumed by kind: view dispatchers so navigation to a target view is gated
// on the target's own declared requirements, not just the action's.
func PluginViewRequire(viewName string) []string {
	if pluginViewRequires == nil {
		return nil
	}
	return pluginViewRequires[viewName]
}

// TargetViewEnabled reports whether navigation to the named view satisfies
// the target's own `require:` list. Unlike selectionSatisfies (which only
// materializes selection-cardinality attributes), this evaluates the full
// attribute set — `view:*`, `ai`, custom tokens — against the AppContext
// the target view will inhabit post-navigation.
//
// carriedSelection is the number of task ids the caller intends to pass
// into the target's params (0 or 1 today).
//
// 6B.20: initial fix. Required for correctness on kind: view dispatch,
// which previously went through selectionSatisfies and silently dropped
// non-selection requirements. Direct-key activation already honors the
// full list because InitPluginActions merges the view's require into the
// activation action and the InputRouter enablement gate runs ActionEnabled
// on it.
//
// 6B.22: the context is built from scratch for the target view rather
// than inherited from the source. Previously the source's `view:<id>`
// attribute leaked into evaluation, so a target declaring
// `!view:plugin:<self>` or `view:plugin:<other>` got the wrong answer.
// Attributes preserved from global state: `ai` (agent-configured); the
// target supplies `view:<target-id>` and the carried selection.
func TargetViewEnabled(targetViewName string, carriedSelection int) bool {
	target := PluginViewRequire(targetViewName)
	if len(target) == 0 {
		return true
	}
	ctx := NewAppContext()
	applySelectionCardinality(ctx, carriedSelection)
	ctx.Set("view:" + string(model.MakePluginViewID(targetViewName)))
	if config.GetAIAgent() != "" {
		ctx.Set(string(RequireAI))
	}

	reqs := make([]Requirement, 0, len(target))
	for _, r := range target {
		reqs = append(reqs, Requirement(r))
	}
	return ActionEnabled(Action{Require: reqs}, ctx)
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

// Requirement is a declarative context attribute that an action needs to be enabled.
// Positive values (e.g. "id", "ai") require the attribute to be present.
// Negated values (e.g. "!view:plugin:Kanban") require the attribute to be absent.
type Requirement string

const (
	RequireID            Requirement = "id"
	RequireAI            Requirement = "ai"
	RequireSelectionOne  Requirement = "selection:one"
	RequireSelectionAny  Requirement = "selection:any"
	RequireSelectionMany Requirement = "selection:many"
)

// AppContext is a dynamic set of active context attributes built from live UI state.
// Actions declare requirements against this set to determine enabled/disabled state.
type AppContext struct {
	attrs map[string]struct{}
}

// NewAppContext creates an empty AppContext.
func NewAppContext() AppContext {
	return AppContext{attrs: make(map[string]struct{})}
}

// Has returns true if the attribute is present.
func (c AppContext) Has(attr string) bool {
	_, ok := c.attrs[attr]
	return ok
}

// Set adds an attribute to the context.
func (c AppContext) Set(attr string) {
	c.attrs[attr] = struct{}{}
}

// Delete removes an attribute from the context.
func (c AppContext) Delete(attr string) {
	delete(c.attrs, attr)
}

// Clone returns a shallow copy of the context.
func (c AppContext) Clone() AppContext {
	cp := NewAppContext()
	for k := range c.attrs {
		cp.attrs[k] = struct{}{}
	}
	return cp
}

// ActionEnabled returns true if the action's requirements are met by the context.
// Empty requirements means always enabled.
// Positive requirements must be present; negated requirements (prefixed with "!") must be absent.
func ActionEnabled(a Action, ctx AppContext) bool {
	for _, r := range a.Require {
		if len(r) == 0 {
			continue
		}
		if r[0] == '!' {
			attr := string(r[1:])
			if ctx.Has(attr) {
				return false
			}
		} else {
			if !ctx.Has(string(r)) {
				return false
			}
		}
	}
	return true
}

// BuildAppContext constructs an AppContext from the current UI state.
func BuildAppContext(currentView *ViewEntry, activeView View) AppContext {
	ctx := NewAppContext()

	selectedCount := 0
	if activeView != nil {
		if sv, ok := activeView.(SelectableView); ok && sv.GetSelectedID() != "" {
			selectedCount = 1
		}
	}

	if selectedCount == 0 && currentView != nil && currentView.ViewID == model.TaskDetailViewID {
		if model.DecodeTaskDetailParams(currentView.Params).TaskID != "" {
			selectedCount = 1
		}
	}

	// Phase 6B.3/6B.7: plugin views (wiki, detail, board, list) may carry a
	// selected task id via PluginViewParams. If the active view did not
	// report a selection — e.g. the view does not implement SelectableView
	// yet, or its selection is set after this gate runs — consult the nav
	// params as a second source so actions gated on selection:one still
	// dispatch from views that received a selection via nav passthrough.
	if selectedCount == 0 && currentView != nil && model.IsPluginViewID(currentView.ViewID) {
		if model.DecodePluginViewParams(currentView.Params).TaskID != "" {
			selectedCount = 1
		}
	}

	applySelectionCardinality(ctx, selectedCount)

	if config.GetAIAgent() != "" {
		ctx.Set(string(RequireAI))
	}

	if currentView != nil {
		ctx.Set("view:" + string(currentView.ViewID))
	}

	return ctx
}

// applySelectionCardinality writes the current selection count into the
// context as the three cardinality attributes. "id" is kept as the legacy
// alias for "selection:one" so existing plugin actions continue to work.
func applySelectionCardinality(ctx AppContext, count int) {
	if count >= 1 {
		ctx.Set(string(RequireSelectionAny))
	}
	if count == 1 {
		ctx.Set(string(RequireID))
		ctx.Set(string(RequireSelectionOne))
	}
	if count >= 2 {
		ctx.Set(string(RequireSelectionMany))
	}
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
	Require         []Requirement
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

// Register adds an action to the registry.
// The binding is normalized before storage so lookups are always consistent.
func (r *ActionRegistry) Register(action Action) {
	action.Key, action.Rune, action.Modifier = normalizeBinding(action.Key, action.Rune, action.Modifier)
	r.actions = append(r.actions, action)
	if action.Key == 0 && action.Rune == 0 {
		return // palette-only action — no keybinding to index
	}
	if action.Key == tcell.KeyRune {
		r.byRune[runeWithMod{action.Rune, action.Modifier}] = action
	} else {
		r.byKey[keyWithMod{action.Key, action.Modifier}] = action
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

// normalizeBinding collapses equivalent Ctrl+letter encodings to a canonical form.
// Canonical form: KeyCtrlX + ModCtrl (where X is the ctrl key constant 1-26).
// This normalizes: KeyCtrlX+0, KeyCtrlX+ModCtrl, and Key='X'+ModCtrl to the same form.
func normalizeBinding(key tcell.Key, ch rune, mod tcell.ModMask) (tcell.Key, rune, tcell.ModMask) {
	mod = mod & (tcell.ModShift | tcell.ModCtrl | tcell.ModAlt | tcell.ModMeta)

	// KeyCtrlA(65)..KeyCtrlZ(90) — ensure ModCtrl is always set
	if key >= tcell.KeyCtrlA && key <= tcell.KeyCtrlZ {
		return key, 0, mod | tcell.ModCtrl
	}

	// Key='a'..'z' with ModCtrl → normalize to KeyCtrlX + ModCtrl
	if mod&tcell.ModCtrl != 0 && key != tcell.KeyRune {
		if key >= 'a' && key <= 'z' {
			return key - 'a' + tcell.KeyCtrlA, 0, mod
		}
	}

	return key, ch, mod
}

// matchBinding is the shared core lookup logic used by both Match() and MatchBinding().
func (r *ActionRegistry) matchBinding(key tcell.Key, ch rune, mod tcell.ModMask) *Action {
	key, ch, mod = normalizeBinding(key, ch, mod)

	if key == tcell.KeyRune {
		if a, ok := r.byRune[runeWithMod{ch, mod}]; ok {
			return &a
		}
		// rune actions registered without a modifier match any modifier
		if mod != 0 {
			if a, ok := r.byRune[runeWithMod{ch, 0}]; ok && a.Modifier == 0 {
				return &a
			}
		}
		return nil
	}

	if a, ok := r.byKey[keyWithMod{key, mod}]; ok {
		return &a
	}

	return nil
}

// Match finds an action matching the given key event using O(1) map lookups.
func (r *ActionRegistry) Match(event *tcell.EventKey) *Action {
	return r.matchBinding(event.Key(), event.Rune(), event.Modifiers())
}

// MatchBinding finds an action matching the given key/rune/modifier triple.
// Used for conflict detection during plugin action registration.
func (r *ActionRegistry) MatchBinding(key tcell.Key, ch rune, mod tcell.ModMask) *Action {
	return r.matchBinding(key, ch, mod)
}

// DefaultGlobalActions returns common actions available in all views
func DefaultGlobalActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionBack, Key: tcell.KeyEscape, Label: "Back", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit", ShowInHeader: true})
	r.Register(Action{ID: ActionRefresh, Key: tcell.KeyRune, Rune: 'r', Label: "Refresh", ShowInHeader: true})
	r.Register(Action{ID: ActionToggleHeader, Key: tcell.KeyF10, Label: "Toggle Header", ShowInHeader: true})
	r.Register(Action{ID: ActionOpenPalette, Key: tcell.KeyCtrlA, Modifier: tcell.ModCtrl, Label: "All", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionEditWorkflow, Label: "Edit Workflow"})
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

// GetByID returns the action with the given ID, or nil if not found.
func (r *ActionRegistry) GetByID(id ActionID) *Action {
	if r == nil {
		return nil
	}
	for i := range r.actions {
		if r.actions[i].ID == id {
			return &r.actions[i]
		}
	}
	return nil
}

// ToHeaderActions converts the registry's header actions to model.HeaderAction slice.
// This bridges the controller→model boundary without requiring callers to do the mapping.
func (r *ActionRegistry) ToHeaderActions() []model.HeaderAction {
	return r.ToHeaderActionsForContext(NewAppContext())
}

// ToHeaderActionsForContext converts header actions with live enabled/disabled state
// evaluated against the given AppContext.
func (r *ActionRegistry) ToHeaderActionsForContext(ctx AppContext) []model.HeaderAction {
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
			Enabled:      ActionEnabled(a, ctx),
		}
	}
	return result
}

// TaskDetailViewActions returns the canonical action registry for the task detail view.
// Single source of truth for both input handling and header display.
func TaskDetailViewActions() *ActionRegistry {
	r := NewActionRegistry()

	idReq := []Requirement{RequireID}
	r.Register(Action{ID: ActionEditTitle, Key: tcell.KeyRune, Rune: 'e', Label: "Edit", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionEditDesc, Key: tcell.KeyRune, Rune: 'D', Label: "Edit desc", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionEditSource, Key: tcell.KeyRune, Rune: 's', Label: "Edit source", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionFullscreen, Key: tcell.KeyRune, Rune: 'f', Label: "Full screen", ShowInHeader: true})
	r.Register(Action{ID: ActionEditDeps, Key: tcell.KeyCtrlD, Modifier: tcell.ModCtrl, Label: "Dependencies", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionEditTags, Key: tcell.KeyRune, Rune: 'T', Label: "Edit tags", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionChat, Key: tcell.KeyRune, Rune: 'c', Label: "Chat", ShowInHeader: true, Require: []Requirement{RequireAI, RequireID}})

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
	idReq := []Requirement{RequireID}
	// Enter is intentionally not bound here. Phase 1 retired the built-in
	// "open task in detail" shortcut: the open keybinding is now declared by
	// the workflow as a `kind: view` action (typically `key: Enter, view: Detail`).
	// Boards without such an action have no Enter behavior — by design.
	r.Register(Action{ID: ActionMoveTaskLeft, Key: tcell.KeyLeft, Modifier: tcell.ModShift, Label: "Move ←", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionMoveTaskRight, Key: tcell.KeyRight, Modifier: tcell.ModShift, Label: "Move →", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionNewTask, Key: tcell.KeyRune, Rune: 'n', Label: "New", ShowInHeader: true})
	r.Register(Action{ID: ActionDeleteTask, Key: tcell.KeyRune, Rune: 'd', Label: "Delete", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionSearch, Key: tcell.KeyRune, Rune: '/', Label: "Search", ShowInHeader: true})
	r.Register(Action{ID: ActionToggleViewMode, Key: tcell.KeyRune, Rune: 'v', Label: "View mode", ShowInHeader: true})
	r.Register(Action{ID: ActionExecute, Key: tcell.KeyRune, Rune: '!', Label: "Execute", ShowInHeader: true})

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
	depsIdReq := []Requirement{RequireID}
	r.Register(Action{ID: ActionMoveTaskLeft, Key: tcell.KeyLeft, Modifier: tcell.ModShift, Label: "Move ←", ShowInHeader: true, Require: depsIdReq})
	r.Register(Action{ID: ActionMoveTaskRight, Key: tcell.KeyRight, Modifier: tcell.ModShift, Label: "Move →", ShowInHeader: true, Require: depsIdReq})

	// task actions
	r.Register(Action{ID: ActionOpenFromPlugin, Key: tcell.KeyEnter, Label: "Open", ShowInHeader: true, Require: depsIdReq})
	r.Register(Action{ID: ActionNewTask, Key: tcell.KeyRune, Rune: 'n', Label: "New", ShowInHeader: true})
	r.Register(Action{ID: ActionDeleteTask, Key: tcell.KeyRune, Rune: 'd', Label: "Delete", ShowInHeader: true, Require: depsIdReq})

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
