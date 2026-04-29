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
			name: "ctrl key without explicit ModCtrl still matches via normalization",
			registry: func() *ActionRegistry {
				r := NewActionRegistry()
				r.Register(Action{ID: ActionSaveTask, Key: tcell.KeyCtrlS, Modifier: tcell.ModCtrl, Label: "Save"})
				return r
			},
			event:      tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone),
			wantMatch:  ActionSaveTask,
			shouldFind: true,
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

	if len(actions) != 6 {
		t.Errorf("expected 6 global actions, got %d", len(actions))
	}

	expectedActions := []ActionID{ActionBack, ActionQuit, ActionRefresh, ActionToggleHeader, ActionOpenPalette, ActionEditWorkflow}
	for i, expected := range expectedActions {
		if i >= len(actions) {
			t.Errorf("missing action at index %d: want %v", i, expected)
			continue
		}
		if actions[i].ID != expected {
			t.Errorf("action at index %d: want %v, got %v", i, expected, actions[i].ID)
		}
	}

	// ActionOpenPalette should show in header with label "All" and use Ctrl+A binding
	for _, a := range actions {
		if a.ID == ActionOpenPalette {
			if !a.ShowInHeader {
				t.Error("ActionOpenPalette should have ShowInHeader=true")
			}
			if a.Label != "All" {
				t.Errorf("ActionOpenPalette label = %q, want %q", a.Label, "All")
			}
			if a.Key != tcell.KeyCtrlA {
				t.Errorf("ActionOpenPalette Key = %v, want KeyCtrlA", a.Key)
			}
			if a.Modifier != tcell.ModCtrl {
				t.Errorf("ActionOpenPalette Modifier = %v, want ModCtrl", a.Modifier)
			}
			if a.Rune != 0 {
				t.Errorf("ActionOpenPalette Rune = %v, want 0", a.Rune)
			}
			continue
		}
		// ActionEditWorkflow is palette-only (no key, no header)
		if a.ID == ActionEditWorkflow {
			if a.ShowInHeader {
				t.Error("ActionEditWorkflow should have ShowInHeader=false")
			}
			if a.Label != "Edit Workflow" {
				t.Errorf("ActionEditWorkflow label = %q, want %q", a.Label, "Edit Workflow")
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

	if len(actions) != 7 {
		t.Errorf("expected 7 task detail actions (always includes Chat), got %d", len(actions))
	}

	expectedActions := []ActionID{ActionEditTitle, ActionEditDesc, ActionEditSource, ActionFullscreen, ActionEditDeps, ActionEditTags, ActionChat}
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

func TestTaskDetailViewActions_ChatAlwaysRegistered(t *testing.T) {
	viper.Set("ai.agent", "")
	defer viper.Set("ai.agent", "")

	registry := TaskDetailViewActions()
	action, found := registry.LookupRune('c')
	if !found {
		t.Fatal("chat action should always be registered")
	}
	if action.ID != ActionChat {
		t.Errorf("expected ActionChat, got %v", action.ID)
	}

	// with no AI agent, ActionEnabled should return false
	ctx := NewAppContext()
	ctx.Set(string(RequireID))
	if ActionEnabled(action, ctx) {
		t.Error("chat should be disabled when ai.agent is empty")
	}

	// total count should always be 7
	actions := registry.GetActions()
	if len(actions) != 7 {
		t.Errorf("expected 7 actions, got %d", len(actions))
	}
}

func TestTaskDetailViewActions_ChatEnabledWithConfig(t *testing.T) {
	viper.Set("ai.agent", "claude")
	defer viper.Set("ai.agent", "")

	registry := TaskDetailViewActions()

	action, found := registry.LookupRune('c')
	if !found {
		t.Fatal("chat action should be registered")
	}
	if action.ID != ActionChat {
		t.Errorf("expected ActionChat, got %v", action.ID)
	}
	if !action.ShowInHeader {
		t.Error("chat action should be shown in header")
	}

	ctx := BuildAppContext(
		&ViewEntry{ViewID: model.TaskDetailViewID, Params: model.EncodeTaskDetailParams(model.TaskDetailParams{TaskID: "ABC123"})},
		nil,
	)
	if !ActionEnabled(action, ctx) {
		t.Error("chat should be enabled when ai.agent is configured and task ID present")
	}
}

func TestPluginActionID(t *testing.T) {
	id := pluginActionID("b")
	if id != "plugin_action:b" {
		t.Errorf("expected 'plugin_action:b', got %q", id)
	}

	keyStr := getPluginActionKeyStr(id)
	if keyStr != "b" {
		t.Errorf("expected 'b', got %q", keyStr)
	}

	// composite key
	id = pluginActionID("Ctrl-U")
	if id != "plugin_action:Ctrl-U" {
		t.Errorf("expected 'plugin_action:Ctrl-U', got %q", id)
	}
	keyStr = getPluginActionKeyStr(id)
	if keyStr != "Ctrl-U" {
		t.Errorf("expected 'Ctrl-U', got %q", keyStr)
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

func TestGetPluginActionKeyStr_NotPluginAction(t *testing.T) {
	tests := []struct {
		name string
		id   ActionID
		want string
	}{
		{"empty", "", ""},
		{"built-in", ActionNavUp, ""},
		{"plugin activation", "plugin:Kanban", ""},
		{"partial prefix returns empty", ActionID("plugin_action:"), ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			keyStr := getPluginActionKeyStr(tc.id)
			if keyStr != tc.want {
				t.Errorf("expected %q, got %q", tc.want, keyStr)
			}
		})
	}

	// multi-char suffix is now valid (e.g. "Ctrl-U")
	t.Run("multi-char suffix valid for composite keys", func(t *testing.T) {
		keyStr := getPluginActionKeyStr(ActionID("plugin_action:Ctrl-U"))
		if keyStr != "Ctrl-U" {
			t.Errorf("expected 'Ctrl-U', got %q", keyStr)
		}
	})
}

func TestMatchBinding_CtrlLetterNormalization(t *testing.T) {
	registry := NewActionRegistry()
	registry.Register(Action{
		ID:       "test_ctrl_u",
		Key:      tcell.KeyCtrlU,
		Modifier: tcell.ModCtrl,
	})

	// all three encodings should match the same action
	if a := registry.MatchBinding(tcell.KeyCtrlU, 0, tcell.ModCtrl); a == nil || a.ID != "test_ctrl_u" {
		t.Error("KeyCtrlU+ModCtrl should match")
	}
	if a := registry.MatchBinding(tcell.KeyCtrlU, 0, 0); a == nil || a.ID != "test_ctrl_u" {
		t.Error("KeyCtrlU+0 should match via normalization")
	}
	if a := registry.MatchBinding('U', 0, tcell.ModCtrl); a == nil || a.ID != "test_ctrl_u" {
		t.Error("Key='U'+ModCtrl should match via normalization")
	}
}

func TestMatchBinding_ExactRune(t *testing.T) {
	registry := NewActionRegistry()
	registry.Register(Action{
		ID:   "test_q",
		Key:  tcell.KeyRune,
		Rune: 'q',
	})

	if a := registry.MatchBinding(tcell.KeyRune, 'q', 0); a == nil || a.ID != "test_q" {
		t.Error("expected match for rune 'q'")
	}
	if a := registry.MatchBinding(tcell.KeyRune, 'x', 0); a != nil {
		t.Error("expected no match for rune 'x'")
	}
}

func TestMatchBinding_UnmodifiedRuneFallback(t *testing.T) {
	registry := NewActionRegistry()
	registry.Register(Action{
		ID:   "test_q",
		Key:  tcell.KeyRune,
		Rune: 'q',
	})

	// unmodified rune action should match even with a modifier present
	if a := registry.MatchBinding(tcell.KeyRune, 'q', tcell.ModAlt); a == nil || a.ID != "test_q" {
		t.Error("unmodified rune action should match rune with any modifier")
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
			if len(a.Require) == 0 {
				t.Fatal("expected Require on plugin:Kanban")
			}
			kanbanCtx := BuildAppContext(kanbanViewEntry, nil)
			if ActionEnabled(a, kanbanCtx) {
				t.Error("Kanban activation should be disabled when Kanban view is active")
			}
			backlogCtx := BuildAppContext(backlogViewEntry, nil)
			if !ActionEnabled(a, backlogCtx) {
				t.Error("Kanban activation should be enabled when Backlog view is active")
			}
		}
	}
}

func TestAppContext_SetHasDeleteClone(t *testing.T) {
	ctx := NewAppContext()

	if ctx.Has("id") {
		t.Error("fresh context should not have 'id'")
	}

	ctx.Set("id")
	if !ctx.Has("id") {
		t.Error("context should have 'id' after Set")
	}

	ctx.Set("ai")
	if !ctx.Has("ai") {
		t.Error("context should have 'ai' after Set")
	}

	clone := ctx.Clone()
	if !clone.Has("id") || !clone.Has("ai") {
		t.Error("clone should have all attributes from original")
	}

	ctx.Delete("id")
	if ctx.Has("id") {
		t.Error("context should not have 'id' after Delete")
	}
	if !clone.Has("id") {
		t.Error("clone should be independent from original")
	}
}

func TestAppContext_ArbitraryAttributes(t *testing.T) {
	ctx := NewAppContext()
	ctx.Set("team:backend")
	ctx.Set("view:plugin:Kanban")

	if !ctx.Has("team:backend") {
		t.Error("should have arbitrary attribute 'team:backend'")
	}
	if !ctx.Has("view:plugin:Kanban") {
		t.Error("should have arbitrary attribute 'view:plugin:Kanban'")
	}
}

func TestActionEnabled(t *testing.T) {
	tests := []struct {
		name    string
		require []Requirement
		attrs   []string
		want    bool
	}{
		{"no requirements always enabled", nil, nil, true},
		{"empty requirements always enabled", []Requirement{}, nil, true},
		{"positive met", []Requirement{"id"}, []string{"id"}, true},
		{"positive unmet", []Requirement{"id"}, nil, false},
		{"multiple positive all met", []Requirement{"id", "ai"}, []string{"id", "ai"}, true},
		{"multiple positive one unmet", []Requirement{"id", "ai"}, []string{"id"}, false},
		{"negated met (absent)", []Requirement{"!view:plugin:Kanban"}, nil, true},
		{"negated unmet (present)", []Requirement{"!view:plugin:Kanban"}, []string{"view:plugin:Kanban"}, false},
		{"mixed positive and negated both met", []Requirement{"id", "!view:plugin:Kanban"}, []string{"id"}, true},
		{"mixed positive met but negated unmet", []Requirement{"id", "!view:plugin:Kanban"}, []string{"id", "view:plugin:Kanban"}, false},
		{"custom requirement", []Requirement{"foo"}, []string{"foo"}, true},
		{"custom requirement absent", []Requirement{"foo"}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewAppContext()
			for _, a := range tt.attrs {
				ctx.Set(a)
			}
			action := Action{Require: tt.require}
			got := ActionEnabled(action, ctx)
			if got != tt.want {
				t.Errorf("ActionEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildAppContext_SelectableView(t *testing.T) {
	pluginViewEntry := &ViewEntry{ViewID: model.MakePluginViewID("Kanban")}

	mockView := &mockSelectableView{selectedID: "ABC123"}
	ctx := BuildAppContext(pluginViewEntry, mockView)
	if !ctx.Has("id") {
		t.Error("context should have 'id' when view has selection")
	}
	if !ctx.Has(string(RequireSelectionOne)) {
		t.Error("context should have 'selection:one' when view has single selection")
	}
	if !ctx.Has(string(RequireSelectionAny)) {
		t.Error("context should have 'selection:any' when view has single selection")
	}
	if ctx.Has(string(RequireSelectionMany)) {
		t.Error("context should NOT have 'selection:many' for single selection")
	}
	if !ctx.Has("view:" + string(model.MakePluginViewID("Kanban"))) {
		t.Error("context should have view identity attribute")
	}

	emptyView := &mockSelectableView{selectedID: ""}
	ctx = BuildAppContext(pluginViewEntry, emptyView)
	if ctx.Has("id") {
		t.Error("context should not have 'id' when view has no selection")
	}
	if ctx.Has(string(RequireSelectionAny)) {
		t.Error("context should not have 'selection:any' when nothing selected")
	}
}

func TestSelectionSatisfies(t *testing.T) {
	tests := []struct {
		name  string
		reqs  []string
		count int
		want  bool
	}{
		{"id + exactly one", []string{"id"}, 1, true},
		{"id + zero", []string{"id"}, 0, false},
		{"id + two rejects", []string{"id"}, 2, false},
		{"selection:one + exactly one", []string{"selection:one"}, 1, true},
		{"selection:one + three rejects", []string{"selection:one"}, 3, false},
		{"selection:any + zero rejects", []string{"selection:any"}, 0, false},
		{"selection:any + one ok", []string{"selection:any"}, 1, true},
		{"selection:any + five ok", []string{"selection:any"}, 5, true},
		{"selection:many + one rejects", []string{"selection:many"}, 1, false},
		{"selection:many + two ok", []string{"selection:many"}, 2, true},
		{"no requirements + zero ok", nil, 0, true},
		{"unrelated requirement ignored", []string{"ai"}, 0, true},
		// negated selection cardinality — honored just like other negated reqs
		{"!selection:any + zero ok", []string{"!selection:any"}, 0, true},
		{"!selection:any + one rejects", []string{"!selection:any"}, 1, false},
		{"!selection:many + one ok", []string{"!selection:many"}, 1, true},
		{"!selection:many + two rejects", []string{"!selection:many"}, 2, false},
		{"!id + two ok", []string{"!id"}, 2, true},
		{"!id + one rejects", []string{"!id"}, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectionSatisfies(tt.reqs, tt.count); got != tt.want {
				t.Errorf("selectionSatisfies(%v, %d) = %v, want %v", tt.reqs, tt.count, got, tt.want)
			}
		})
	}
}

func TestBuildAppContext_TaskDetail(t *testing.T) {
	entry := &ViewEntry{
		ViewID: model.TaskDetailViewID,
		Params: model.EncodeTaskDetailParams(model.TaskDetailParams{TaskID: "ABC123"}),
	}
	ctx := BuildAppContext(entry, nil)
	if !ctx.Has("id") {
		t.Error("context should have 'id' from task detail params")
	}

	emptyEntry := &ViewEntry{
		ViewID: model.TaskDetailViewID,
		Params: model.EncodeTaskDetailParams(model.TaskDetailParams{}),
	}
	ctx = BuildAppContext(emptyEntry, nil)
	if ctx.Has("id") {
		t.Error("context should not have 'id' with empty task detail params")
	}
}

func TestNoCallbackBasedEnablement(t *testing.T) {
	registries := []*ActionRegistry{
		DefaultGlobalActions(),
		TaskDetailViewActions(),
		PluginViewActions(),
		DepsViewActions(),
		TaskEditViewActions(),
		ReadonlyTaskDetailViewActions(),
	}

	for _, r := range registries {
		for _, a := range r.GetActions() {
			if len(a.Require) > 0 {
				for _, req := range a.Require {
					if req == "" {
						t.Errorf("action %v has empty requirement string", a.ID)
					}
				}
			}
		}
	}
}

func TestEditWorkflow_EmptyPath(t *testing.T) {
	statusline := model.NewStatuslineConfig()
	ir := &InputRouter{
		navController: newMockNavigationController(),
		globalActions: DefaultGlobalActions(),
		statusline:    statusline,
		workflowPath:  "",
	}

	handled := ir.handleGlobalAction(ActionEditWorkflow)
	if !handled {
		t.Error("ActionEditWorkflow should be handled")
	}

	msg, level, _ := statusline.GetMessage()
	if msg != "no workflow file found" {
		t.Errorf("message = %q, want %q", msg, "no workflow file found")
	}
	if level != model.MessageLevelError {
		t.Errorf("level = %v, want MessageLevelError", level)
	}
}
