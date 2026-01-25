package controller

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// Test Draft Task Lifecycle

func TestTaskController_SetDraft(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	draft := newTestTask()
	tc.SetDraft(draft)

	if tc.GetDraftTask() != draft {
		t.Error("SetDraft did not set the draft task")
	}

	if tc.GetCurrentTaskID() != draft.ID {
		t.Errorf("SetDraft did not set currentTaskID, got %q, want %q", tc.GetCurrentTaskID(), draft.ID)
	}
}

func TestTaskController_ClearDraft(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	tc.SetDraft(newTestTask())
	tc.ClearDraft()

	if tc.GetDraftTask() != nil {
		t.Error("ClearDraft did not clear the draft task")
	}
}

func TestTaskController_StartEditSession(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	// Create a task in the store
	original := newTestTask()
	original.LoadedMtime = time.Now()
	_ = taskStore.CreateTask(original)

	// Start edit session
	editingTask := tc.StartEditSession(original.ID)

	if editingTask == nil {
		t.Fatal("StartEditSession returned nil")
	}

	if editingTask.ID != original.ID {
		t.Errorf("StartEditSession returned wrong task, got ID %q, want %q", editingTask.ID, original.ID)
	}

	if tc.GetEditingTask() == nil {
		t.Error("StartEditSession did not set editingTask")
	}

	if tc.GetCurrentTaskID() != original.ID {
		t.Errorf("StartEditSession did not set currentTaskID, got %q, want %q", tc.GetCurrentTaskID(), original.ID)
	}
}

func TestTaskController_StartEditSession_NonExistent(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	editingTask := tc.StartEditSession("NONEXISTENT")

	if editingTask != nil {
		t.Error("StartEditSession should return nil for non-existent task")
	}
}

func TestTaskController_CancelEditSession(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	// Start an edit session
	original := newTestTask()
	_ = taskStore.CreateTask(original)
	tc.StartEditSession(original.ID)

	// Cancel it
	tc.CancelEditSession()

	if tc.GetEditingTask() != nil {
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
				tc.SetDraft(newTestTask())
			},
			statusDisplay: "Todo",
			wantStatus:    task.StatusReady,
			wantSuccess:   true,
		},
		{
			name: "valid status on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTask(t)
				tc.StartEditSession(t.ID)
			},
			statusDisplay: "In Progress",
			wantStatus:    task.StatusInProgress,
			wantSuccess:   true,
		},
		{
			name: "draft takes priority over editing",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTask(t)
				tc.StartEditSession(t.ID)
				tc.SetDraft(newTestTaskWithID())
			},
			statusDisplay: "Done",
			wantStatus:    task.StatusDone,
			wantSuccess:   true,
		},
		{
			name: "invalid status normalizes to default",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTask())
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
			statusDisplay: "Todo",
			wantSuccess:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, navController)

			tt.setupTask(tc, taskStore)

			got := tc.SaveStatus(tt.statusDisplay)

			if got != tt.wantSuccess {
				t.Errorf("SaveStatus() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var actualStatus task.Status
				if tc.draftTask != nil {
					actualStatus = tc.draftTask.Status
				} else if tc.editingTask != nil {
					actualStatus = tc.editingTask.Status
				}
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
				tc.SetDraft(newTestTask())
			},
			typeDisplay: "Bug",
			wantType:    task.TypeBug,
			wantSuccess: true,
		},
		{
			name: "valid type on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTask(t)
				tc.StartEditSession(t.ID)
			},
			typeDisplay: "Spike",
			wantType:    task.TypeSpike,
			wantSuccess: true,
		},
		{
			name: "invalid type normalizes to default",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTask())
			},
			typeDisplay: "InvalidType",
			wantType:    task.TypeStory, // NormalizeType defaults to story
			wantSuccess: true,
		},
		{
			name: "no active task",
			setupTask: func(tc *TaskController, s store.Store) {
				// Don't set up any task
			},
			typeDisplay: "Story",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskStore := store.NewInMemoryStore()
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, navController)

			tt.setupTask(tc, taskStore)

			got := tc.SaveType(tt.typeDisplay)

			if got != tt.wantSuccess {
				t.Errorf("SaveType() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var actualType task.Type
				if tc.draftTask != nil {
					actualType = tc.draftTask.Type
				} else if tc.editingTask != nil {
					actualType = tc.editingTask.Type
				}
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
				tc.SetDraft(newTestTask())
			},
			priority:     1,
			wantPriority: 1,
			wantSuccess:  true,
		},
		{
			name: "valid priority on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTask(t)
				tc.StartEditSession(t.ID)
			},
			priority:     5,
			wantPriority: 5,
			wantSuccess:  true,
		},
		{
			name: "invalid priority - negative",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTask())
			},
			priority:    -1,
			wantSuccess: false,
		},
		{
			name: "invalid priority - too high",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTask())
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
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, navController)

			tt.setupTask(tc, taskStore)

			got := tc.SavePriority(tt.priority)

			if got != tt.wantSuccess {
				t.Errorf("SavePriority() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var actualPriority int
				if tc.draftTask != nil {
					actualPriority = tc.draftTask.Priority
				} else if tc.editingTask != nil {
					actualPriority = tc.editingTask.Priority
				}
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
				tc.SetDraft(newTestTask())
			},
			assignee:     "john.doe",
			wantAssignee: "john.doe",
			wantSuccess:  true,
		},
		{
			name: "unassigned becomes empty string",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTask())
			},
			assignee:     "Unassigned",
			wantAssignee: "",
			wantSuccess:  true,
		},
		{
			name: "valid assignee on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTask(t)
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
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, navController)

			tt.setupTask(tc, taskStore)

			got := tc.SaveAssignee(tt.assignee)

			if got != tt.wantSuccess {
				t.Errorf("SaveAssignee() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var actualAssignee string
				if tc.draftTask != nil {
					actualAssignee = tc.draftTask.Assignee
				} else if tc.editingTask != nil {
					actualAssignee = tc.editingTask.Assignee
				}
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
				tc.SetDraft(newTestTask())
			},
			points:      8,
			wantPoints:  8,
			wantSuccess: true,
		},
		{
			name: "valid points on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTask(t)
				tc.StartEditSession(t.ID)
			},
			points:      3,
			wantPoints:  3,
			wantSuccess: true,
		},
		{
			name: "invalid points - negative",
			setupTask: func(tc *TaskController, s store.Store) {
				tc.SetDraft(newTestTask())
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
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, navController)

			tt.setupTask(tc, taskStore)

			got := tc.SavePoints(tt.points)

			if got != tt.wantSuccess {
				t.Errorf("SavePoints() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var actualPoints int
				if tc.draftTask != nil {
					actualPoints = tc.draftTask.Points
				} else if tc.editingTask != nil {
					actualPoints = tc.editingTask.Points
				}
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
				tc.SetDraft(newTestTask())
			},
			title:       "New Title",
			wantTitle:   "New Title",
			wantSuccess: true,
		},
		{
			name: "valid title on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTask(t)
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
				_ = s.CreateTask(t)
				tc.StartEditSession(t.ID)
				tc.SetDraft(newTestTaskWithID())
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
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, navController)

			tt.setupTask(tc, taskStore)

			got := tc.SaveTitle(tt.title)

			if got != tt.wantSuccess {
				t.Errorf("SaveTitle() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var actualTitle string
				if tc.draftTask != nil {
					actualTitle = tc.draftTask.Title
				} else if tc.editingTask != nil {
					actualTitle = tc.editingTask.Title
				}
				if actualTitle != tt.wantTitle {
					t.Errorf("task.Title = %q, want %q", actualTitle, tt.wantTitle)
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
				tc.SetDraft(newTestTask())
			},
			description:     "New description",
			wantDescription: "New description",
			wantSuccess:     true,
		},
		{
			name: "valid description on editing task",
			setupTask: func(tc *TaskController, s store.Store) {
				t := newTestTask()
				_ = s.CreateTask(t)
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
			navController := newMockNavigationController()
			tc := NewTaskController(taskStore, navController)

			tt.setupTask(tc, taskStore)

			got := tc.SaveDescription(tt.description)

			if got != tt.wantSuccess {
				t.Errorf("SaveDescription() = %v, want %v", got, tt.wantSuccess)
			}

			if tt.wantSuccess {
				var actualDescription string
				if tc.draftTask != nil {
					actualDescription = tc.draftTask.Description
				} else if tc.editingTask != nil {
					actualDescription = tc.editingTask.Description
				}
				if actualDescription != tt.wantDescription {
					t.Errorf("task.Description = %q, want %q", actualDescription, tt.wantDescription)
				}
			}
		})
	}
}

// Test Edit Session Management

func TestTaskController_CommitEditSession_Draft(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	draft := newTestTaskWithID()
	draft.Title = "Draft Title"
	tc.SetDraft(draft)

	err := tc.CommitEditSession()
	if err != nil {
		t.Fatalf("CommitEditSession failed: %v", err)
	}

	// Verify draft was cleared
	if tc.GetDraftTask() != nil {
		t.Error("CommitEditSession did not clear draft")
	}

	// Verify task was created in store
	created := taskStore.GetTask("DRAFT-1")
	if created == nil {
		t.Fatal("Task was not created in store")
	}

	if created.Title != "Draft Title" {
		t.Errorf("Created task has wrong title, got %q, want %q", created.Title, "Draft Title")
	}
}

func TestTaskController_CommitEditSession_DraftValidationFailure(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	draft := newTestTaskWithID()
	draft.Title = "" // Invalid - empty title
	tc.SetDraft(draft)

	err := tc.CommitEditSession()
	if err != nil {
		// Note: Current implementation returns nil on validation failure for drafts
		// and just logs a warning. This test documents that behavior.
		t.Logf("CommitEditSession returned error as expected: %v", err)
	}

	// Draft should still exist since validation failed
	if tc.GetDraftTask() == nil {
		t.Error("Draft was cleared despite validation failure")
	}
}

func TestTaskController_CommitEditSession_Existing(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	// Create original task
	original := newTestTask()
	_ = taskStore.CreateTask(original)

	// Start edit session and modify
	tc.StartEditSession(original.ID)
	tc.editingTask.Title = "Modified Title"

	err := tc.CommitEditSession()
	if err != nil {
		t.Fatalf("CommitEditSession failed: %v", err)
	}

	// Verify editing task was cleared
	if tc.GetEditingTask() != nil {
		t.Error("CommitEditSession did not clear editingTask")
	}

	// Verify task was updated in store
	updated := taskStore.GetTask(original.ID)
	if updated == nil {
		t.Fatal("Task not found in store")
	}

	if updated.Title != "Modified Title" {
		t.Errorf("Task was not updated, got title %q, want %q", updated.Title, "Modified Title")
	}
}

func TestTaskController_CommitEditSession_NoActiveSession(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	err := tc.CommitEditSession()
	if err != nil {
		t.Errorf("CommitEditSession with no active session should return nil, got error: %v", err)
	}
}

// Test Helper Methods

func TestTaskController_GetCurrentTask(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	// Create task
	original := newTestTask()
	_ = taskStore.CreateTask(original)

	// Set as current
	tc.SetCurrentTask(original.ID)

	current := tc.GetCurrentTask()
	if current == nil {
		t.Fatal("GetCurrentTask returned nil")
	}

	if current.ID != original.ID {
		t.Errorf("GetCurrentTask returned wrong task, got ID %q, want %q", current.ID, original.ID)
	}
}

func TestTaskController_GetCurrentTask_Empty(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	current := tc.GetCurrentTask()
	if current != nil {
		t.Error("GetCurrentTask should return nil when currentTaskID is empty")
	}
}

func TestTaskController_GetCurrentTask_NonExistent(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

	tc.SetCurrentTask("NONEXISTENT")

	current := tc.GetCurrentTask()
	if current != nil {
		t.Error("GetCurrentTask should return nil for non-existent task")
	}
}

// Test Action Registry

func TestTaskController_GetActionRegistry(t *testing.T) {
	taskStore := store.NewInMemoryStore()
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

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
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

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
	navController := newMockNavigationController()
	tc := NewTaskController(taskStore, navController)

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
