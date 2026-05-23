package controller

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
	"github.com/boolean-maybe/tiki/workflow/value"
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

	if tc.GetCurrentTikiID() != draft.ID {
		t.Errorf("SetDraft did not set currentTikiID, got %q, want %q", tc.GetCurrentTikiID(), draft.ID)
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
	editingTiki := tc.StartEditSession(original.ID)

	if editingTiki == nil {
		t.Fatal("StartEditSession returned nil")
		return
	}

	if editingTiki.ID != original.ID {
		t.Errorf("StartEditSession returned wrong tiki, got ID %q, want %q", editingTiki.ID, original.ID)
	}

	if tc.GetEditingTiki() == nil {
		t.Error("StartEditSession did not set editingTiki")
	}

	if tc.GetCurrentTikiID() != original.ID {
		t.Errorf("StartEditSession did not set currentTikiID, got %q, want %q", tc.GetCurrentTikiID(), original.ID)
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
	tc.StartEditSession(original.ID)

	// Cancel it
	tc.CancelEditSession()

	if tc.GetEditingTiki() != nil {
		t.Error("CancelEditSession did not clear editingTiki")
	}

	if tc.GetCurrentTikiID() != "" {
		t.Errorf("CancelEditSession did not clear currentTikiID, got %q", tc.GetCurrentTikiID())
	}
}

// Test Field Update Methods

func TestTikiEditSession_SaveStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupTiki     func(*TikiEditSession, store.Store)
		statusDisplay string
		wantStatus    string
		wantSuccess   bool
	}{
		{
			name: "valid status on draft tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			statusDisplay: enumDisplay("status", "ready"),
			wantStatus:    "ready",
			wantSuccess:   true,
		},
		{
			name: "valid status on editing tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			statusDisplay: enumDisplay("status", "inProgress"),
			wantStatus:    "inProgress",
			wantSuccess:   true,
		},
		{
			name: "draft takes priority over editing",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
				tc.SetDraft(newTestTikiWithID())
			},
			statusDisplay: enumDisplay("status", "done"),
			wantStatus:    "done",
			wantSuccess:   true,
		},
		{
			name: "invalid status normalizes to default",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			statusDisplay: "InvalidStatus",
			wantStatus:    "inbox", // NormalizeStatus defaults to inbox
			wantSuccess:   true,
		},
		{
			name: "no active tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				// Don't set up any tiki
			},
			statusDisplay: enumDisplay("status", "ready"),
			wantSuccess:   false,
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

			got := tc.SaveStatus(tt.statusDisplay)

			if got != tt.wantSuccess {
				t.Errorf("SaveStatus() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var activeTiki *tikipkg.Tiki
				if tc.draftTiki != nil {
					activeTiki = tc.draftTiki
				} else if tc.editingTiki != nil {
					activeTiki = tc.editingTiki
				}
				actualStatus, _, _ := activeTiki.StringField(tikipkg.FieldStatus)
				if actualStatus != tt.wantStatus {
					t.Errorf("status = %v, want %v", actualStatus, tt.wantStatus)
				}
			}
		})
	}
}

func TestTikiEditSession_SaveType(t *testing.T) {
	tests := []struct {
		name        string
		setupTiki   func(*TikiEditSession, store.Store)
		typeDisplay string
		wantType    string
		wantSuccess bool
	}{
		{
			name: "valid type on draft tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			typeDisplay: enumDisplay("type", "bug"),
			wantType:    "bug",
			wantSuccess: true,
		},
		{
			name: "valid type on editing tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			typeDisplay: enumDisplay("type", "spike"),
			wantType:    "spike",
			wantSuccess: true,
		},
		{
			name: "invalid type is rejected",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			typeDisplay: "InvalidType",
			wantType:    "story", // tiki type unchanged from setup
			wantSuccess: false,
		},
		{
			name: "no active tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				// Don't set up any tiki
			},
			typeDisplay: enumDisplay("type", "story"),
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

			got := tc.SaveType(tt.typeDisplay)

			if got != tt.wantSuccess {
				t.Errorf("SaveType() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var activeTiki *tikipkg.Tiki
				if tc.draftTiki != nil {
					activeTiki = tc.draftTiki
				} else if tc.editingTiki != nil {
					activeTiki = tc.editingTiki
				}
				actualType, _, _ := activeTiki.StringField(tikipkg.FieldType)
				if actualType != tt.wantType {
					t.Errorf("type = %v, want %v", actualType, tt.wantType)
				}
			}
		})
	}
}

func TestTikiEditSession_SavePriority(t *testing.T) {
	tests := []struct {
		name         string
		setupTiki    func(*TikiEditSession, store.Store)
		priority     string
		wantPriority string
		wantSuccess  bool
	}{
		{
			name: "valid priority on draft tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			priority:     "high",
			wantPriority: "high",
			wantSuccess:  true,
		},
		{
			name: "valid priority on editing tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			priority:     "low",
			wantPriority: "low",
			wantSuccess:  true,
		},
		{
			name: "invalid priority - unknown key",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			priority:    "ultra-critical",
			wantSuccess: false,
		},
		{
			name: "invalid priority - numeric leftover",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			priority:    "10",
			wantSuccess: false,
		},
		{
			name: "no active tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				// Don't set up any tiki
			},
			priority:    "medium",
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

			got := tc.SavePriority(tt.priority)

			if got != tt.wantSuccess {
				t.Errorf("SavePriority() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var activeTiki *tikipkg.Tiki
				if tc.draftTiki != nil {
					activeTiki = tc.draftTiki
				} else if tc.editingTiki != nil {
					activeTiki = tc.editingTiki
				}
				actualPriority, _, _ := activeTiki.StringField(tikipkg.FieldPriority)
				if actualPriority != tt.wantPriority {
					t.Errorf("tiki.Priority = %v, want %v", actualPriority, tt.wantPriority)
				}
			}
		})
	}
}

// TestTikiEditSession_SaveWorkflowEnum pins the path that custom workflow
// enum fields (severity, environment, etc.) take when their SemanticEnum
// editor commits a value. Without this method, custom enum edits would
// have no save handler at all and the in-flight values would be dropped.
func TestTikiEditSession_SaveWorkflowEnum(t *testing.T) {
	// Register a custom enum field for the duration of this test.
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
		{"non-enum field rejected", "points", "high", false, ""},
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

func TestTikiEditSession_SaveAssignee(t *testing.T) {
	tests := []struct {
		name         string
		setupTiki    func(*TikiEditSession, store.Store)
		assignee     string
		wantAssignee string
		wantSuccess  bool
	}{
		{
			name: "valid assignee on draft tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			assignee:     "john.doe",
			wantAssignee: "john.doe",
			wantSuccess:  true,
		},
		{
			name: "unassigned becomes empty string",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			assignee:     "Unassigned",
			wantAssignee: "",
			wantSuccess:  true,
		},
		{
			name: "valid assignee on editing tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			assignee:     "jane.smith",
			wantAssignee: "jane.smith",
			wantSuccess:  true,
		},
		{
			name: "no active tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				// Don't set up any tiki
			},
			assignee:    "john.doe",
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

			got := tc.SaveAssignee(tt.assignee)

			if got != tt.wantSuccess {
				t.Errorf("SaveAssignee() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var activeTiki *tikipkg.Tiki
				if tc.draftTiki != nil {
					activeTiki = tc.draftTiki
				} else if tc.editingTiki != nil {
					activeTiki = tc.editingTiki
				}
				actualAssignee, _, _ := activeTiki.StringField(tikipkg.FieldAssignee)
				if actualAssignee != tt.wantAssignee {
					t.Errorf("tiki.Assignee = %q, want %q", actualAssignee, tt.wantAssignee)
				}
			}
		})
	}
}

func TestTikiEditSession_SavePoints(t *testing.T) {
	tests := []struct {
		name        string
		setupTiki   func(*TikiEditSession, store.Store)
		points      int
		wantPoints  int
		wantSuccess bool
	}{
		{
			name: "valid points on draft tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			points:      7,
			wantPoints:  7,
			wantSuccess: true,
		},
		{
			name: "valid points on editing tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			points:      3,
			wantPoints:  3,
			wantSuccess: true,
		},
		{
			name: "invalid points - not in enum",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			points:      5, // not a declared enum value (declared: 11, 7, 3, 1)
			wantSuccess: false,
		},
		{
			name: "no active tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				// Don't set up any tiki
			},
			points:      7,
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

			got := tc.SavePoints(tt.points)

			if got != tt.wantSuccess {
				t.Errorf("SavePoints() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var activeTiki *tikipkg.Tiki
				if tc.draftTiki != nil {
					activeTiki = tc.draftTiki
				} else if tc.editingTiki != nil {
					activeTiki = tc.editingTiki
				}
				actualPoints, _, _ := activeTiki.StringField(tikipkg.FieldPoints)
				want := strconv.Itoa(tt.wantPoints)
				if actualPoints != want {
					t.Errorf("tiki.Points = %q, want %q", actualPoints, want)
				}
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
				tc.StartEditSession(t.ID)
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
				tc.StartEditSession(t.ID)
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
				if activeTiki.Title != tt.wantTitle {
					t.Errorf("tiki.Title = %q, want %q", activeTiki.Title, tt.wantTitle)
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
				tc.StartEditSession(t.ID)
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
				if activeTiki.Body != tt.wantDescription {
					t.Errorf("tiki.Description = %q, want %q", activeTiki.Body, tt.wantDescription)
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
	draft.Title = "Draft Title"
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

	if created.Title != "Draft Title" {
		t.Errorf("Created tiki has wrong title, got %q, want %q", created.Title, "Draft Title")
	}
}

func TestTikiEditSession_CommitEditSession_DraftValidationFailure(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.BuildGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	draft := newTestTikiWithID()
	draft.Title = "" // Invalid - empty title
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
	tc.StartEditSession(original.ID)
	tc.editingTiki.Title = "Modified Title"

	err := tc.CommitEditSession()
	if err != nil {
		t.Fatalf("CommitEditSession failed: %v", err)
	}

	// Verify editing tiki was cleared
	if tc.GetEditingTiki() != nil {
		t.Error("CommitEditSession did not clear editingTiki")
	}

	// Verify tiki was updated in store
	updated := tikiStore.GetTiki(original.ID)
	if updated == nil {
		t.Fatal("Tiki not found in store")
		return
	}

	if updated.Title != "Modified Title" {
		t.Errorf("Tiki was not updated, got title %q, want %q", updated.Title, "Modified Title")
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
	tc.SetCurrentTiki(original.ID)

	current := tc.GetCurrentTiki()
	if current == nil {
		t.Fatal("GetCurrentTiki returned nil")
		return
	}

	if current.ID != original.ID {
		t.Errorf("GetCurrentTiki returned wrong tiki, got ID %q, want %q", current.ID, original.ID)
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

func TestTikiEditSession_SaveDue(t *testing.T) {
	tests := []struct {
		name        string
		setupTiki   func(*TikiEditSession, store.Store)
		dateStr     string
		wantDue     string // expected Format(DateFormat) or "" for zero
		wantSuccess bool
	}{
		{
			name: "valid date on draft tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			dateStr:     "2025-06-15",
			wantDue:     "2025-06-15",
			wantSuccess: true,
		},
		{
			name: "valid date on editing tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			dateStr:     "2025-12-31",
			wantDue:     "2025-12-31",
			wantSuccess: true,
		},
		{
			name: "empty string clears due date",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				draft := newTestTiki()
				draft.Set(tikipkg.FieldDue, time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC))
				tc.SetDraft(draft)
			},
			dateStr:     "",
			wantDue:     "",
			wantSuccess: true,
		},
		{
			name: "invalid date returns false",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			dateStr:     "not-a-date",
			wantSuccess: false,
		},
		{
			name: "no active tiki returns false",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				// no tiki set up
			},
			dateStr:     "2025-06-15",
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

			got := tc.SaveDue(tt.dateStr)

			if got != tt.wantSuccess {
				t.Errorf("SaveDue() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var activeTiki *tikipkg.Tiki
				if tc.draftTiki != nil {
					activeTiki = tc.draftTiki
				} else if tc.editingTiki != nil {
					activeTiki = tc.editingTiki
				}
				actualDue, _, _ := activeTiki.TimeField(tikipkg.FieldDue)

				var actualStr string
				if !actualDue.IsZero() {
					actualStr = actualDue.Format(value.DateFormat)
				}
				if actualStr != tt.wantDue {
					t.Errorf("tiki.Due = %q, want %q", actualStr, tt.wantDue)
				}
			}
		})
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
				tc.SetCurrentTiki(original.ID)
			}

			got := tc.HandleAction(tt.actionID)
			if got != tt.want {
				t.Errorf("HandleAction(%q) = %v, want %v", tt.actionID, got, tt.want)
			}
		})
	}
}

func TestTikiEditSession_SaveRecurrence(t *testing.T) {
	tests := []struct {
		name           string
		setupTiki      func(*TikiEditSession, store.Store)
		cron           string
		wantRecurrence value.Recurrence
		wantDueSet     bool
		wantSuccess    bool
	}{
		{
			name: "valid daily recurrence on draft",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			cron:           string(value.RecurrenceDaily),
			wantRecurrence: value.RecurrenceDaily,
			wantDueSet:     true,
			wantSuccess:    true,
		},
		{
			name: "clear recurrence sets none and clears due",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				draft := newTestTiki()
				draft.Set(tikipkg.FieldDue, time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC))
				draft.Set(tikipkg.FieldRecurrence, string(value.RecurrenceDaily))
				tc.SetDraft(draft)
			},
			cron:           string(value.RecurrenceNone),
			wantRecurrence: value.RecurrenceNone,
			wantDueSet:     false,
			wantSuccess:    true,
		},
		{
			name: "invalid recurrence rejected",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			cron:        "invalid-cron",
			wantSuccess: false,
		},
		{
			name: "no active tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				// no tiki
			},
			cron:        string(value.RecurrenceDaily),
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

			got := tc.SaveRecurrence(tt.cron)
			if got != tt.wantSuccess {
				t.Errorf("SaveRecurrence() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess && tc.draftTiki != nil {
				recurrenceStr, _, _ := tc.draftTiki.StringField(tikipkg.FieldRecurrence)
				actualRecurrence := value.Recurrence(recurrenceStr)
				if actualRecurrence != tt.wantRecurrence {
					t.Errorf("Recurrence = %q, want %q", actualRecurrence, tt.wantRecurrence)
				}
				dueTime, _, _ := tc.draftTiki.TimeField(tikipkg.FieldDue)
				if tt.wantDueSet && dueTime.IsZero() {
					t.Error("expected Due to be set for non-none recurrence")
				}
				if !tt.wantDueSet && !dueTime.IsZero() {
					t.Error("expected Due to be zero for none recurrence")
				}
			}
		})
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
	tiki := tc.StartEditSession(original.ID)
	tiki.Title = "Updated via UpdateTiki"
	if err := tc.CommitEditSession(); err != nil {
		t.Fatalf("CommitEditSession failed: %v", err)
	}

	persisted := tikiStore.GetTiki(original.ID)
	if persisted.Title != "Updated via UpdateTiki" {
		t.Errorf("tiki not updated, got title %q", persisted.Title)
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
	tc.SetCurrentTiki(original.ID)

	if !tc.AddComment("user", "hello") {
		t.Error("expected true for successful comment")
	}

	persistedTiki := tikiStore.GetTiki(original.ID)
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
	tc.StartEditSession(original.ID)
	tc.editingTiki.Title = "" // invalid - empty title will fail validation

	err := tc.CommitEditSession()
	if err == nil {
		t.Fatal("expected error for empty title in edit session")
	}
}

func TestTikiEditSession_SaveType_InvalidType(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	navController := newMockNavigationController()
	tc := NewTikiEditSession(tikiStore, gate, navController, nil)

	original := newTestTiki()
	_ = tikiStore.CreateTiki(original)
	tc.StartEditSession(original.ID)

	// unrecognized display string
	got := tc.SaveType("Nonexistent Type 🤷")
	if got {
		t.Error("SaveType should return false for unrecognized type display")
	}
}

func TestTikiEditSession_SaveTags(t *testing.T) {
	tests := []struct {
		name        string
		setupTiki   func(*TikiEditSession, store.Store)
		tags        []string
		wantTags    []string
		wantSuccess bool
	}{
		{
			name: "valid tags on draft tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			tags:        []string{"api", "backend"},
			wantTags:    []string{"api", "backend"},
			wantSuccess: true,
		},
		{
			name: "valid tags on editing tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			tags:        []string{"frontend", "ui"},
			wantTags:    []string{"frontend", "ui"},
			wantSuccess: true,
		},
		{
			name: "duplicate tags deduped",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			tags:        []string{"frontend", " frontend ", "frontend", "backend"},
			wantTags:    []string{"frontend", "backend"},
			wantSuccess: true,
		},
		{
			name: "empty tags slice",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			tags:        []string{},
			wantTags:    []string{},
			wantSuccess: true,
		},
		{
			name: "nil tags",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			tags:        nil,
			wantTags:    []string{},
			wantSuccess: true,
		},
		{
			name: "draft takes priority over editing",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				t := newTestTiki()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
				tc.SetDraft(newTestTikiWithID())
			},
			tags:        []string{"draft-tag"},
			wantTags:    []string{"draft-tag"},
			wantSuccess: true,
		},
		{
			name: "no active tiki",
			setupTiki: func(tc *TikiEditSession, s store.Store) {
				// no tiki set up
			},
			tags:        []string{"orphan"},
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

			got := tc.SaveTags(tt.tags)

			if got != tt.wantSuccess {
				t.Errorf("SaveTags() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var activeTiki *tikipkg.Tiki
				if tc.draftTiki != nil {
					activeTiki = tc.draftTiki
				} else if tc.editingTiki != nil {
					activeTiki = tc.editingTiki
				}
				actualTags, _, _ := activeTiki.StringSliceField(tikipkg.FieldTags)
				if actualTags == nil {
					actualTags = []string{}
				}
				if !slices.Equal(actualTags, tt.wantTags) {
					t.Errorf("tiki.Tags = %v, want %v", actualTags, tt.wantTags)
				}
			}
		})
	}
}
