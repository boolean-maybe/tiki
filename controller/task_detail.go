package controller

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"

	"time"
)

// TaskController handles task detail actions: editing, status changes, comments.

// TaskController handles task detail view actions
type TaskController struct {
	taskStore     store.Store
	mutationGate  *service.TaskMutationGate
	navController *NavigationController
	statusline    *model.StatuslineConfig
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
	mutationGate *service.TaskMutationGate,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
) *TaskController {
	return &TaskController{
		taskStore:     taskStore,
		mutationGate:  mutationGate,
		navController: navController,
		statusline:    statusline,
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
		setAuthorFromGit(tc.draftTask, tc.taskStore)

		if err := tc.mutationGate.CreateTask(context.Background(), tc.draftTask); err != nil {
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

	// Check for conflicts (file was modified externally)
	currentTask := tc.taskStore.GetTask(tc.currentTaskID)
	if currentTask != nil && !currentTask.LoadedMtime.Equal(tc.originalMtime) {
		// TODO: Better error handling - show error to user
		slog.Warn("task was modified externally", "taskID", tc.currentTaskID)
		// For now, proceed with save (last write wins)
	}

	if err := tc.mutationGate.UpdateTask(context.Background(), tc.editingTask); err != nil {
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

	// Use the task's own path so renames, moves, and new-at-root layouts
	// all target the real file. Falling back to the id-derived default
	// keeps the behavior meaningful for in-memory tasks that haven't
	// been persisted yet.
	filePath := task.FilePath
	if filePath == "" {
		filePath = filepath.Join(config.GetDocDir(), task.ID+".md")
	}

	if err := tc.navController.SuspendAndEdit(filePath); err != nil {
		if tc.statusline != nil {
			tc.statusline.SetMessage("editor failed: "+err.Error(), model.MessageLevelError, true)
		}
		return true
	}

	// Surface reload errors — after an external edit the file's frontmatter
	// may have gained a conflict (collision, invalid id, unknown type) the
	// user needs to resolve. Silently swallowing the error leaves the UI
	// showing stale data with no hint that anything went wrong.
	if err := tc.taskStore.ReloadTask(task.ID); err != nil && tc.statusline != nil {
		tc.statusline.SetMessage("reload failed: "+err.Error(), model.MessageLevelError, true)
	}

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

// SaveTags saves the new tags to the current task (draft or editing).
// Returns true if a task was updated, false if no task is being edited.
func (tc *TaskController) SaveTags(tags []string) bool {
	return tc.updateTaskField(func(t *taskpkg.Task) {
		t.Tags = taskpkg.NormalizeStringSet(tags)
	})
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

	for _, s := range taskpkg.AllStatuses() {
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

	// Validate status
	tempTask := &taskpkg.Task{Status: newStatus}
	if msg := taskpkg.ValidateStatus(tempTask); msg != "" {
		slog.Warn("invalid status", "display", statusDisplay, "normalized", newStatus, "error", msg)
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
	// reverse the display string ("Bug 💥") back to a canonical key ("bug")
	newType, ok := taskpkg.ParseDisplay(typeDisplay)
	if !ok {
		slog.Warn("unrecognized type display", "display", typeDisplay)
		return false
	}

	// Validate type
	tempTask := &taskpkg.Task{Type: newType}
	if msg := taskpkg.ValidateType(tempTask); msg != "" {
		slog.Warn("invalid type", "display", typeDisplay, "normalized", newType, "error", msg)
		return false
	}

	return tc.updateTaskField(func(t *taskpkg.Task) {
		t.Type = newType
	})
}

// SavePriority saves the new priority to the current task.
// Returns true if the priority was successfully updated, false otherwise.
func (tc *TaskController) SavePriority(priority int) bool {
	// Validate priority
	tempTask := &taskpkg.Task{Priority: priority}
	if msg := taskpkg.ValidatePriority(tempTask); msg != "" {
		slog.Warn("invalid priority", "value", priority, "error", msg)
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
	// Validate points
	tempTask := &taskpkg.Task{Points: points}
	if msg := taskpkg.ValidatePoints(tempTask); msg != "" {
		slog.Warn("invalid points", "value", points, "error", msg)
		return false
	}

	return tc.updateTaskField(func(t *taskpkg.Task) {
		t.Points = points
	})
}

// SaveDue saves the new due date to the current task.
// Empty string clears the due date (sets to zero time).
// Returns true if the due date was successfully updated, false otherwise.
func (tc *TaskController) SaveDue(dateStr string) bool {
	parsed, ok := taskpkg.ParseDueDate(dateStr)
	if !ok {
		return false
	}
	return tc.updateTaskField(func(t *taskpkg.Task) {
		t.Due = parsed
	})
}

// SaveRecurrence saves the new recurrence cron expression to the current task.
// When recurrence is set, Due is auto-computed as the next occurrence.
// When recurrence is cleared, Due is also cleared.
// Returns true if the recurrence was successfully updated, false otherwise.
func (tc *TaskController) SaveRecurrence(cron string) bool {
	r := taskpkg.Recurrence(cron)
	if !taskpkg.IsValidRecurrence(r) {
		slog.Warn("invalid recurrence", "cron", cron)
		return false
	}
	return tc.updateTaskField(func(t *taskpkg.Task) {
		t.Recurrence = r
		if r == taskpkg.RecurrenceNone {
			t.Due = time.Time{}
		} else {
			t.Due = taskpkg.NextOccurrence(r)
		}
	})
}

func (tc *TaskController) handleCloneTask() bool {
	// TODO: trigger task clone flow from detail view
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

// UpdateTask persists changes to the specified task via the mutation gate.
func (tc *TaskController) UpdateTask(task *taskpkg.Task) {
	_ = tc.mutationGate.UpdateTask(context.Background(), task)
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
	if err := tc.mutationGate.AddComment(tc.currentTaskID, comment); err != nil {
		slog.Error("failed to add comment", "taskID", tc.currentTaskID, "error", err)
		if tc.statusline != nil {
			tc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
		}
		return false
	}
	return true
}
