package controller

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"

	"github.com/gdamore/tcell/v2"
)

func TestActionRegistry_Merge(t *testing.T) {
	tests := []struct {
		name           string
		registry1      func() *ActionRegistry
		registry2      func() *ActionRegistry
		wantActionIDs  []ActionID
		wantKeyLookup  map[tcell.Key]ActionID
		wantRuneLookup map[rune]ActionID
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
			wantKeyLookup: map[tcell.Key]ActionID{
				tcell.KeyEscape: ActionBack,
			},
			wantRuneLookup: map[rune]ActionID{
				'q': ActionQuit,
				'r': ActionRefresh,
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
			wantRuneLookup: map[rune]ActionID{
				'q': ActionSearch, // overwritten by second registry
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
			wantRuneLookup: map[rune]ActionID{
				'q': ActionQuit,
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
			wantRuneLookup: map[rune]ActionID{
				'r': ActionRefresh,
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
		if action.ID != expectedIDs[i] {
			t.Errorf("header action %d: expected %v, got %v", i, expectedIDs[i], action.ID)
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

	if len(actions) != 4 {
		t.Errorf("expected 4 global actions, got %d", len(actions))
	}

	expectedActions := []ActionID{ActionBack, ActionQuit, ActionRefresh, ActionToggleHeader}
	for i, expected := range expectedActions {
		if i >= len(actions) {
			t.Errorf("missing action at index %d: want %v", i, expected)
			continue
		}
		if actions[i].ID != expected {
			t.Errorf("action at index %d: want %v, got %v", i, expected, actions[i].ID)
		}
		if !actions[i].ShowInHeader {
			t.Errorf("global action %v should have ShowInHeader=true", expected)
		}
	}
}

func TestTaskDetailViewActions(t *testing.T) {
	registry := TaskDetailViewActions()
	actions := registry.GetActions()

	if len(actions) != 3 {
		t.Errorf("expected 3 task detail actions, got %d", len(actions))
	}

	expectedActions := []ActionID{ActionEditTitle, ActionEditSource, ActionFullscreen}
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
