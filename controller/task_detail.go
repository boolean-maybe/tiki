package controller

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"

	"time"
)

// TaskController handles task detail actions: editing, status changes, comments.

// TaskController handles task detail view actions
type TaskController struct {
	taskStore     store.Store
	navController *NavigationController
	currentTaskID string
	draftTask     *taskpkg.Task // For new task creation only
	editingTask   *taskpkg.Task // In-memory copy being edited (existing tasks)
	originalMtime time.Time     // LoadedMtime when edit started
	registry      *ActionRegistry
	editRegistry  *ActionRegistry
	focusedField  model.EditField // currently focused field in edit mode
}

// NewTaskController creates a new TaskController for managing task detail operations.
// It initializes action registries for both detail and edit views.
func NewTaskController(
	taskStore store.Store,
	navController *NavigationController,
) *TaskController {
	return &TaskController{
		taskStore:     taskStore,
		navController: navController,
		registry:      TaskDetailViewActions(),
		editRegistry:  TaskEditViewActions(),
	}
}

// SetCurrentTask sets the task ID for the currently viewed or edited task.
func (tc *TaskController) SetCurrentTask(taskID string) {
	tc.currentTaskID = taskID
}

// SetDraft sets a draft task for creation flow (not yet persisted).
func (tc *TaskController) SetDraft(task *taskpkg.Task) {
	tc.draftTask = task
	if task != nil {
		tc.currentTaskID = task.ID
	}
}

// ClearDraft removes any in-progress draft task.
func (tc *TaskController) ClearDraft() {
	tc.draftTask = nil
}

// StartEditSession creates an in-memory copy of the specified task for editing.
// It loads the task from the store and records its modification time for optimistic locking.
// Returns the editing copy, or nil if the task cannot be found.
func (tc *TaskController) StartEditSession(taskID string) *taskpkg.Task {
	task := tc.taskStore.GetTask(taskID)
	if task == nil {
		return nil
	}

	tc.editingTask = task.Clone()
	tc.originalMtime = task.LoadedMtime
	tc.currentTaskID = taskID

	return tc.editingTask
}

// GetEditingTask returns the task being edited (or nil if not editing)
func (tc *TaskController) GetEditingTask() *taskpkg.Task {
	return tc.editingTask
}

// GetDraftTask returns the draft task being created (or nil if not creating)
func (tc *TaskController) GetDraftTask() *taskpkg.Task {
	return tc.draftTask
}

// CancelEditSession discards the editing copy without saving changes.
// This clears the in-memory editing task and resets the current task ID.
func (tc *TaskController) CancelEditSession() {
	tc.editingTask = nil
	tc.originalMtime = time.Time{}
	tc.currentTaskID = ""
}

// CommitEditSession validates and persists changes from the current edit session.
// For draft tasks (new task creation), it validates, sets timestamps, and creates the file.
// For existing tasks, it checks for external modifications and updates the task in the store.
// Returns an error if validation fails or the task cannot be saved.
func (tc *TaskController) CommitEditSession() error {
	// Handle draft task creation
	if tc.draftTask != nil {
		// Validate draft task before persisting
		if errors := tc.draftTask.Validate(); errors.HasErrors() {
			slog.Warn("draft task validation failed", "errors", errors.Error())
			return nil // Don't save invalid draft
		}

		// Set timestamps and author for new task
		now := time.Now()
		if tc.draftTask.CreatedAt.IsZero() {
			tc.draftTask.CreatedAt = now
		}
		setAuthorFromGit(tc.draftTask, tc.taskStore)

		// Create the task file
		if err := tc.taskStore.CreateTask(tc.draftTask); err != nil {
			slog.Error("failed to create draft task", "error", err)
			return fmt.Errorf("failed to create task: %w", err)
		}

		// Clear the draft
		tc.draftTask = nil
		return nil
	}

	// Handle existing task updates
	if tc.editingTask == nil {
		return nil // No active edit session, nothing to commit
	}

	// Validate editing task before persisting
	if errors := tc.editingTask.Validate(); errors.HasErrors() {
		slog.Warn("editing task validation failed", "taskID", tc.currentTaskID, "errors", errors.Error())
		return fmt.Errorf("validation failed: %w", errors)
	}

	// Check for conflicts (file was modified externally)
	currentTask := tc.taskStore.GetTask(tc.currentTaskID)
	if currentTask != nil && !currentTask.LoadedMtime.Equal(tc.originalMtime) {
		// TODO: Better error handling - show error to user
		slog.Warn("task was modified externally", "taskID", tc.currentTaskID)
		// For now, proceed with save (last write wins)
	}

	// Update the task in the store
	if err := tc.taskStore.UpdateTask(tc.editingTask); err != nil {
		slog.Error("failed to update task", "taskID", tc.currentTaskID, "error", err)
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Clear the edit session
	tc.editingTask = nil
	tc.originalMtime = time.Time{}

	return nil
}

// GetActionRegistry returns the actions for the task detail view
func (tc *TaskController) GetActionRegistry() *ActionRegistry {
	return tc.registry
}

// GetEditActionRegistry returns the actions for the task edit view
func (tc *TaskController) GetEditActionRegistry() *ActionRegistry {
	return tc.editRegistry
}

// HandleAction processes task detail view actions such as editing title or source.
// Returns true if the action was handled, false otherwise.
func (tc *TaskController) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionEditTitle:
		return tc.handleEditTitle()
	case ActionEditSource:
		return tc.handleEditSource()
	case ActionCloneTask:
		return tc.handleCloneTask()
	default:
		return false
	}
}

func (tc *TaskController) handleEditTitle() bool {
	task := tc.GetCurrentTask()
	if task == nil {
		return false
	}

	// Title editing is handled by InputRouter which has access to the view
	// This method is kept for consistency but the actual work is done in InputRouter
	return true
}

func (tc *TaskController) handleEditSource() bool {
	task := tc.GetCurrentTask()
	if task == nil {
		return false
	}

	// Construct the file path for this task
	filename := strings.ToLower(task.ID) + ".md"
	filePath := filepath.Join(config.TaskDir, filename)

	// Suspend the tview app and open the editor
	tc.navController.SuspendAndEdit(filePath)

	// Reload only this task after editing (more efficient than reloading all tasks)
	// This preserves any custom YAML fields, comments, or formatting added in the external editor
	_ = tc.taskStore.ReloadTask(task.ID)

	return true
}

// SaveTitle saves the new title to the current task (draft or editing).
// For draft tasks (new task creation), updates the draft; for editing tasks, updates the editing copy.
// Returns true if a task was updated, false if no task is being edited.
func (tc *TaskController) SaveTitle(newTitle string) bool {
	// Update draft task first (new task creation takes priority)
	if tc.draftTask != nil {
		tc.draftTask.Title = newTitle
		return true
	}
	// Otherwise update editing task (existing task editing)
	if tc.editingTask != nil {
		tc.editingTask.Title = newTitle
		return true
	}
	return false
}

// SaveDescription saves the new description to the current task (draft or editing).
// For draft tasks (new task creation), updates the draft; for editing tasks, updates the editing copy.
// Returns true if a task was updated, false if no task is being edited.
func (tc *TaskController) SaveDescription(newDescription string) bool {
	// Update draft task first (new task creation takes priority)
	if tc.draftTask != nil {
		tc.draftTask.Description = newDescription
		return true
	}
	// Otherwise update editing task (existing task editing)
	if tc.editingTask != nil {
		tc.editingTask.Description = newDescription
		return true
	}
	return false
}

// updateTaskField updates a field in either the draft task or editing task.
// It applies the setter function to the appropriate task based on priority:
// draft task (new task creation) takes priority over editing task (existing task edit).
// Returns true if a task was updated, false if no task is being edited.
func (tc *TaskController) updateTaskField(setter func(*taskpkg.Task)) bool {
	if tc.draftTask != nil {
		setter(tc.draftTask)
		return true
	}
	if tc.editingTask != nil {
		setter(tc.editingTask)
		return true
	}
	return false
}

// SaveStatus saves the new status to the current task after validating the display value.
// Returns true if the status was successfully updated, false otherwise.
func (tc *TaskController) SaveStatus(statusDisplay string) bool {
	// Parse status display back to TaskStatus
	// Try to match the display string to a known status
	var newStatus taskpkg.Status
	statusFound := false

	for _, s := range []taskpkg.Status{
		taskpkg.StatusBacklog,
		taskpkg.StatusReady,
		taskpkg.StatusInProgress,
		taskpkg.StatusReview,
		taskpkg.StatusDone,
	} {
		if taskpkg.StatusDisplay(s) == statusDisplay {
			newStatus = s
			statusFound = true
			break
		}
	}

	if !statusFound {
		// fallback: try to normalize the input
		newStatus = taskpkg.NormalizeStatus(statusDisplay)
	}

	// Validate using StatusValidator
	tempTask := &taskpkg.Task{Status: newStatus}
	if err := tempTask.ValidateField("status"); err != nil {
		slog.Warn("invalid status", "display", statusDisplay, "normalized", newStatus, "error", err.Message)
		return false
	}

	// Use generic updater
	return tc.updateTaskField(func(t *taskpkg.Task) {
		t.Status = newStatus
	})
}

// SaveType saves the new type to the current task after validating the display value.
// Returns true if the type was successfully updated, false otherwise.
func (tc *TaskController) SaveType(typeDisplay string) bool {
	// Parse type display back to TaskType
	var newType taskpkg.Type
	typeFound := false

	for _, t := range []taskpkg.Type{
		taskpkg.TypeStory,
		taskpkg.TypeBug,
		taskpkg.TypeSpike,
		taskpkg.TypeEpic,
	} {
		if taskpkg.TypeDisplay(t) == typeDisplay {
			newType = t
			typeFound = true
			break
		}
	}

	if !typeFound {
		newType = taskpkg.NormalizeType(typeDisplay)
	}

	// Validate using TypeValidator
	tempTask := &taskpkg.Task{Type: newType}
	if err := tempTask.ValidateField("type"); err != nil {
		slog.Warn("invalid type", "display", typeDisplay, "normalized", newType, "error", err.Message)
		return false
	}

	return tc.updateTaskField(func(t *taskpkg.Task) {
		t.Type = newType
	})
}

// SavePriority saves the new priority to the current task.
// Returns true if the priority was successfully updated, false otherwise.
func (tc *TaskController) SavePriority(priority int) bool {
	// Validate using PriorityValidator
	tempTask := &taskpkg.Task{Priority: priority}
	if err := tempTask.ValidateField("priority"); err != nil {
		slog.Warn("invalid priority", "value", priority, "error", err.Message)
		return false
	}

	return tc.updateTaskField(func(t *taskpkg.Task) {
		t.Priority = priority
	})
}

// SaveAssignee saves the new assignee to the current task.
// The special value "Unassigned" is normalized to an empty string.
// Returns true if the assignee was successfully updated, false otherwise.
func (tc *TaskController) SaveAssignee(assignee string) bool {
	// Normalize "Unassigned" to empty string
	if assignee == "Unassigned" {
		assignee = ""
	}

	return tc.updateTaskField(func(t *taskpkg.Task) {
		t.Assignee = assignee
	})
}

// SavePoints saves the new story points to the current task.
// Returns true if the points were successfully updated, false otherwise.
func (tc *TaskController) SavePoints(points int) bool {
	// Validate using PointsValidator
	tempTask := &taskpkg.Task{Points: points}
	if err := tempTask.ValidateField("points"); err != nil {
		slog.Warn("invalid points", "value", points, "error", err.Message)
		return false
	}

	return tc.updateTaskField(func(t *taskpkg.Task) {
		t.Points = points
	})
}

func (tc *TaskController) handleCloneTask() bool {
	// TODO: trigger task clone flow from detail view
	return true
}

// SaveTaskDetails persists edited task fields in a single update
func (tc *TaskController) SaveTaskDetails(newTitle, newDescription string) bool {
	if tc.currentTaskID == "" {
		return false
	}

	task := tc.taskStore.GetTask(tc.currentTaskID)

	// new task creation flow using draft (not yet persisted)
	if task == nil && tc.draftTask != nil && tc.draftTask.ID == tc.currentTaskID {
		title := strings.TrimSpace(newTitle)
		if title == "" {
			return false
		}

		draft := tc.draftTask
		draft.Title = title
		draft.Description = newDescription
		now := time.Now()
		if draft.CreatedAt.IsZero() {
			draft.CreatedAt = now
		}
		// Note: UpdatedAt will be computed after save based on file mtime

		setAuthorFromGit(draft, tc.taskStore)

		if err := tc.taskStore.CreateTask(draft); err != nil {
			slog.Error("failed to create task from draft", "error", err)
			// Don't clear draft on error - let user retry
			return false
		}

		tc.draftTask = nil
		return true
	}

	if task == nil {
		return false
	}

	updated := false

	if task.Title != newTitle {
		task.Title = newTitle
		updated = true
	}

	if task.Description != newDescription {
		task.Description = newDescription
		updated = true
	}

	if updated {
		_ = tc.taskStore.UpdateTask(task)
	}

	return true
}

// GetCurrentTask returns the task being viewed or edited.
// Returns nil if no task is currently active.
func (tc *TaskController) GetCurrentTask() *taskpkg.Task {
	if tc.currentTaskID == "" {
		return nil
	}
	return tc.taskStore.GetTask(tc.currentTaskID)
}

// GetCurrentTaskID returns the ID of the current task
func (tc *TaskController) GetCurrentTaskID() string {
	return tc.currentTaskID
}

// GetFocusedField returns the currently focused field in edit mode
func (tc *TaskController) GetFocusedField() model.EditField {
	return tc.focusedField
}

// SetFocusedField sets the currently focused field in edit mode
func (tc *TaskController) SetFocusedField(field model.EditField) {
	tc.focusedField = field
}

// UpdateTask persists changes to the specified task in the store.
func (tc *TaskController) UpdateTask(task *taskpkg.Task) {
	_ = tc.taskStore.UpdateTask(task)
}

// AddComment adds a new comment to the current task with the specified author and text.
// Returns false if no task is currently active, true if the comment was added successfully.
func (tc *TaskController) AddComment(author, text string) bool {
	if tc.currentTaskID == "" {
		return false
	}

	comment := taskpkg.Comment{
		ID:     generateID(),
		Author: author,
		Text:   text,
	}
	return tc.taskStore.AddComment(tc.currentTaskID, comment)
}
