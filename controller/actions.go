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
	ActionBack         ActionID = "back"
	ActionQuit         ActionID = "quit"
	ActionRefresh      ActionID = "refresh"
	ActionToggleHeader ActionID = "toggle_header"
	ActionOpenPalette  ActionID = "open_palette"
	ActionEditWorkflow ActionID = "edit_workflow"
)

// ActionID values for tiki navigation and manipulation (used by plugins).
const (
	ActionMoveTikiLeft  ActionID = "move_tiki_left"
	ActionMoveTikiRight ActionID = "move_tiki_right"
	ActionNewTiki       ActionID = "new_tiki"
	ActionNavLeft       ActionID = "nav_left"
	ActionNavRight      ActionID = "nav_right"
	ActionNavUp         ActionID = "nav_up"
	ActionNavDown       ActionID = "nav_down"
)

// ActionID values for tiki detail view actions.
const (
	ActionEditTitle  ActionID = "edit_title"
	ActionEditSource ActionID = "edit_source"
	ActionFullscreen ActionID = "fullscreen"
	ActionCloneTiki  ActionID = "clone_tiki"
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

	// ActionDetailSaveAndClose: commits edits in in-place detail edit mode and
	// pops the detail view, returning to the originating board. Bound to Enter
	// on single-line fields (multi-line TextArea fields keep Enter as newline).
	ActionDetailSaveAndClose ActionID = "detail_save_and_close"
)

// ActionID values for tiki edit view actions.
const (
	ActionSaveTiki   ActionID = "save_tiki"
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

// ActionID values for wiki plugin (markdown navigation) actions.
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
		notSelf := Requirement("!view:" + string(pluginViewID))
		require := []Requirement{notSelf}
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
			HideRequire:  []Requirement{notSelf},
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
// carriedSelection is the number of tiki ids the caller intends to pass
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
	RequireDetailPlugin  Requirement = "detail-plugin"
	RequireSingleLane    Requirement = "single-lane"
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

// detailViewIDPredicate decides whether a ViewID represents a configurable
// detail view, set at bootstrap so this package doesn't have to hardcode
// the plugin name. Defaults to false (no detail view registered) which
// keeps tests that don't bootstrap plugins working unchanged.
var detailViewIDPredicate = func(model.ViewID) bool { return false }

// SetDetailViewIDPredicate installs the predicate used by BuildAppContext
// to detect configurable detail views — a plugin-name lookup that isn't
// available in the controller package directly. Bootstrap wires this once
// per session.
func SetDetailViewIDPredicate(fn func(model.ViewID) bool) {
	if fn == nil {
		detailViewIDPredicate = func(model.ViewID) bool { return false }
		return
	}
	detailViewIDPredicate = fn
}

// IsDetailView reports whether the given ViewID is a configurable detail
// view. Used as a successor to the legacy `viewID == TikiDetailViewID`
// comparison after the legacy view's deletion.
func IsDetailView(viewID model.ViewID) bool {
	return detailViewIDPredicate(viewID)
}

// detailPluginPredicate decides whether the active workflow declares a
// kind: detail plugin. Set at bootstrap so this package doesn't have to
// reach into plugin defs directly. Defaults to false (no detail plugin)
// which keeps tests that don't bootstrap plugins working unchanged.
var detailPluginPredicate = func() bool { return false }

// SetDetailPluginPredicate installs the predicate used by BuildAppContext
// to detect whether the active workflow has a Detail plugin. Bootstrap
// wires this once per session. Passing nil resets to the default.
func SetDetailPluginPredicate(fn func() bool) {
	if fn == nil {
		detailPluginPredicate = func() bool { return false }
		return
	}
	detailPluginPredicate = fn
}

// singleLanePredicate decides whether the plugin view identified by id is
// a board/list view with one lane or fewer. Set at bootstrap so this
// package doesn't have to reach into plugin defs directly. Defaults to
// false (no view is single-lane) which keeps tests unchanged.
var singleLanePredicate = func(model.ViewID) bool { return false }

// SetSingleLanePredicate installs the predicate used by BuildAppContext
// to gate move-tiki-left/right actions. Bootstrap wires this once per
// session. Passing nil resets to the default.
func SetSingleLanePredicate(fn func(model.ViewID) bool) {
	if fn == nil {
		singleLanePredicate = func(model.ViewID) bool { return false }
		return
	}
	singleLanePredicate = fn
}

// BuildAppContext constructs an AppContext from the current UI state.
func BuildAppContext(currentView *ViewEntry, activeView View) AppContext {
	ctx := NewAppContext()

	selectedCount := 0
	// A live SelectableView is authoritative about its own selection: when the
	// active view can answer, its answer wins and the params fallback below is
	// skipped entirely. Otherwise a stale TikiID left in the nav params would
	// contaminate the context — making an empty/filtered board advertise
	// selection-gated actions as enabled (H/H2).
	_, activeViewIsSelectable := activeView.(SelectableView)
	if sv, ok := activeView.(SelectableView); ok && sv.GetSelectedID() != "" {
		selectedCount = 1
	}

	if !activeViewIsSelectable && selectedCount == 0 && currentView != nil && IsDetailView(currentView.ViewID) {
		if model.DecodePluginViewParams(currentView.Params).TikiID != "" {
			selectedCount = 1
		}
	}

	// Phase 6B.3/6B.7: plugin views (wiki, detail, board, list) may carry a
	// selected tiki id via PluginViewParams. Consulted only when the active
	// view is not a live SelectableView — e.g. the view does not implement
	// SelectableView yet, or its selection is set after this gate runs — so
	// actions gated on selection:one still dispatch from views that received a
	// selection via nav passthrough without overriding a live "nothing
	// selected" answer.
	if !activeViewIsSelectable && selectedCount == 0 && currentView != nil && model.IsPluginViewID(currentView.ViewID) {
		if model.DecodePluginViewParams(currentView.Params).TikiID != "" {
			selectedCount = 1
		}
	}

	applySelectionCardinality(ctx, selectedCount)

	if config.GetAIAgent() != "" {
		ctx.Set(string(RequireAI))
	}

	if detailPluginPredicate() {
		ctx.Set(string(RequireDetailPlugin))
	}

	if currentView != nil {
		ctx.Set("view:" + string(currentView.ViewID))
		if singleLanePredicate(currentView.ViewID) {
			ctx.Set(string(RequireSingleLane))
		}
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
	// HideRequire is an additional requirement set whose unmet state hides
	// the action from the header entirely (rather than rendering it greyed).
	// Use for structural conditions that cannot change within the current
	// view — e.g. single-lane boards have no left/right neighbor to move to.
	// HideRequire does NOT affect keypress dispatch; only header rendering.
	HideRequire []Requirement
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

// normalizeBinding collapses equivalent key encodings to a canonical form.
// Ctrl+letter → KeyCtrlX + ModCtrl (where X is the ctrl key constant 1-26),
// normalizing KeyCtrlX+0, KeyCtrlX+ModCtrl, and Key='X'+ModCtrl alike. The
// backspace/delete family (KeyBackspace, KeyBackspace2, KeyDelete) → KeyDelete,
// so a Delete- or Backspace-bound action fires no matter which the platform
// reports for the physical delete key.
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

	// collapse the backspace/delete family to one canonical key. A workflow
	// `key: "Delete"` registers under KeyDelete, but the Mac main delete key
	// sends KeyBackspace2 (and some terminals send KeyBackspace); collapsing
	// them here lets a Delete- or Backspace-bound action fire regardless of
	// which the platform reports. Text-editing backspace is unaffected —
	// it is diverted to the focused widget upstream of the action registry
	// (input_router.go isTextInputKey), so it never reaches this lookup.
	if key == tcell.KeyBackspace || key == tcell.KeyBackspace2 {
		return tcell.KeyDelete, 0, mod
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
	r.Register(Action{ID: ActionOpenPalette, Key: tcell.KeyCtrlA, Modifier: tcell.ModCtrl, Label: "All actions", ShowInHeader: true, HideFromPalette: true})
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
	result := make([]model.HeaderAction, 0, len(actions))
	for _, a := range actions {
		if !ActionEnabled(Action{Require: a.HideRequire}, ctx) {
			continue
		}
		result = append(result, model.HeaderAction{
			ID:           string(a.ID),
			Key:          a.Key,
			Rune:         a.Rune,
			Label:        a.Label,
			Modifier:     a.Modifier,
			ShowInHeader: a.ShowInHeader,
			Enabled:      ActionEnabled(a, ctx),
		})
	}
	return result
}

// TikiDetailViewActions returns the canonical action registry for the tiki detail view.
// Single source of truth for both input handling and header display.
func TikiDetailViewActions() *ActionRegistry {
	r := NewActionRegistry()

	idReq := []Requirement{RequireID}
	r.Register(Action{ID: ActionEditSource, Key: tcell.KeyRune, Rune: 's', Label: "Edit source", ShowInHeader: true, Require: idReq})
	r.Register(Action{ID: ActionFullscreen, Key: tcell.KeyRune, Rune: 'f', Label: "Full screen", ShowInHeader: true})
	r.Register(Action{ID: ActionChat, Key: tcell.KeyRune, Rune: 'c', Label: "Chat", ShowInHeader: true, Require: []Requirement{RequireAI, RequireID}})

	return r
}

// TikiEditViewActions returns the canonical action registry for the tiki edit view.
// Separate registry so view/edit modes can diverge while sharing rendering helpers.
func TikiEditViewActions() *ActionRegistry {
	r := NewActionRegistry()

	r.Register(Action{ID: ActionSaveTiki, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
	r.Register(Action{ID: ActionNextField, Key: tcell.KeyTab, Label: "Next", ShowInHeader: true, HideFromPalette: true})
	r.Register(Action{ID: ActionPrevField, Key: tcell.KeyBacktab, Label: "Prev", ShowInHeader: true, HideFromPalette: true})

	return r
}

// DescOnlyEditActions returns actions for description-only edit mode (no field navigation).
func DescOnlyEditActions() *ActionRegistry {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionSaveTiki, Key: tcell.KeyCtrlS, Label: "Save", ShowInHeader: true})
	return r
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
	// Enter is intentionally not bound here. Phase 1 retired the built-in
	// "open tiki in detail" shortcut: the open keybinding is now declared by
	// the workflow as a `kind: view` action (typically `key: Enter, view: Detail`).
	// Boards without such an action have no Enter behavior — by design. Delete
	// was likewise migrated: it is a global workflow action (`delete where
	// id = id()`), no longer a hardcoded `d` binding here.
	notSingleLane := "!" + RequireSingleLane
	moveReq := []Requirement{RequireID, notSingleLane}
	hideOnSingleLane := []Requirement{notSingleLane}
	r.Register(Action{ID: ActionMoveTikiLeft, Key: tcell.KeyLeft, Modifier: tcell.ModShift, Label: "Move ←", ShowInHeader: true, Require: moveReq, HideRequire: hideOnSingleLane})
	r.Register(Action{ID: ActionMoveTikiRight, Key: tcell.KeyRight, Modifier: tcell.ModShift, Label: "Move →", ShowInHeader: true, Require: moveReq, HideRequire: hideOnSingleLane})
	r.Register(Action{ID: ActionSearch, Key: tcell.KeyRune, Rune: '/', Label: "Search", ShowInHeader: true})
	r.Register(Action{ID: ActionExecute, Key: tcell.KeyRune, Rune: '!', Label: "Execute", ShowInHeader: true})

	// plugin activation keys are merged dynamically after plugins load
	r.MergePluginActions()

	return r
}

// WikiViewActions returns the action registry for wiki plugin views.
// Wiki views primarily handle navigation through the NavigableMarkdown component.
func WikiViewActions() *ActionRegistry {
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
