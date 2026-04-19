package controller

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"

	"github.com/gdamore/tcell/v2"
	"github.com/spf13/viper"
)

func TestActionRegistry_Merge(t *testing.T) {
	tests := []struct {
		name           string
		registry1      func() *ActionRegistry
		registry2      func() *ActionRegistry
		wantActionIDs  []ActionID
		wantKeyLookup  map[keyWithMod]ActionID
		wantRuneLookup map[runeWithMod]ActionID
	}{
		{
			name: "merge two non-overlapping registries",
			registry1: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit"})
				return r
			},
			registry2: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionRefresh, Key: tcell.KeyRune, Rune: 'r', Label: "Refresh"})
				r.Register(Action{ID: ActionBack, Key: tcell.KeyEscape, Label: "Back"})
				return r
			},
			wantActionIDs: []ActionID{ActionQuit, ActionRefresh, ActionBack},
			wantKeyLookup: map[keyWithMod]ActionID{
				{tcell.KeyEscape, 0}: ActionBack,
			},
			wantRuneLookup: map[runeWithMod]ActionID{
				{'q', 0}: ActionQuit,
				{'r', 0}: ActionRefresh,
			},
		},
		{
			name: "merge with overlapping key - second registry wins",
			registry1: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit"})
				return r
			},
			registry2: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionSearch, Key: tcell.KeyRune, Rune: 'q', Label: "Quick Search"})
				return r
			},
			wantActionIDs: []ActionID{ActionQuit, ActionSearch},
			wantRuneLookup: map[runeWithMod]ActionID{
				{'q', 0}: ActionSearch, // overwritten by second registry
			},
		},
		{
			name: "merge empty registry",
			registry1: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit"})
				return r
			},
			registry2: func() *ActionRegistry {
				return NewActionRegistry()
			},
			wantActionIDs: []ActionID{ActionQuit},
			wantRuneLookup: map[runeWithMod]ActionID{
				{'q', 0}: ActionQuit,
			},
		},
		{
			name: "merge into empty registry",
			registry1: func() *ActionRegistry {
				return NewActionRegistry()
			},
			registry2: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionRefresh, Key: tcell.KeyRune, Rune: 'r', Label: "Refresh"})
				return r
			},
			wantActionIDs: []ActionID{ActionRefresh},
			wantRuneLookup: map[runeWithMod]ActionID{
				{'r', 0}: ActionRefresh,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r1 := tt.registry1()
			r2 := tt.registry2()

			r1.Merge(r2)

			// Check that all expected actions are present in order
			actions := r1.GetActions()
			if len(actions) != len(tt.wantActionIDs) {
				t.Errorf("expected %d actions, got %d", len(tt.wantActionIDs), len(actions))
			}

			for i, wantID := range tt.wantActionIDs {
				if i >= len(actions) {
					t.Errorf("missing action at index %d: want %v", i, wantID)
					continue
				}
				if actions[i].ID != wantID {
					t.Errorf("action at index %d: want ID %v, got %v", i, wantID, actions[i].ID)
				}
			}

			// Check key lookups
			if tt.wantKeyLookup != nil {
				for key, wantID := range tt.wantKeyLookup {
					if action, exists := r1.byKey[key]; !exists {
						t.Errorf("key %v not found in byKey map", key)
					} else if action.ID != wantID {
						t.Errorf("byKey[%v]: want ID %v, got %v", key, wantID, action.ID)
					}
				}
			}

			// Check rune lookups
			if tt.wantRuneLookup != nil {
				for r, wantID := range tt.wantRuneLookup {
					if action, exists := r1.byRune[r]; !exists {
						t.Errorf("rune %q not found in byRune map", r)
					} else if action.ID != wantID {
						t.Errorf("byRune[%q]: want ID %v, got %v", r, wantID, action.ID)
					}
				}
			}
		})
	}
}

func TestActionRegistry_Register(t *testing.T) {
	tests := []struct {
		name          string
		actions       []Action
		wantCount     int
		wantByKeyLen  int
		wantByRuneLen int
	}{
		{
			name: "register rune action",
			actions: []Action{
				{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit"},
			},
			wantCount:     1,
			wantByKeyLen:  0,
			wantByRuneLen: 1,
		},
		{
			name: "register special key action",
			actions: []Action{
				{ID: ActionBack, Key: tcell.KeyEscape, Label: "Back"},
			},
			wantCount:     1,
			wantByKeyLen:  1,
			wantByRuneLen: 0,
		},
		{
			name: "register multiple mixed actions",
			actions: []Action{
				{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit"},
				{ID: ActionBack, Key: tcell.KeyEscape, Label: "Back"},
				{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Label: "Save"},
			},
			wantCount:     3,
			wantByKeyLen:  2,
			wantByRuneLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewActionRegistry()
			for _, action := range tt.actions {
				r.Register(action)
			}

			if len(r.actions) != tt.wantCount {
				t.Errorf("actions count: want %d, got %d", tt.wantCount, len(r.actions))
			}
			if len(r.byKey) != tt.wantByKeyLen {
				t.Errorf("byKey count: want %d, got %d", tt.wantByKeyLen, len(r.byKey))
			}
			if len(r.byRune) != tt.wantByRuneLen {
				t.Errorf("byRune count: want %d, got %d", tt.wantByRuneLen, len(r.byRune))
			}
		})
	}
}

func TestActionRegistry_Match(t *testing.T) {
	tests := []struct {
		name       string
		registry   func() *ActionRegistry
		event      *tcell.EventKey
		wantMatch  ActionID
		shouldFind bool
	}{
		{
			name: "match rune action",
			registry: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit"})
				return r
			},
			event:      tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone),
			wantMatch:  ActionQuit,
			shouldFind: true,
		},
		{
			name: "match special key action",
			registry: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionBack, Key: tcell.KeyEscape, Label: "Back"})
				return r
			},
			event:      tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
			wantMatch:  ActionBack,
			shouldFind: true,
		},
		{
			name: "match key with modifier",
			registry: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Modifier: tcell.ModCtrl, Label: "Save"})
				return r
			},
			event:      tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModCtrl),
			wantMatch:  ActionSaveTask,
			shouldFind: true,
		},
		{
			name: "match key with shift modifier",
			registry: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionMoveTaskRight, Key: tcell.KeyRight, Modifier: tcell.ModShift, Label: "Move →"})
				return r
			},
			event:      tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift),
			wantMatch:  ActionMoveTaskRight,
			shouldFind: true,
		},
		{
			name: "no match - wrong rune",
			registry: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit"})
				return r
			},
			event:      tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone),
			shouldFind: false,
		},
		{
			name: "no match - wrong modifier",
			registry: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Modifier: tcell.ModCtrl, Label: "Save"})
				return r
			},
			event:      tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone),
			shouldFind: false,
		},
		{
			name: "match first when multiple actions registered",
			registry: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionNavLeft, Key: tcell.KeyLeft, Label: "←"})
				r.Register(Action{ID: ActionMoveTaskLeft, Key: tcell.KeyLeft, Modifier: tcell.ModShift, Label: "Move ←"})
				return r
			},
			event:      tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone),
			wantMatch:  ActionNavLeft,
			shouldFind: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.registry()
			action := r.Match(tt.event)

			if !tt.shouldFind {
				if action != nil {
					t.Errorf("expected no match, got action %v", action.ID)
				}
			} else {
				if action == nil {
					t.Errorf("expected match for action %v, got nil", tt.wantMatch)
				} else if action.ID != tt.wantMatch {
					t.Errorf("expected action %v, got %v", tt.wantMatch, action.ID)
				}
			}
		})
	}
}

func TestActionRegistry_GetHeaderActions(t *testing.T) {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit", ShowInHeader: true})
	r.Register(Action{ID: ActionNavLeft, Key: tcell.KeyLeft, Label: "←", ShowInHeader: false})
	r.Register(Action{ID: ActionNavRight, Key: tcell.KeyRight, Label: "→", ShowInHeader: false})

	headerActions := r.GetHeaderActions()

	if len(headerActions) != 1 {
		t.Errorf("expected 1 header actions, got %d", len(headerActions))
	}

	expectedIDs := []ActionID{ActionQuit}
	for i, action := range headerActions {
		if action.ID != expectedIDs[i] { //nolint:gosec // G602: len verified equal on line 312
			t.Errorf("header action %d: expected %v, got %v", i, expectedIDs[i], action.ID) //nolint:gosec // G602: len verified equal on line 312
		}
		if !action.ShowInHeader {
			t.Errorf("header action %d: ShowInHeader should be true", i)
		}
	}
}

func TestGetActionsForField(t *testing.T) {
	tests := []struct {
		name            string
		field           model.EditField
		wantActionCount int
		mustHaveActions []ActionID
	}{
		{
			name:            "title field has quick save and save",
			field:           model.EditFieldTitle,
			wantActionCount: 4, // QuickSave, Save, NextField, PrevField
			mustHaveActions: []ActionID{ActionQuickSave, ActionSaveTask, ActionNextField, ActionPrevField},
		},
		{
			name:            "status field has next/prev value",
			field:           model.EditFieldStatus,
			wantActionCount: 4, // NextField, PrevField, NextValue, PrevValue
			mustHaveActions: []ActionID{ActionNextField, ActionPrevField, ActionNextValue, ActionPrevValue},
		},
		{
			name:            "type field has next/prev value",
			field:           model.EditFieldType,
			wantActionCount: 4, // NextField, PrevField, NextValue, PrevValue
			mustHaveActions: []ActionID{ActionNextField, ActionPrevField, ActionNextValue, ActionPrevValue},
		},
		{
			name:            "assignee field has next/prev value",
			field:           model.EditFieldAssignee,
			wantActionCount: 4, // NextField, PrevField, NextValue, PrevValue
			mustHaveActions: []ActionID{ActionNextField, ActionPrevField, ActionNextValue, ActionPrevValue},
		},
		{
			name:            "priority field has only navigation",
			field:           model.EditFieldPriority,
			wantActionCount: 2, // NextField, PrevField
			mustHaveActions: []ActionID{ActionNextField, ActionPrevField},
		},
		{
			name:            "points field has only navigation",
			field:           model.EditFieldPoints,
			wantActionCount: 2, // NextField, PrevField
			mustHaveActions: []ActionID{ActionNextField, ActionPrevField},
		},
		{
			name:            "description field has save",
			field:           model.EditFieldDescription,
			wantActionCount: 3, // Save, NextField, PrevField
			mustHaveActions: []ActionID{ActionSaveTask, ActionNextField, ActionPrevField},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := GetActionsForField(tt.field)

			actions := registry.GetActions()
			if len(actions) != tt.wantActionCount {
				t.Errorf("expected %d actions, got %d", tt.wantActionCount, len(actions))
			}

			// Check that all required actions are present
			actionMap := make(map[ActionID]bool)
			for _, action := range actions {
				actionMap[action.ID] = true
			}

			for _, mustHave := range tt.mustHaveActions {
				if !actionMap[mustHave] {
					t.Errorf("missing required action: %v", mustHave)
				}
			}
		})
	}
}

func TestDefaultGlobalActions(t *testing.T) {
	registry := DefaultGlobalActions()
	actions := registry.GetActions()

	if len(actions) != 5 {
		t.Errorf("expected 5 global actions, got %d", len(actions))
	}

	expectedActions := []ActionID{ActionBack, ActionQuit, ActionRefresh, ActionToggleHeader, ActionOpenPalette}
	for i, expected := range expectedActions {
		if i >= len(actions) {
			t.Errorf("missing action at index %d: want %v", i, expected)
			continue
		}
		if actions[i].ID != expected {
			t.Errorf("action at index %d: want %v, got %v", i, expected, actions[i].ID)
		}
	}

	// ActionOpenPalette should NOT show in header
	for _, a := range actions {
		if a.ID == ActionOpenPalette {
			if a.ShowInHeader {
				t.Error("ActionOpenPalette should have ShowInHeader=false")
			}
			continue
		}
		if !a.ShowInHeader {
			t.Errorf("global action %v should have ShowInHeader=true", a.ID)
		}
	}
}

func TestTaskDetailViewActions(t *testing.T) {
	registry := TaskDetailViewActions()
	actions := registry.GetActions()

	if len(actions) != 6 {
		t.Errorf("expected 6 task detail actions, got %d", len(actions))
	}

	expectedActions := []ActionID{ActionEditTitle, ActionEditDesc, ActionEditSource, ActionFullscreen, ActionEditDeps, ActionEditTags}
	for i, expected := range expectedActions {
		if i >= len(actions) {
			t.Errorf("missing action at index %d: want %v", i, expected)
			continue
		}
		if actions[i].ID != expected {
			t.Errorf("action at index %d: want %v, got %v", i, expected, actions[i].ID)
		}
	}
}

func TestCommonFieldNavigationActions(t *testing.T) {
	registry := CommonFieldNavigationActions()
	actions := registry.GetActions()

	if len(actions) != 2 {
		t.Errorf("expected 2 navigation actions, got %d", len(actions))
	}

	expectedActions := []ActionID{ActionNextField, ActionPrevField}
	for i, expected := range expectedActions {
		if i >= len(actions) {
			t.Errorf("missing action at index %d: want %v", i, expected)
			continue
		}
		if actions[i].ID != expected {
			t.Errorf("action at index %d: want %v, got %v", i, expected, actions[i].ID)
		}
		if !actions[i].ShowInHeader {
			t.Errorf("navigation action %v should have ShowInHeader=true", expected)
		}
	}

	// Verify specific keys
	if actions[0].Key != tcell.KeyTab {
		t.Errorf("NextField should use Tab key, got %v", actions[0].Key)
	}
	if actions[1].Key != tcell.KeyBacktab {
		t.Errorf("PrevField should use Backtab key, got %v", actions[1].Key)
	}
}

func TestDescOnlyEditActions(t *testing.T) {
	registry := DescOnlyEditActions()
	actions := registry.GetActions()

	if len(actions) != 1 {
		t.Errorf("expected 1 desc-only action, got %d", len(actions))
	}

	if actions[0].ID != ActionSaveTask {
		t.Errorf("expected save action, got %v", actions[0].ID)
	}

	// verify no Tab/Backtab actions
	for _, a := range actions {
		if a.ID == ActionNextField || a.ID == ActionPrevField {
			t.Errorf("desc-only actions should not include field navigation, found %v", a.ID)
		}
	}
}

func TestTaskDetailViewActions_HasEditDesc(t *testing.T) {
	registry := TaskDetailViewActions()

	// Shift+D should match ActionEditDesc
	event := tcell.NewEventKey(tcell.KeyRune, 'D', tcell.ModNone)
	action := registry.Match(event)
	if action == nil {
		t.Fatal("expected Shift+D to match an action")
		return
	}
	if action.ID != ActionEditDesc {
		t.Errorf("expected ActionEditDesc, got %v", action.ID)
	}
}

func TestTaskDetailViewActions_NoChatWithoutConfig(t *testing.T) {
	// ensure ai.agent is not set
	viper.Set("ai.agent", "")
	defer viper.Set("ai.agent", "")

	registry := TaskDetailViewActions()
	_, found := registry.LookupRune('c')
	if found {
		t.Error("chat action should not be registered when ai.agent is empty")
	}
}

func TestTaskDetailViewActions_ChatWithConfig(t *testing.T) {
	viper.Set("ai.agent", "claude")
	defer viper.Set("ai.agent", "")

	registry := TaskDetailViewActions()

	action, found := registry.LookupRune('c')
	if !found {
		t.Fatal("chat action should be registered when ai.agent is configured")
	}
	if action.ID != ActionChat {
		t.Errorf("expected ActionChat, got %v", action.ID)
	}
	if !action.ShowInHeader {
		t.Error("chat action should be shown in header")
	}

	// total count should be 7 (6 base + chat)
	actions := registry.GetActions()
	if len(actions) != 7 {
		t.Errorf("expected 7 actions with ai.agent configured, got %d", len(actions))
	}
}

func TestPluginActionID(t *testing.T) {
	id := pluginActionID('b')
	if id != "plugin_action:b" {
		t.Errorf("expected 'plugin_action:b', got %q", id)
	}

	r := getPluginActionRune(id)
	if r != 'b' {
		t.Errorf("expected 'b', got %q", r)
	}
}

func TestLookupRune_ConflictDetection(t *testing.T) {
	r := PluginViewActions()

	// built-in plugin view keys should be found
	conflicting := []rune{'k', 'j', 'h', 'l', 'n', 'd', '/', 'v'}
	for _, ch := range conflicting {
		if _, ok := r.LookupRune(ch); !ok {
			t.Errorf("expected built-in action for rune %q", ch)
		}
	}

	// non-conflicting key should not be found
	if _, ok := r.LookupRune('b'); ok {
		t.Errorf("rune 'b' should not conflict with built-in actions")
	}

	// global actions
	g := DefaultGlobalActions()
	if _, ok := g.LookupRune('q'); !ok {
		t.Error("expected global action for rune 'q'")
	}
	if _, ok := g.LookupRune('r'); !ok {
		t.Error("expected global action for rune 'r'")
	}
}

func TestGetPluginActionRune_NotPluginAction(t *testing.T) {
	tests := []struct {
		name string
		id   ActionID
	}{
		{"empty", ""},
		{"built-in", ActionNavUp},
		{"plugin activation", "plugin:Kanban"},
		{"partial prefix", "plugin_action:"},
		{"multi-char", ActionID("plugin_action:ab")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := getPluginActionRune(tc.id)
			if r != 0 {
				t.Errorf("expected 0, got %q", r)
			}
		})
	}
}

func TestMatchWithModifiers(t *testing.T) {
	registry := NewActionRegistry()

	// Register action requiring Alt-M
	registry.Register(Action{
		ID:       "test_alt_m",
		Key:      tcell.KeyRune,
		Rune:     'M',
		Modifier: tcell.ModAlt,
	})

	// Test Alt-M matches
	event := tcell.NewEventKey(tcell.KeyRune, 'M', tcell.ModAlt)
	match := registry.Match(event)
	if match == nil || match.ID != "test_alt_m" {
		t.Error("Alt-M should match action with Alt-M binding")
	}

	// Test plain M does NOT match
	event = tcell.NewEventKey(tcell.KeyRune, 'M', 0)
	match = registry.Match(event)
	if match != nil {
		t.Error("M (no modifier) should not match action with Alt-M binding")
	}
}

func TestGetPaletteActions_HidesMarkedActions(t *testing.T) {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit"})
	r.Register(Action{ID: ActionBack, Key: tcell.KeyEscape, Label: "Back", HideFromPalette: true})
	r.Register(Action{ID: ActionRefresh, Key: tcell.KeyRune, Rune: 'r', Label: "Refresh"})

	actions := r.GetPaletteActions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 palette actions, got %d", len(actions))
	}
	if actions[0].ID != ActionQuit {
		t.Errorf("expected first action to be Quit, got %v", actions[0].ID)
	}
	if actions[1].ID != ActionRefresh {
		t.Errorf("expected second action to be Refresh, got %v", actions[1].ID)
	}
}

func TestGetPaletteActions_DedupsByActionID(t *testing.T) {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionNavUp, Key: tcell.KeyUp, Label: "↑"})
	r.Register(Action{ID: ActionNavUp, Key: tcell.KeyRune, Rune: 'k', Label: "↑ (vim)"})
	r.Register(Action{ID: ActionNavDown, Key: tcell.KeyDown, Label: "↓"})

	actions := r.GetPaletteActions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 deduped actions, got %d", len(actions))
	}
	if actions[0].ID != ActionNavUp {
		t.Errorf("expected first action to be NavUp, got %v", actions[0].ID)
	}
	if actions[0].Key != tcell.KeyUp {
		t.Error("dedup should keep first registered binding (arrow key), not vim key")
	}
	if actions[1].ID != ActionNavDown {
		t.Errorf("expected second action to be NavDown, got %v", actions[1].ID)
	}
}

func TestGetPaletteActions_NilRegistry(t *testing.T) {
	var r *ActionRegistry
	if actions := r.GetPaletteActions(); actions != nil {
		t.Errorf("expected nil from nil registry, got %v", actions)
	}
}

func TestContainsID(t *testing.T) {
	r := NewActionRegistry()
	r.Register(Action{ID: ActionQuit, Key: tcell.KeyRune, Rune: 'q', Label: "Quit"})
	r.Register(Action{ID: ActionBack, Key: tcell.KeyEscape, Label: "Back"})

	if !r.ContainsID(ActionQuit) {
		t.Error("expected ContainsID to find ActionQuit")
	}
	if !r.ContainsID(ActionBack) {
		t.Error("expected ContainsID to find ActionBack")
	}
	if r.ContainsID(ActionRefresh) {
		t.Error("expected ContainsID to not find ActionRefresh")
	}
}

func TestContainsID_NilRegistry(t *testing.T) {
	var r *ActionRegistry
	if r.ContainsID(ActionQuit) {
		t.Error("expected false from nil registry")
	}
}

func TestDefaultGlobalActions_BackHiddenFromPalette(t *testing.T) {
	registry := DefaultGlobalActions()
	paletteActions := registry.GetPaletteActions()

	for _, a := range paletteActions {
		if a.ID == ActionBack {
			t.Error("ActionBack should be hidden from palette")
		}
	}

	if !registry.ContainsID(ActionBack) {
		t.Error("ActionBack should still be registered in global actions")
	}
}

func TestPluginViewActions_NavHiddenFromPalette(t *testing.T) {
	registry := PluginViewActions()
	paletteActions := registry.GetPaletteActions()

	navIDs := map[ActionID]bool{
		ActionNavUp: true, ActionNavDown: true,
		ActionNavLeft: true, ActionNavRight: true,
	}
	for _, a := range paletteActions {
		if navIDs[a.ID] {
			t.Errorf("navigation action %v should be hidden from palette", a.ID)
		}
	}

	// semantic actions should remain visible
	found := map[ActionID]bool{}
	for _, a := range paletteActions {
		found[a.ID] = true
	}
	for _, want := range []ActionID{ActionOpenFromPlugin, ActionNewTask, ActionDeleteTask, ActionSearch} {
		if !found[want] {
			t.Errorf("expected palette-visible action %v", want)
		}
	}
}

func TestTaskEditActions_FieldLocalHidden_SaveVisible(t *testing.T) {
	registry := TaskEditTitleActions()
	paletteActions := registry.GetPaletteActions()

	found := map[ActionID]bool{}
	for _, a := range paletteActions {
		found[a.ID] = true
	}

	if !found[ActionSaveTask] {
		t.Error("Save should be palette-visible in task edit")
	}
	if found[ActionQuickSave] {
		t.Error("Quick Save should be hidden from palette")
	}
	if found[ActionNextField] {
		t.Error("Next field should be hidden from palette")
	}
	if found[ActionPrevField] {
		t.Error("Prev field should be hidden from palette")
	}
}

func TestInitPluginActions_ActivePluginDisabled(t *testing.T) {
	InitPluginActions([]PluginInfo{
		{Name: "Kanban", Key: tcell.KeyRune, Rune: '1'},
		{Name: "Backlog", Key: tcell.KeyRune, Rune: '2'},
	})

	registry := GetPluginActions()
	actions := registry.GetPaletteActions()

	kanbanViewEntry := &ViewEntry{ViewID: model.MakePluginViewID("Kanban")}
	backlogViewEntry := &ViewEntry{ViewID: model.MakePluginViewID("Backlog")}

	for _, a := range actions {
		if a.ID == "plugin:Kanban" {
			if a.IsEnabled == nil {
				t.Fatal("expected IsEnabled on plugin:Kanban")
			}
			if a.IsEnabled(kanbanViewEntry, nil) {
				t.Error("Kanban activation should be disabled when Kanban view is active")
			}
			if !a.IsEnabled(backlogViewEntry, nil) {
				t.Error("Kanban activation should be enabled when Backlog view is active")
			}
		}
	}
}
