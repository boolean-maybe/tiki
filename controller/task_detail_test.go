package controller

import (
	"slices"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

func init() {
	// set up the default status registry for tests.
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "inProgress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "review", Label: "Review", Emoji: "👀", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	})
}

// Test Draft Task Lifecycle

func TestTaskController_SetDraft(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	draft := newTestTiki()
	tc.SetDraft(draft)

	if tc.GetDraftTiki() == nil {
		t.Error("SetDraft did not set the draft task")
	}

	if tc.GetCurrentTaskID() != draft.ID {
		t.Errorf("SetDraft did not set currentTaskID, got %q, want %q", tc.GetCurrentTaskID(), draft.ID)
	}
}

func TestTaskController_ClearDraft(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	tc.SetDraft(newTestTiki())
	tc.ClearDraft()

	if tc.GetDraftTiki() != nil {
		t.Error("ClearDraft did not clear the draft task")
	}
}

func TestTaskController_StartEditSession(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	// Create a task in the store
	original := newTestTask()
	original.LoadedMtime = time.Now()
	_ = taskStore.CreateTiki(original)

	// Start edit session
	editingTask := tc.StartEditSession(original.ID)

	if editingTask == nil {
		t.Fatal("StartEditSession returned nil")
		return
	}

	if editingTask.ID != original.ID {
		t.Errorf("StartEditSession returned wrong task, got ID %q, want %q", editingTask.ID, original.ID)
	}

	if tc.GetEditingTiki() == nil {
		t.Error("StartEditSession did not set editingTask")
	}

	if tc.GetCurrentTaskID() != original.ID {
		t.Errorf("StartEditSession did not set currentTaskID, got %q, want %q", tc.GetCurrentTaskID(), original.ID)
	}
}

func TestTaskController_StartEditSession_NonExistent(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	editingTask := tc.StartEditSession("NONEXISTENT")

	if editingTask != nil {
		t.Error("StartEditSession should return nil for non-existent task")
	}
}

func TestTaskController_CancelEditSession(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	// Start an edit session
	original := newTestTask()
	_ = taskStore.CreateTiki(original)
	tc.StartEditSession(original.ID)

	// Cancel it
	tc.CancelEditSession()

	if tc.GetEditingTiki() != nil {
		t.Error("CancelEditSession did not clear editingTask")
	}

	if tc.GetCurrentTaskID() != "" {
		t.Errorf("CancelEditSession did not clear currentTaskID, got %q", tc.GetCurrentTaskID())
	}
}

// Test Field Update Methods

func TestTaskController_SaveStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupTask     func(*TaskController, store.Store)
		statusDisplay string
		wantStatus    task.Status
		wantSuccess   bool
	}{
		{
			name: "valid status on draft task",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			statusDisplay: task.StatusDisplay(task.StatusReady),
			wantStatus:    task.StatusReady,
			wantSuccess:   true,
		},
		{
			name: "valid status on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			statusDisplay: task.StatusDisplay(task.StatusInProgress),
			wantStatus:    task.StatusInProgress,
			wantSuccess:   true,
		},
		{
			name: "draft takes priority over editing",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
				tc.SetDraft(newTestTikiWithID())
			},
			statusDisplay: task.StatusDisplay(task.StatusDone),
			wantStatus:    task.StatusDone,
			wantSuccess:   true,
		},
		{
			name: "invalid status normalizes to default",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			statusDisplay: "InvalidStatus",
			wantStatus:    task.StatusBacklog, // NormalizeStatus defaults to backlog
			wantSuccess:   true,
		},
		{
			name: "no active task",
			setupTask: func(tc *TaskController, s store.Store) {
				// Don't set up any task
			},
			statusDisplay: task.StatusDisplay(task.StatusReady),
			wantSuccess:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			tt.setupTask(tc, taskStore)

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
				statusStr, _, _ := activeTiki.StringField(tikipkg.FieldStatus)
				actualStatus := task.Status(statusStr)
				if actualStatus != tt.wantStatus {
					t.Errorf("task.Status = %v, want %v", actualStatus, tt.wantStatus)
				}
			}
		})
	}
}

func TestTaskController_SaveType(t *testing.T) {
	tests := []struct {
		name        string
		setupTask   func(*TaskController, store.Store)
		typeDisplay string
		wantType    task.Type
		wantSuccess bool
	}{
		{
			name: "valid type on draft task",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			typeDisplay: task.TypeDisplay(task.TypeBug),
			wantType:    task.TypeBug,
			wantSuccess: true,
		},
		{
			name: "valid type on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			typeDisplay: task.TypeDisplay(task.TypeSpike),
			wantType:    task.TypeSpike,
			wantSuccess: true,
		},
		{
			name: "invalid type is rejected",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			typeDisplay: "InvalidType",
			wantType:    task.TypeStory, // task type unchanged from setup
			wantSuccess: false,
		},
		{
			name: "no active task",
			setupTask: func(tc *TaskController, s store.Store) {
				// Don't set up any task
			},
			typeDisplay: task.TypeDisplay(task.TypeStory),
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			tt.setupTask(tc, taskStore)

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
				typeStr, _, _ := activeTiki.StringField(tikipkg.FieldType)
				actualType := task.Type(typeStr)
				if actualType != tt.wantType {
					t.Errorf("task.Type = %v, want %v", actualType, tt.wantType)
				}
			}
		})
	}
}

func TestTaskController_SavePriority(t *testing.T) {
	tests := []struct {
		name         string
		setupTask    func(*TaskController, store.Store)
		priority     int
		wantPriority int
		wantSuccess  bool
	}{
		{
			name: "valid priority on draft task",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			priority:     1,
			wantPriority: 1,
			wantSuccess:  true,
		},
		{
			name: "valid priority on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			priority:     5,
			wantPriority: 5,
			wantSuccess:  true,
		},
		{
			name: "invalid priority - negative",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			priority:    -1,
			wantSuccess: false,
		},
		{
			name: "invalid priority - too high",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			priority:    10,
			wantSuccess: false,
		},
		{
			name: "no active task",
			setupTask: func(tc *TaskController, s store.Store) {
				// Don't set up any task
			},
			priority:    3,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			tt.setupTask(tc, taskStore)

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
				actualPriority, _, _ := activeTiki.IntField(tikipkg.FieldPriority)
				if actualPriority != tt.wantPriority {
					t.Errorf("task.Priority = %v, want %v", actualPriority, tt.wantPriority)
				}
			}
		})
	}
}

func TestTaskController_SaveAssignee(t *testing.T) {
	tests := []struct {
		name         string
		setupTask    func(*TaskController, store.Store)
		assignee     string
		wantAssignee string
		wantSuccess  bool
	}{
		{
			name: "valid assignee on draft task",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			assignee:     "john.doe",
			wantAssignee: "john.doe",
			wantSuccess:  true,
		},
		{
			name: "unassigned becomes empty string",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			assignee:     "Unassigned",
			wantAssignee: "",
			wantSuccess:  true,
		},
		{
			name: "valid assignee on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			assignee:     "jane.smith",
			wantAssignee: "jane.smith",
			wantSuccess:  true,
		},
		{
			name: "no active task",
			setupTask: func(tc *TaskController, s store.Store) {
				// Don't set up any task
			},
			assignee:    "john.doe",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			tt.setupTask(tc, taskStore)

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
					t.Errorf("task.Assignee = %q, want %q", actualAssignee, tt.wantAssignee)
				}
			}
		})
	}
}

func TestTaskController_SavePoints(t *testing.T) {
	tests := []struct {
		name        string
		setupTask   func(*TaskController, store.Store)
		points      int
		wantPoints  int
		wantSuccess bool
	}{
		{
			name: "valid points on draft task",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			points:      8,
			wantPoints:  8,
			wantSuccess: true,
		},
		{
			name: "valid points on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			points:      3,
			wantPoints:  3,
			wantSuccess: true,
		},
		{
			name: "invalid points - negative",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			points:      -1,
			wantSuccess: false,
		},
		{
			name: "no active task",
			setupTask: func(tc *TaskController, s store.Store) {
				// Don't set up any task
			},
			points:      5,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			tt.setupTask(tc, taskStore)

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
				actualPoints, _, _ := activeTiki.IntField(tikipkg.FieldPoints)
				if actualPoints != tt.wantPoints {
					t.Errorf("task.Points = %v, want %v", actualPoints, tt.wantPoints)
				}
			}
		})
	}
}

func TestTaskController_SaveTitle(t *testing.T) {
	tests := []struct {
		name        string
		setupTask   func(*TaskController, store.Store)
		title       string
		wantTitle   string
		wantSuccess bool
	}{
		{
			name: "valid title on draft task",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			title:       "New Title",
			wantTitle:   "New Title",
			wantSuccess: true,
		},
		{
			name: "valid title on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			title:       "Updated Title",
			wantTitle:   "Updated Title",
			wantSuccess: true,
		},
		{
			name: "draft takes priority over editing",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
				tc.SetDraft(newTestTikiWithID())
			},
			title:       "Draft Title",
			wantTitle:   "Draft Title",
			wantSuccess: true,
		},
		{
			name: "no active task",
			setupTask: func(tc *TaskController, s store.Store) {
				// Don't set up any task
			},
			title:       "Title",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			tt.setupTask(tc, taskStore)

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
					t.Errorf("task.Title = %q, want %q", activeTiki.Title, tt.wantTitle)
				}
			}
		})
	}
}

func TestTaskController_SaveDescription(t *testing.T) {
	tests := []struct {
		name            string
		setupTask       func(*TaskController, store.Store)
		description     string
		wantDescription string
		wantSuccess     bool
	}{
		{
			name: "valid description on draft task",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			description:     "New description",
			wantDescription: "New description",
			wantSuccess:     true,
		},
		{
			name: "valid description on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			description:     "Updated description",
			wantDescription: "Updated description",
			wantSuccess:     true,
		},
		{
			name: "no active task",
			setupTask: func(tc *TaskController, s store.Store) {
				// Don't set up any task
			},
			description: "Description",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			tt.setupTask(tc, taskStore)

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
					t.Errorf("task.Description = %q, want %q", activeTiki.Body, tt.wantDescription)
				}
			}
		})
	}
}

// Test Edit Session Management

func TestTaskController_CommitEditSession_Draft(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

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

	// Verify task was created in store
	created := taskStore.GetTiki("DRAFT1")
	if created == nil {
		t.Fatal("Task was not created in store")
		return
	}

	if created.Title != "Draft Title" {
		t.Errorf("Created task has wrong title, got %q, want %q", created.Title, "Draft Title")
	}
}

func TestTaskController_CommitEditSession_DraftValidationFailure(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.BuildGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

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

func TestTaskController_CommitEditSession_Existing(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	// Create original task
	original := newTestTask()
	_ = taskStore.CreateTiki(original)

	// Start edit session and modify
	tc.StartEditSession(original.ID)
	tc.editingTiki.Title = "Modified Title"

	err := tc.CommitEditSession()
	if err != nil {
		t.Fatalf("CommitEditSession failed: %v", err)
	}

	// Verify editing task was cleared
	if tc.GetEditingTiki() != nil {
		t.Error("CommitEditSession did not clear editingTask")
	}

	// Verify task was updated in store
	updated := taskStore.GetTiki(original.ID)
	if updated == nil {
		t.Fatal("Task not found in store")
		return
	}

	if updated.Title != "Modified Title" {
		t.Errorf("Task was not updated, got title %q, want %q", updated.Title, "Modified Title")
	}
}

func TestTaskController_CommitEditSession_NoActiveSession(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	err := tc.CommitEditSession()
	if err != nil {
		t.Errorf("CommitEditSession with no active session should return nil, got error: %v", err)
	}
}

// Test Helper Methods

func TestTaskController_GetCurrentTask(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	// Create task
	original := newTestTask()
	_ = taskStore.CreateTiki(original)

	// Set as current
	tc.SetCurrentTask(original.ID)

	current := tc.GetCurrentTiki()
	if current == nil {
		t.Fatal("GetCurrentTask returned nil")
		return
	}

	if current.ID != original.ID {
		t.Errorf("GetCurrentTask returned wrong task, got ID %q, want %q", current.ID, original.ID)
	}
}

func TestTaskController_GetCurrentTask_Empty(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	current := tc.GetCurrentTiki()
	if current != nil {
		t.Error("GetCurrentTask should return nil when currentTaskID is empty")
	}
}

func TestTaskController_GetCurrentTask_NonExistent(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	tc.SetCurrentTask("NONEXISTENT")

	current := tc.GetCurrentTiki()
	if current != nil {
		t.Error("GetCurrentTask should return nil for non-existent task")
	}
}

// Test Action Registry

func TestTaskController_GetActionRegistry(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	registry := tc.GetActionRegistry()
	if registry == nil {
		t.Error("GetActionRegistry returned nil")
	}

	// Verify it's the task detail registry (should have some actions)
	actions := registry.GetActions()
	if len(actions) == 0 {
		t.Error("Task detail action registry has no actions")
	}
}

func TestTaskController_GetEditActionRegistry(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	registry := tc.GetEditActionRegistry()
	if registry == nil {
		t.Error("GetEditActionRegistry returned nil")
	}

	// Verify it's the edit registry (should have some actions)
	actions := registry.GetActions()
	if len(actions) == 0 {
		t.Error("Task edit action registry has no actions")
	}
}

// Test Focused Field

func TestTaskController_FocusedField(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

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

func TestTaskController_SaveDue(t *testing.T) {
	tests := []struct {
		name        string
		setupTask   func(*TaskController, store.Store)
		dateStr     string
		wantDue     string // expected Format(DateFormat) or "" for zero
		wantSuccess bool
	}{
		{
			name: "valid date on draft task",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			dateStr:     "2025-06-15",
			wantDue:     "2025-06-15",
			wantSuccess: true,
		},
		{
			name: "valid date on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			dateStr:     "2025-12-31",
			wantDue:     "2025-12-31",
			wantSuccess: true,
		},
		{
			name: "empty string clears due date",
			setupTask: func(tc *TaskController, s store.Store) {
				draft := newTestTask()
				draft.Set(tikipkg.FieldDue, time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC))
				tc.SetDraft(draft)
			},
			dateStr:     "",
			wantDue:     "",
			wantSuccess: true,
		},
		{
			name: "invalid date returns false",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			dateStr:     "not-a-date",
			wantSuccess: false,
		},
		{
			name: "no active task returns false",
			setupTask: func(tc *TaskController, s store.Store) {
				// no task set up
			},
			dateStr:     "2025-06-15",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			tt.setupTask(tc, taskStore)

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
					actualStr = actualDue.Format(task.DateFormat)
				}
				if actualStr != tt.wantDue {
					t.Errorf("task.Due = %q, want %q", actualStr, tt.wantDue)
				}
			}
		})
	}
}

func TestTaskController_HandleAction(t *testing.T) {
	tests := []struct {
		name     string
		actionID ActionID
		hasTask  bool
		want     bool
	}{
		{"edit title with task", ActionEditTitle, true, true},
		{"edit title without task", ActionEditTitle, false, false},
		{"clone task", ActionCloneTask, true, true},
		{"unknown action", ActionID("unknown"), true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			if tt.hasTask {
				original := newTestTask()
				_ = taskStore.CreateTiki(original)
				tc.SetCurrentTask(original.ID)
			}

			got := tc.HandleAction(tt.actionID)
			if got != tt.want {
				t.Errorf("HandleAction(%q) = %v, want %v", tt.actionID, got, tt.want)
			}
		})
	}
}

func TestTaskController_SaveRecurrence(t *testing.T) {
	tests := []struct {
		name           string
		setupTask      func(*TaskController, store.Store)
		cron           string
		wantRecurrence task.Recurrence
		wantDueSet     bool
		wantSuccess    bool
	}{
		{
			name: "valid daily recurrence on draft",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			cron:           string(task.RecurrenceDaily),
			wantRecurrence: task.RecurrenceDaily,
			wantDueSet:     true,
			wantSuccess:    true,
		},
		{
			name: "clear recurrence sets none and clears due",
			setupTask: func(tc *TaskController, s store.Store) {
				draft := newTestTask()
				draft.Set(tikipkg.FieldDue, time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC))
				draft.Set(tikipkg.FieldRecurrence, string(task.RecurrenceDaily))
				tc.SetDraft(draft)
			},
			cron:           string(task.RecurrenceNone),
			wantRecurrence: task.RecurrenceNone,
			wantDueSet:     false,
			wantSuccess:    true,
		},
		{
			name: "invalid recurrence rejected",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			cron:        "invalid-cron",
			wantSuccess: false,
		},
		{
			name: "no active task",
			setupTask: func(tc *TaskController, s store.Store) {
				// no task
			},
			cron:        string(task.RecurrenceDaily),
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			tt.setupTask(tc, taskStore)

			got := tc.SaveRecurrence(tt.cron)
			if got != tt.wantSuccess {
				t.Errorf("SaveRecurrence() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess && tc.draftTiki != nil {
				recurrenceStr, _, _ := tc.draftTiki.StringField(tikipkg.FieldRecurrence)
				actualRecurrence := task.Recurrence(recurrenceStr)
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

func TestTaskController_UpdateTiki(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	original := newTestTask()
	_ = taskStore.CreateTiki(original)

	// Start an edit session, modify the title, and commit
	tiki := tc.StartEditSession(original.ID)
	tiki.Title = "Updated via UpdateTiki"
	if err := tc.CommitEditSession(); err != nil {
		t.Fatalf("CommitEditSession failed: %v", err)
	}

	persisted := taskStore.GetTiki(original.ID)
	if persisted.Title != "Updated via UpdateTiki" {
		t.Errorf("tiki not updated, got title %q", persisted.Title)
	}
}

func TestTaskController_AddComment(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	statusline := model.NewStatuslineConfig()
	tc := NewTaskController(taskStore, gate, navController, statusline)

	// no current task — should return false
	if tc.AddComment("user", "hello") {
		t.Error("expected false when no current task")
	}

	original := newTestTask()
	_ = taskStore.CreateTiki(original)
	tc.SetCurrentTask(original.ID)

	if !tc.AddComment("user", "hello") {
		t.Error("expected true for successful comment")
	}

	persistedTiki := taskStore.GetTiki(original.ID)
	persistedComments, _ := persistedTiki.Fields["comments"].([]task.Comment)
	if len(persistedComments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(persistedComments))
	}
	if persistedComments[0].Text != "hello" {
		t.Errorf("comment text = %q, want %q", persistedComments[0].Text, "hello")
	}
}

func TestTaskController_HandleAction_EditSource(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	// with no task, should return false
	got := tc.HandleAction(ActionEditSource)
	if got {
		t.Error("HandleAction(EditSource) should return false with no current task")
	}
}

func TestTaskController_CommitEditSession_UpdateError(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.BuildGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	original := newTestTask()
	_ = taskStore.CreateTiki(original)
	tc.StartEditSession(original.ID)
	tc.editingTiki.Title = "" // invalid - empty title will fail validation

	err := tc.CommitEditSession()
	if err == nil {
		t.Fatal("expected error for empty title in edit session")
	}
}

func TestTaskController_SaveType_InvalidType(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	gate := service.NewTaskMutationGate()
	gate.SetStore(taskStore)
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, gate, navController, nil)

	original := newTestTask()
	_ = taskStore.CreateTiki(original)
	tc.StartEditSession(original.ID)

	// unrecognized display string
	got := tc.SaveType("Nonexistent Type 🤷")
	if got {
		t.Error("SaveType should return false for unrecognized type display")
	}
}

func TestTaskController_SaveTags(t *testing.T) {
	tests := []struct {
		name        string
		setupTask   func(*TaskController, store.Store)
		tags        []string
		wantTags    []string
		wantSuccess bool
	}{
		{
			name: "valid tags on draft task",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			tags:        []string{"api", "backend"},
			wantTags:    []string{"api", "backend"},
			wantSuccess: true,
		},
		{
			name: "valid tags on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
			},
			tags:        []string{"frontend", "ui"},
			wantTags:    []string{"frontend", "ui"},
			wantSuccess: true,
		},
		{
			name: "duplicate tags deduped",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			tags:        []string{"frontend", " frontend ", "frontend", "backend"},
			wantTags:    []string{"frontend", "backend"},
			wantSuccess: true,
		},
		{
			name: "empty tags slice",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			tags:        []string{},
			wantTags:    []string{},
			wantSuccess: true,
		},
		{
			name: "nil tags",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTiki())
			},
			tags:        nil,
			wantTags:    []string{},
			wantSuccess: true,
		},
		{
			name: "draft takes priority over editing",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTiki(t)
				tc.StartEditSession(t.ID)
				tc.SetDraft(newTestTikiWithID())
			},
			tags:        []string{"draft-tag"},
			wantTags:    []string{"draft-tag"},
			wantSuccess: true,
		},
		{
			name: "no active task",
			setupTask: func(tc *TaskController, s store.Store) {
				// no task set up
			},
			tags:        []string{"orphan"},
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			gate := service.NewTaskMutationGate()
			gate.SetStore(taskStore)
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, gate, navController, nil)

			tt.setupTask(tc, taskStore)

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
					t.Errorf("task.Tags = %v, want %v", actualTags, tt.wantTags)
				}
			}
		})
	}
}
