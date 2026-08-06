package controller

import (
	"testing"
	"time"

	"github.com/boolean-maybe/ruki/recurrence"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

func init() {
	teststatuses.Init()
}

// Test Draft Tiki Lifecycle

func TestTikiEditSession_SetDraft(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	draft := newTestTiki()
	tc.SetDraft(draft)

	if tc.GetDraftTiki() == nil {
		t.Error("SetDraft did not set the draft tiki")
	}

	if tc.GetCurrentTikiID() != draft.ID() {
		t.Errorf("SetDraft did not set currentTikiID, got %q, want %q", tc.GetCurrentTikiID(), draft.ID())
	}
}

func TestTikiEditSession_ClearDraft(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	tc.SetDraft(newTestTiki())
	tc.ClearDraft()

	if tc.GetDraftTiki() != nil {
		t.Error("ClearDraft did not clear the draft tiki")
	}
}

func TestTikiEditSession_StartEditSession(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	// Create a tiki in the store
	original := newTestTiki()
	original.LoadedMtime = time.Now()
	_ = tikiStore.CreateTiki(original)

	// Start edit session
	editingTiki := tc.StartEditSession(original.ID())

	if editingTiki == nil {
		t.Fatal("StartEditSession returned nil")
		return
	}

	if editingTiki.ID() != original.ID() {
		t.Errorf("StartEditSession returned wrong tiki, got ID %q, want %q", editingTiki.ID(), original.ID())
	}

	if tc.GetEditingTiki() == nil {
		t.Error("StartEditSession did not set editingTiki")
	}

	if tc.GetCurrentTikiID() != original.ID() {
		t.Errorf("StartEditSession did not set currentTikiID, got %q, want %q", tc.GetCurrentTikiID(), original.ID())
	}
}

func TestTikiEditSession_StartEditSession_NonExistent(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	editingTiki := tc.StartEditSession("NONEXISTENT")

	if editingTiki != nil {
		t.Error("StartEditSession should return nil for non-existent tiki")
	}
}

func TestTikiEditSession_CancelEditSession(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	// Start an edit session
	original := newTestTiki()
	_ = tikiStore.CreateTiki(original)
	tc.StartEditSession(original.ID())

	// Cancel it
	tc.CancelEditSession()

	if tc.GetEditingTiki() != nil {
		t.Error("CancelEditSession did not clear editingTiki")
	}

	if tc.GetCurrentTikiID() != "" {
		t.Errorf("CancelEditSession did not clear currentTikiID, got %q", tc.GetCurrentTikiID())
	}
}

// TestTikiEditSession_SaveWorkflowEnum pins the path workflow enum fields take
// when their SemanticEnum editor commits a value.
func TestTikiEditSession_SaveWorkflowEnum(t *testing.T) {
	// register an enum field for the duration of this test
	teststatuses.Init()
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{
			Name: "severity",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "low"},
				{Value: "medium", Default: true},
				{Value: "high"},
			},
		},
		{Name: "note", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("register severity: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tests := []struct {
		name        string
		field       string
		value       string
		wantSuccess bool
		wantStored  string // empty means "should be deleted/absent"
	}{
		{"valid key", "severity", "high", true, "high"},
		{"empty deletes", "severity", "", true, ""},
		{"unknown key rejected", "severity", "bogus", false, ""},
		{"non-enum field rejected", "note", "high", false, ""},
		{"unregistered field rejected", "nonexistent", "any", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tikiStore := store.NewInMemoryStore()
			gate := service.NewTikiMutationGate()
			gate.SetStore(tikiStore)
			navController := newMockNavigationController()
			tc := NewTikiEditSession(tikiStore, gate, navController, nil)
			tc.SetDraft(newTestTiki())
			// Pre-seed the field so we can observe deletion when value=="".
			tc.draftTiki.Set("severity", "low")

			got := tc.SaveWorkflowEnum(tt.field, tt.value)
			if got != tt.wantSuccess {
				t.Errorf("SaveWorkflowEnum(%q,%q) = %v, want %v", tt.field, tt.value, got, tt.wantSuccess)
			}
			if !tt.wantSuccess {
				return
			}
			if tt.wantStored == "" {
				if tc.draftTiki.Has(tt.field) {
					stored, _, _ := tc.draftTiki.StringField(tt.field)
					t.Errorf("field %q still present with value %q after empty save", tt.field, stored)
				}
				return
			}
			stored, _, _ := tc.draftTiki.StringField(tt.field)
			if stored != tt.wantStored {
				t.Errorf("draftTiki.%s = %q, want %q", tt.field, stored, tt.wantStored)
			}
		})
	}
}

func TestTikiEditSession_SaveTitle(t *testing.T) {
	tests := []struct {
		name        string
		setupTiki   func(*TikiEditSession, store.Store)
		title       string
		wantTitle   string
		wantSuccess bool
	}{
		{
			name: "valid title on draft tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			title:       "New Title",
			wantTitle:   "New Title",
			wantSuccess: true,
		},
		{
			name: "valid title on editing tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID())
			},
			title:       "Updated Title",
			wantTitle:   "Updated Title",
			wantSuccess: true,
		},
		{
			name: "draft takes priority over editing",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID())
				tc.SetDraft(newTestTikiWithID())
			},
			title:       "Draft Title",
			wantTitle:   "Draft Title",
			wantSuccess: true,
		},
		{
			name: "no active tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				// Don't set up any tiki
			},
			title:       "Title",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tikiStore := store.NewInMemoryStore()
			gate := service.NewTikiMutationGate()
			gate.SetStore(tikiStore)
			navController := newMockNavigationController()
			tc := NewTikiEditSession(tikiStore, gate, navController, nil)

			tt.setupTiki(tc, tikiStore)

			got := tc.SaveTitle(tt.title)

			if got != tt.wantSuccess {
				t.Errorf("SaveTitle() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var activeTiki *tikipkg.Tiki
				if tc.draftTiki != nil {
					activeTiki = tc.draftTiki
				} else if tc.editingTiki != nil {
					activeTiki = tc.editingTiki
				}
				if activeTiki.Title() != tt.wantTitle {
					t.Errorf("tiki.Title = %q, want %q", activeTiki.Title(), tt.wantTitle)
				}
			}
		})
	}
}

func TestTikiEditSession_SaveDescription(t *testing.T) {
	tests := []struct {
		name            string
		setupTiki       func(*TikiEditSession, store.Store)
		description     string
		wantDescription string
		wantSuccess     bool
	}{
		{
			name: "valid description on draft tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			description:     "New description",
			wantDescription: "New description",
			wantSuccess:     true,
		},
		{
			name: "valid description on editing tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID())
			},
			description:     "Updated description",
			wantDescription: "Updated description",
			wantSuccess:     true,
		},
		{
			name: "no active tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				// Don't set up any tiki
			},
			description: "Description",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tikiStore := store.NewInMemoryStore()
			gate := service.NewTikiMutationGate()
			gate.SetStore(tikiStore)
			navController := newMockNavigationController()
			tc := NewTikiEditSession(tikiStore, gate, navController, nil)

			tt.setupTiki(tc, tikiStore)

			got := tc.SaveDescription(tt.description)

			if got != tt.wantSuccess {
				t.Errorf("SaveDescription() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var activeTiki *tikipkg.Tiki
				if tc.draftTiki != nil {
					activeTiki = tc.draftTiki
				} else if tc.editingTiki != nil {
					activeTiki = tc.editingTiki
				}
				if activeTiki.Body() != tt.wantDescription {
					t.Errorf("tiki.Description = %q, want %q", activeTiki.Body(), tt.wantDescription)
				}
			}
		})
	}
}

// Test Edit Session Management

func TestTikiEditSession_CommitEditSession_Draft(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	draft := newTestTikiWithID()
	draft.SetTitle("Draft Title")
	tc.SetDraft(draft)

	err := tc.CommitEditSession()
	if err != nil {
		t.Fatalf("CommitEditSession failed: %v", err)
	}

	// Verify draft was cleared
	if tc.GetDraftTiki() != nil {
		t.Error("CommitEditSession did not clear draft")
	}

	// Verify tiki was created in store
	created := tikiStore.GetTiki("DRAFT1")
	if created == nil {
		t.Fatal("Tiki was not created in store")
		return
	}

	if created.Title() != "Draft Title" {
		t.Errorf("Created tiki has wrong title, got %q, want %q", created.Title(), "Draft Title")
	}
}

func TestTikiEditSession_CommitEditSession_DraftValidationFailure(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.BuildGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	draft := newTestTikiWithID()
	draft.SetTitle("") // Invalid - empty title
	tc.SetDraft(draft)

	err := tc.CommitEditSession()
	if err == nil {
		t.Fatal("expected error for empty title")
	}

	// Draft should still exist since validation failed
	if tc.GetDraftTiki() == nil {
		t.Error("Draft was cleared despite validation failure")
	}
}

func TestTikiEditSession_CommitEditSession_Existing(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	// Create original tiki
	original := newTestTiki()
	_ = tikiStore.CreateTiki(original)

	// Start edit session and modify
	tc.StartEditSession(original.ID())
	tc.editingTiki.SetTitle("Modified Title")

	err := tc.CommitEditSession()
	if err != nil {
		t.Fatalf("CommitEditSession failed: %v", err)
	}

	// Verify editing tiki was cleared
	if tc.GetEditingTiki() != nil {
		t.Error("CommitEditSession did not clear editingTiki")
	}

	// Verify tiki was updated in store
	updated := tikiStore.GetTiki(original.ID())
	if updated == nil {
		t.Fatal("Tiki not found in store")
		return
	}

	if updated.Title() != "Modified Title" {
		t.Errorf("Tiki was not updated, got title %q, want %q", updated.Title(), "Modified Title")
	}
}

func TestTikiEditSession_CommitEditSession_NoActiveSession(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	err := tc.CommitEditSession()
	if err != nil {
		t.Errorf("CommitEditSession with no active session should return nil, got error: %v", err)
	}
}

// Test Helper Methods

func TestTikiEditSession_GetCurrentTiki(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	// Create tiki
	original := newTestTiki()
	_ = tikiStore.CreateTiki(original)

	// Set as current
	tc.SetCurrentTiki(original.ID())

	current := tc.GetCurrentTiki()
	if current == nil {
		t.Fatal("GetCurrentTiki returned nil")
		return
	}

	if current.ID() != original.ID() {
		t.Errorf("GetCurrentTiki returned wrong tiki, got ID %q, want %q", current.ID(), original.ID())
	}
}

func TestTikiEditSession_GetCurrentTiki_Empty(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	current := tc.GetCurrentTiki()
	if current != nil {
		t.Error("GetCurrentTiki should return nil when currentTikiID is empty")
	}
}

func TestTikiEditSession_GetCurrentTiki_NonExistent(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	tc.SetCurrentTiki("NONEXISTENT")

	current := tc.GetCurrentTiki()
	if current != nil {
		t.Error("GetCurrentTiki should return nil for non-existent tiki")
	}
}

// Test Action Registry

func TestTikiEditSession_GetActionRegistry(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	registry := tc.GetActionRegistry()
	if registry == nil {
		t.Error("GetActionRegistry returned nil")
	}

	// Verify it's the tiki detail registry (should have some actions)
	actions := registry.GetActions()
	if len(actions) == 0 {
		t.Error("Tiki detail action registry has no actions")
	}
}

func TestTikiEditSession_GetEditActionRegistry(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	registry := tc.GetEditActionRegistry()
	if registry == nil {
		t.Error("GetEditActionRegistry returned nil")
	}

	// Verify it's the edit registry (should have some actions)
	actions := registry.GetActions()
	if len(actions) == 0 {
		t.Error("Tiki edit action registry has no actions")
	}
}

// Test Focused Field

func TestTikiEditSession_FocusedField(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	// Initially should be empty
	if tc.GetFocusedField() != "" {
		t.Errorf("Initial focused field should be empty, got %v", tc.GetFocusedField())
	}

	// Set focused field
	tc.SetFocusedField(model.EditFieldTitle)
	if tc.GetFocusedField() != model.EditFieldTitle {
		t.Errorf("SetFocusedField did not set field, got %v, want %v", tc.GetFocusedField(), model.EditFieldTitle)
	}
}

func TestTikiEditSession_HandleAction(t *testing.T) {
	tests := []struct {
		name     string
		actionID ActionID
		hasTiki  bool
		want     bool
	}{
		{"edit title with tiki", ActionEditTitle, true, true},
		{"edit title without tiki", ActionEditTitle, false, false},
		{"clone tiki", ActionCloneTiki, true, true},
		{"unknown action", ActionID("unknown"), true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tikiStore := store.NewInMemoryStore()
			gate := service.NewTikiMutationGate()
			gate.SetStore(tikiStore)
			navController := newMockNavigationController()
			tc := NewTikiEditSession(tikiStore, gate, navController, nil)

			if tt.hasTiki {
				original := newTestTiki()
				_ = tikiStore.CreateTiki(original)
				tc.SetCurrentTiki(original.ID())
			}

			got := tc.HandleAction(tt.actionID)
			if got != tt.want {
				t.Errorf("HandleAction(%q) = %v, want %v", tt.actionID, got, tt.want)
			}
		})
	}
}

func TestTikiEditSession_SaveWorkflowRecurrenceDoesNotMutateDateField(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "schedule", Type: workflow.TypeRecurrence},
		{Name: "deadline", Type: workflow.TypeDate},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	tc := NewTikiEditSession(tikiStore, gate, newMockNavigationController(), nil)

	const dateField = "deadline"
	original := time.Date(2025, time.June, 15, 0, 0, 0, 0, time.UTC)
	draft := newTestTiki()
	draft.Set(dateField, original)
	tc.SetDraft(draft)

	if !tc.SaveWorkflowField("schedule", string(recurrence.RecurrenceDaily)) {
		t.Fatal("SaveWorkflowField returned false")
	}
	got, _, _ := tc.draftTiki.TimeField(dateField)
	if !got.Equal(original) {
		t.Fatalf("date field changed to %s, want %s", got, original)
	}
}

func TestTikiEditSession_UpdateTiki(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	original := newTestTiki()
	_ = tikiStore.CreateTiki(original)

	// Start an edit session, modify the title, and commit
	tiki := tc.StartEditSession(original.ID())
	tiki.SetTitle("Updated via UpdateTiki")
	if err := tc.CommitEditSession(); err != nil {
		t.Fatalf("CommitEditSession failed: %v", err)
	}

	persisted := tikiStore.GetTiki(original.ID())
	if persisted.Title() != "Updated via UpdateTiki" {
		t.Errorf("tiki not updated, got title %q", persisted.Title())
	}
}

func TestTikiEditSession_AddComment(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	statusline := model.NewStatuslineConfig()
	tc := NewTikiEditSession(tikiStore, gate, navController, statusline)

	// no current tiki — should return false
	if tc.AddComment("user", "hello") {
		t.Error("expected false when no current tiki")
	}

	original := newTestTiki()
	_ = tikiStore.CreateTiki(original)
	tc.SetCurrentTiki(original.ID())

	if !tc.AddComment("user", "hello") {
		t.Error("expected true for successful comment")
	}

	persistedTiki := tikiStore.GetTiki(original.ID())
	persistedComments, _ := persistedTiki.Fields["comments"].([]tikipkg.Comment)
	if len(persistedComments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(persistedComments))
	}
	if persistedComments[0].Text != "hello" {
		t.Errorf("comment text = %q, want %q", persistedComments[0].Text, "hello")
	}
}

func TestTikiEditSession_HandleAction_EditSource(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	// with no tiki, should return false
	got := tc.HandleAction(ActionEditSource)
	if got {
		t.Error("HandleAction(EditSource) should return false with no current tiki")
	}
}

func TestTikiEditSession_CommitEditSession_UpdateError(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.BuildGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	original := newTestTiki()
	_ = tikiStore.CreateTiki(original)
	tc.StartEditSession(original.ID())
	tc.editingTiki.SetTitle("") // invalid - empty title will fail validation

	err := tc.CommitEditSession()
	if err == nil {
		t.Fatal("expected error for empty title in edit session")
	}
}
