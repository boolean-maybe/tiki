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
	tikipkg "github.com/boolean-maybe/tiki/tiki"

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
	draftTiki     *tikipkg.Tiki // For new task creation only
	editingTiki   *tikipkg.Tiki // In-memory copy being edited (existing tasks)
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

// SetDraft sets a draft tiki for creation flow (not yet persisted).
func (tc *TaskController) SetDraft(tk *tikipkg.Tiki) {
	tc.draftTiki = tk
	if tk != nil {
		tc.currentTaskID = tk.ID
	}
}

// ClearDraft removes any in-progress draft.
func (tc *TaskController) ClearDraft() {
	tc.draftTiki = nil
}

// StartEditSession creates an in-memory copy of the specified tiki for editing.
// It loads the tiki from the store and records its modification time for optimistic locking.
// Returns the editing copy, or nil if the tiki cannot be found.
func (tc *TaskController) StartEditSession(taskID string) *tikipkg.Tiki {
	tk := tc.taskStore.GetTiki(taskID)
	if tk == nil {
		return nil
	}

	tc.editingTiki = tk.Clone()
	tc.originalMtime = tk.LoadedMtime
	tc.currentTaskID = taskID

	return tc.editingTiki
}

// GetEditingTiki returns the tiki being edited (or nil if not editing)
func (tc *TaskController) GetEditingTiki() *tikipkg.Tiki {
	return tc.editingTiki
}

// GetDraftTiki returns the draft tiki being created (or nil if not creating)
func (tc *TaskController) GetDraftTiki() *tikipkg.Tiki {
	return tc.draftTiki
}

// CancelEditSession discards the editing copy without saving changes.
// This clears the in-memory editing tiki and resets the current task ID.
func (tc *TaskController) CancelEditSession() {
	tc.editingTiki = nil
	tc.originalMtime = time.Time{}
	tc.currentTaskID = ""
}

// CommitEditSession validates and persists changes from the current edit session.
// For draft tikis (new task creation), it validates, sets timestamps, and creates the file.
// For existing tikis, it checks for external modifications and updates the tiki in the store.
// Returns an error if validation fails or the tiki cannot be saved.
func (tc *TaskController) CommitEditSession() error {
	// Handle draft tiki creation
	if tc.draftTiki != nil {
		setAuthorOnTiki(tc.draftTiki, tc.taskStore)

		if err := tc.mutationGate.CreateTiki(context.Background(), tc.draftTiki); err != nil {
			slog.Error("failed to create draft tiki", "error", err)
			return fmt.Errorf("failed to create task: %w", err)
		}

		// Clear the draft
		tc.draftTiki = nil
		return nil
	}

	// Handle existing task updates
	if tc.editingTiki == nil {
		return nil // No active edit session, nothing to commit
	}

	// Check for conflicts (file was modified externally)
	currentTiki := tc.taskStore.GetTiki(tc.currentTaskID)
	if currentTiki != nil && !currentTiki.LoadedMtime.Equal(tc.originalMtime) {
		// TODO: Better error handling - show error to user
		slog.Warn("task was modified externally", "taskID", tc.currentTaskID)
		// For now, proceed with save (last write wins)
	}

	if err := tc.mutationGate.UpdateTiki(context.Background(), tc.editingTiki); err != nil {
		slog.Error("failed to update tiki", "taskID", tc.currentTaskID, "error", err)
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Clear the edit session
	tc.editingTiki = nil
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
	tk := tc.GetCurrentTiki()
	if tk == nil {
		return false
	}

	// Title editing is handled by InputRouter which has access to the view
	// This method is kept for consistency but the actual work is done in InputRouter
	return true
}

func (tc *TaskController) handleEditSource() bool {
	tk := tc.GetCurrentTiki()
	if tk == nil {
		return false
	}

	// Use the tiki's own path so renames, moves, and new-at-root layouts
	// all target the real file. Falling back to the id-derived default
	// keeps the behavior meaningful for in-memory tikis that haven't
	// been persisted yet.
	filePath := tk.Path
	if filePath == "" {
		filePath = filepath.Join(config.GetDocDir(), tk.ID+".md")
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
	if err := tc.taskStore.ReloadTask(tk.ID); err != nil && tc.statusline != nil {
		tc.statusline.SetMessage("reload failed: "+err.Error(), model.MessageLevelError, true)
	}

	return true
}

// SaveTitle saves the new title to the current tiki (draft or editing).
// For draft tikis (new task creation), updates the draft; for editing tikis, updates the editing copy.
// Returns true if a tiki was updated, false if no tiki is being edited.
func (tc *TaskController) SaveTitle(newTitle string) bool {
	if tc.draftTiki != nil {
		tc.draftTiki.Title = newTitle
		return true
	}
	if tc.editingTiki != nil {
		tc.editingTiki.Title = newTitle
		return true
	}
	return false
}

// SaveTags saves the new tags to the current tiki (draft or editing).
// Returns true if a tiki was updated, false if no tiki is being edited.
func (tc *TaskController) SaveTags(tags []string) bool {
	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		normalized := taskpkg.NormalizeStringSet(tags)
		if len(normalized) > 0 {
			tk.Set(tikipkg.FieldTags, normalized)
		} else {
			tk.Delete(tikipkg.FieldTags)
		}
	})
}

// SaveDescription saves the new description to the current tiki (draft or editing).
// For draft tikis (new task creation), updates the draft; for editing tikis, updates the editing copy.
// Returns true if a tiki was updated, false if no tiki is being edited.
func (tc *TaskController) SaveDescription(newDescription string) bool {
	if tc.draftTiki != nil {
		tc.draftTiki.Body = newDescription
		return true
	}
	if tc.editingTiki != nil {
		tc.editingTiki.Body = newDescription
		return true
	}
	return false
}

// updateTikiField updates a field in either the draft tiki or editing tiki.
// It applies the setter function to the appropriate tiki based on priority:
// draft tiki (new task creation) takes priority over editing tiki (existing task edit).
// Returns true if a tiki was updated, false if no tiki is being edited.
func (tc *TaskController) updateTikiField(setter func(*tikipkg.Tiki)) bool {
	if tc.draftTiki != nil {
		setter(tc.draftTiki)
		return true
	}
	if tc.editingTiki != nil {
		setter(tc.editingTiki)
		return true
	}
	return false
}

// SaveStatus saves the new status to the current tiki after validating the display value.
// Returns true if the status was successfully updated, false otherwise.
func (tc *TaskController) SaveStatus(statusDisplay string) bool {
	// parse status display back to Status
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

	// Validate status: non-empty values must be registered
	if newStatus != "" && !config.GetStatusRegistry().IsValid(string(newStatus)) {
		slog.Warn("invalid status", "display", statusDisplay, "normalized", newStatus)
		return false
	}

	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		tk.Set(tikipkg.FieldStatus, string(newStatus))
	})
}

// SaveType saves the new type to the current tiki after validating the display value.
// Returns true if the type was successfully updated, false otherwise.
func (tc *TaskController) SaveType(typeDisplay string) bool {
	// reverse the display string ("Bug 💥") back to a canonical key ("bug")
	newType, ok := taskpkg.ParseDisplay(typeDisplay)
	if !ok {
		slog.Warn("unrecognized type display", "display", typeDisplay)
		return false
	}

	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		tk.Set(tikipkg.FieldType, string(newType))
	})
}

// SavePriority saves the new priority to the current tiki.
// Returns true if the priority was successfully updated, false otherwise.
func (tc *TaskController) SavePriority(priority int) bool {
	// Validate priority: zero means absent (valid); non-zero must be in range
	if priority != 0 && !taskpkg.IsValidPriority(priority) {
		slog.Warn("invalid priority", "value", priority)
		return false
	}

	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		if priority == 0 {
			tk.Delete(tikipkg.FieldPriority)
		} else {
			tk.Set(tikipkg.FieldPriority, priority)
		}
	})
}

// SaveAssignee saves the new assignee to the current tiki.
// The special value "Unassigned" is normalized to an empty string.
// Returns true if the assignee was successfully updated, false otherwise.
func (tc *TaskController) SaveAssignee(assignee string) bool {
	// Normalize "Unassigned" to empty string
	if assignee == "Unassigned" {
		assignee = ""
	}

	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		if assignee == "" {
			tk.Delete(tikipkg.FieldAssignee)
		} else {
			tk.Set(tikipkg.FieldAssignee, assignee)
		}
	})
}

// SavePoints saves the new story points to the current tiki.
// Returns true if the points were successfully updated, false otherwise.
func (tc *TaskController) SavePoints(points int) bool {
	// Validate points: zero means absent (valid); non-zero must be in range
	if !taskpkg.IsValidPoints(points) {
		slog.Warn("invalid points", "value", points)
		return false
	}

	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		if points == 0 {
			tk.Delete(tikipkg.FieldPoints)
		} else {
			tk.Set(tikipkg.FieldPoints, points)
		}
	})
}

// SaveDue saves the new due date to the current tiki.
// Empty string clears the due date (sets to zero time).
// Returns true if the due date was successfully updated, false otherwise.
func (tc *TaskController) SaveDue(dateStr string) bool {
	parsed, ok := taskpkg.ParseDueDate(dateStr)
	if !ok {
		return false
	}
	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		if parsed.IsZero() {
			tk.Delete(tikipkg.FieldDue)
		} else {
			tk.Set(tikipkg.FieldDue, parsed)
		}
	})
}

// SaveRecurrence saves the new recurrence cron expression to the current tiki.
// When recurrence is set, Due is auto-computed as the next occurrence.
// When recurrence is cleared, Due is also cleared.
// Returns true if the recurrence was successfully updated, false otherwise.
func (tc *TaskController) SaveRecurrence(cron string) bool {
	r := taskpkg.Recurrence(cron)
	if !taskpkg.IsValidRecurrence(r) {
		slog.Warn("invalid recurrence", "cron", cron)
		return false
	}
	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		if r == taskpkg.RecurrenceNone {
			tk.Delete(tikipkg.FieldRecurrence)
			tk.Delete(tikipkg.FieldDue)
		} else {
			tk.Set(tikipkg.FieldRecurrence, string(r))
			tk.Set(tikipkg.FieldDue, taskpkg.NextOccurrence(r))
		}
	})
}

func (tc *TaskController) handleCloneTask() bool {
	// TODO: trigger task clone flow from detail view
	return true
}

// GetCurrentTiki returns the tiki being viewed or edited.
// Returns nil if no task is currently active.
func (tc *TaskController) GetCurrentTiki() *tikipkg.Tiki {
	if tc.currentTaskID == "" {
		return nil
	}
	return tc.taskStore.GetTiki(tc.currentTaskID)
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

// AddComment adds a new comment to the current task with the specified author and text.
// Returns false if no task is currently active, true if the comment was added successfully.
func (tc *TaskController) AddComment(author, text string) bool {
	if tc.currentTaskID == "" {
		return false
	}

	tk := tc.taskStore.GetTiki(tc.currentTaskID)
	if tk == nil {
		err := fmt.Errorf("task not found: %s", tc.currentTaskID)
		slog.Error("failed to add comment", "taskID", tc.currentTaskID, "error", err)
		if tc.statusline != nil {
			tc.statusline.SetMessage(err.Error(), model.MessageLevelError, true)
		}
		return false
	}

	comment := taskpkg.Comment{
		ID:        generateID(),
		Author:    author,
		Text:      text,
		CreatedAt: time.Now(),
	}

	var existing []taskpkg.Comment
	if v, ok := tk.Fields["comments"]; ok {
		if cs, ok := v.([]taskpkg.Comment); ok {
			existing = append(existing, cs...)
		}
	}
	// comments is in-memory-only: it is explicitly excluded from frontmatter
	// serialization (tiki_bridge.go inMemoryOnlyFields). Mutate the stored
	// pointer directly rather than routing through UpdateTiki, which would
	// trigger an unnecessary file write and fire validators/hooks/triggers.
	tk.Set("comments", append(existing, comment))
	return true
}
