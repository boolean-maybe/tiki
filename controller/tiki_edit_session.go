package controller

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"
	"github.com/boolean-maybe/tiki/workflow/value"

	"time"
)

// TikiEditSession handles task detail actions: editing, status changes, comments.

// TikiEditSession handles task detail view actions
type TikiEditSession struct {
	taskStore     store.Store
	mutationGate  *service.TikiMutationGate
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

// NewTikiEditSession creates a new TikiEditSession for managing task detail operations.
// It initializes action registries for both detail and edit views.
func NewTikiEditSession(
	taskStore store.Store,
	mutationGate *service.TikiMutationGate,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
) *TikiEditSession {
	return &TikiEditSession{
		taskStore:     taskStore,
		mutationGate:  mutationGate,
		navController: navController,
		statusline:    statusline,
		registry:      TaskDetailViewActions(),
		editRegistry:  TaskEditViewActions(),
	}
}

// SetCurrentTask sets the task ID for the currently viewed or edited task.
func (tc *TikiEditSession) SetCurrentTask(taskID string) {
	tc.currentTaskID = taskID
}

// SetDraft sets a draft tiki for creation flow (not yet persisted).
func (tc *TikiEditSession) SetDraft(tk *tikipkg.Tiki) {
	tc.draftTiki = tk
	if tk != nil {
		tc.currentTaskID = tk.ID
	}
}

// ClearDraft removes any in-progress draft.
func (tc *TikiEditSession) ClearDraft() {
	tc.draftTiki = nil
}

// StartEditSession creates an in-memory copy of the specified tiki for editing.
// It loads the tiki from the store and records its modification time for optimistic locking.
// Returns the editing copy, or nil if the tiki cannot be found.
func (tc *TikiEditSession) StartEditSession(taskID string) *tikipkg.Tiki {
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
func (tc *TikiEditSession) GetEditingTiki() *tikipkg.Tiki {
	return tc.editingTiki
}

// GetDraftTiki returns the draft tiki being created (or nil if not creating)
func (tc *TikiEditSession) GetDraftTiki() *tikipkg.Tiki {
	return tc.draftTiki
}

// CancelEditSession discards the editing copy without saving changes.
// This clears the in-memory editing tiki and resets the current task ID.
func (tc *TikiEditSession) CancelEditSession() {
	tc.editingTiki = nil
	tc.originalMtime = time.Time{}
	tc.currentTaskID = ""
}

// CommitEditSession validates and persists changes from the current edit session.
// For draft tikis (new task creation), it validates, sets timestamps, and creates the file.
// For existing tikis, it checks for external modifications and updates the tiki in the store.
// Returns an error if validation fails or the tiki cannot be saved.
func (tc *TikiEditSession) CommitEditSession() error {
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
func (tc *TikiEditSession) GetActionRegistry() *ActionRegistry {
	return tc.registry
}

// GetEditActionRegistry returns the actions for the task edit view
func (tc *TikiEditSession) GetEditActionRegistry() *ActionRegistry {
	return tc.editRegistry
}

// HandleAction processes task detail view actions such as editing title or source.
// Returns true if the action was handled, false otherwise.
func (tc *TikiEditSession) HandleAction(actionID ActionID) bool {
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

func (tc *TikiEditSession) handleEditTitle() bool {
	tk := tc.GetCurrentTiki()
	if tk == nil {
		return false
	}

	// Title editing is handled by InputRouter which has access to the view
	// This method is kept for consistency but the actual work is done in InputRouter
	return true
}

func (tc *TikiEditSession) handleEditSource() bool {
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
func (tc *TikiEditSession) SaveTitle(newTitle string) bool {
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
func (tc *TikiEditSession) SaveTags(tags []string) bool {
	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		normalized := collectionutil.NormalizeStringSet(tags)
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
func (tc *TikiEditSession) SaveDescription(newDescription string) bool {
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
func (tc *TikiEditSession) updateTikiField(setter func(*tikipkg.Tiki)) bool {
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

// SaveStatus saves the new status to the current tiki. Accepts either a
// canonical enum key (the form emitted by the SemanticEnum editor) or a
// display string (legacy form from the older typed editor). The lookup
// order matters: a display-string match takes priority over normalization,
// because NormalizeStatus("Done ✅") would camelCase the emoji into a
// nonsense key ("done✅") and silently fall back to the default status.
// Returns true on a successful update.
func (tc *TikiEditSession) SaveStatus(statusOrDisplay string) bool {
	if statusOrDisplay == "" {
		return tc.updateTikiField(func(tk *tikipkg.Tiki) {
			tk.Delete(tikipkg.FieldStatus)
		})
	}
	statusFD, hasStatus := workflow.Field(tikipkg.FieldStatus)
	// Canonical key path: editor-emitted values land here directly.
	if hasStatus && statusFD.IsValidEnum(statusOrDisplay) {
		return tc.updateTikiField(func(tk *tikipkg.Tiki) {
			tk.Set(tikipkg.FieldStatus, statusOrDisplay)
		})
	}
	// Display-string path: legacy callers pass "Ready 📋" etc.
	if hasStatus {
		if key, ok := statusFD.EnumParseDisplay(statusOrDisplay); ok {
			return tc.updateTikiField(func(tk *tikipkg.Tiki) {
				tk.Set(tikipkg.FieldStatus, key)
			})
		}
	}
	// Loose normalization: camelCase / underscore / space variants.
	if newStatus := normalizeStatusKey(statusOrDisplay); hasStatus && statusFD.IsValidEnum(newStatus) {
		return tc.updateTikiField(func(tk *tikipkg.Tiki) {
			tk.Set(tikipkg.FieldStatus, newStatus)
		})
	}
	// Catch-all fallback to the configured default — preserves the legacy
	// behavior where unrecognized input lands on the default status rather
	// than rejecting the save.
	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		def := ""
		if hasStatus {
			def = statusFD.EnumDefault()
		}
		tk.Set(tikipkg.FieldStatus, def)
	})
}

// SaveType saves the new type to the current tiki. Accepts either a canonical
// enum key or a display string — same dual-form contract as SaveStatus, since
// the SemanticEnum editor emits keys but legacy callers still pass displays.
// Returns true on a successful update.
func (tc *TikiEditSession) SaveType(typeOrDisplay string) bool {
	if typeOrDisplay == "" {
		return tc.updateTikiField(func(tk *tikipkg.Tiki) {
			tk.Delete(tikipkg.FieldType)
		})
	}
	typeFD, hasType := workflow.Field(tikipkg.FieldType)
	if hasType && typeFD.IsValidEnum(typeOrDisplay) {
		return tc.updateTikiField(func(tk *tikipkg.Tiki) {
			tk.Set(tikipkg.FieldType, typeOrDisplay)
		})
	}
	if hasType {
		if newType, ok := typeFD.EnumParseDisplay(typeOrDisplay); ok {
			return tc.updateTikiField(func(tk *tikipkg.Tiki) {
				tk.Set(tikipkg.FieldType, newType)
			})
		}
	}
	slog.Warn("unrecognized type", "input", typeOrDisplay)
	return false
}

// SaveWorkflowEnum saves a value to a workflow-declared enum field. Used by
// the detail edit handler for custom enum fields (severity, environment,
// etc.) — fields that have no built-in Save* method but still need an edit
// path. The value must be a valid key for the configured enum, or empty
// to delete the field. Returns true on success.
func (tc *TikiEditSession) SaveWorkflowEnum(fieldName, value string) bool {
	wfd, ok := workflow.Field(fieldName)
	if !ok || wfd.Type != workflow.TypeEnum {
		slog.Warn("SaveWorkflowEnum: not an enum field", "field", fieldName)
		return false
	}
	if value != "" && !wfd.IsValidEnum(value) {
		slog.Warn("SaveWorkflowEnum: invalid enum key", "field", fieldName, "value", value)
		return false
	}
	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		if value == "" {
			tk.Delete(fieldName)
		} else {
			tk.Set(fieldName, value)
		}
	})
}

// SavePriority saves the new priority to the current tiki. Priority is now
// a workflow enum: empty string deletes the field, any other value must be
// a recognized canonical key for the configured priority enum. Display
// strings are not accepted — the editor emits canonical keys directly.
// Returns true if the priority was successfully updated, false otherwise.
func (tc *TikiEditSession) SavePriority(priority string) bool {
	if priority != "" {
		fd, ok := workflow.Field(tikipkg.FieldPriority)
		if !ok || !fd.IsValidEnum(priority) {
			slog.Warn("invalid priority", "value", priority)
			return false
		}
	}

	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		if priority == "" {
			tk.Delete(tikipkg.FieldPriority)
		} else {
			tk.Set(tikipkg.FieldPriority, priority)
		}
	})
}

// SaveAssignee saves the new assignee to the current tiki.
// The special value "Unassigned" is normalized to an empty string.
// Returns true if the assignee was successfully updated, false otherwise.
func (tc *TikiEditSession) SaveAssignee(assignee string) bool {
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

// SavePoints saves the new story points to the current tiki. Points is now
// a workflow enum (declared values like "1"/"3"/"7"/"11" in kanban.yaml);
// the int argument is the legacy interface from the per-field save plumbing
// and is normalized to its decimal string form for enum-key validation.
// Zero (or any value not declared as an enum key) clears the field.
func (tc *TikiEditSession) SavePoints(points int) bool {
	fd, ok := workflow.Field(tikipkg.FieldPoints)
	clearField := func(tk *tikipkg.Tiki) { tk.Delete(tikipkg.FieldPoints) }
	if !ok || fd.Type != workflow.TypeEnum {
		// Points isn't an enum in the current workflow; treat as a no-op
		// rather than writing an integer that would fail validation.
		slog.Warn("points field is not a workflow enum; ignoring save", "value", points)
		return false
	}

	if points == 0 {
		return tc.updateTikiField(clearField)
	}
	key := strconv.Itoa(points)
	if !fd.IsValidEnum(key) {
		slog.Warn("points value not in workflow enum", "value", key)
		return false
	}
	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		tk.Set(tikipkg.FieldPoints, key)
	})
}

// SaveDue saves the new due date to the current tiki.
// Empty string clears the due date (sets to zero time).
// Returns true if the due date was successfully updated, false otherwise.
func (tc *TikiEditSession) SaveDue(dateStr string) bool {
	parsed, ok := value.ParseDate(dateStr)
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
func (tc *TikiEditSession) SaveRecurrence(cron string) bool {
	r := value.Recurrence(cron)
	if !value.IsValidRecurrence(r) {
		slog.Warn("invalid recurrence", "cron", cron)
		return false
	}
	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		if r == value.RecurrenceNone {
			tk.Delete(tikipkg.FieldRecurrence)
			tk.Delete(tikipkg.FieldDue)
		} else {
			tk.Set(tikipkg.FieldRecurrence, string(r))
			tk.Set(tikipkg.FieldDue, value.NextOccurrence(r))
		}
	})
}

func (tc *TikiEditSession) handleCloneTask() bool {
	// TODO: trigger task clone flow from detail view
	return true
}

// GetCurrentTiki returns the tiki being viewed or edited.
// Returns nil if no task is currently active.
func (tc *TikiEditSession) GetCurrentTiki() *tikipkg.Tiki {
	if tc.currentTaskID == "" {
		return nil
	}
	return tc.taskStore.GetTiki(tc.currentTaskID)
}

// GetCurrentTaskID returns the ID of the current task
func (tc *TikiEditSession) GetCurrentTaskID() string {
	return tc.currentTaskID
}

// GetFocusedField returns the currently focused field in edit mode
func (tc *TikiEditSession) GetFocusedField() model.EditField {
	return tc.focusedField
}

// SetFocusedField sets the currently focused field in edit mode
func (tc *TikiEditSession) SetFocusedField(field model.EditField) {
	tc.focusedField = field
}

// AddComment adds a new comment to the current task with the specified author and text.
// Returns false if no task is currently active, true if the comment was added successfully.
func (tc *TikiEditSession) AddComment(author, text string) bool {
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

	comment := tikipkg.Comment{
		ID:        generateID(),
		Author:    author,
		Text:      text,
		CreatedAt: time.Now(),
	}

	var existing []tikipkg.Comment
	if v, ok := tk.Fields["comments"]; ok {
		if cs, ok := v.([]tikipkg.Comment); ok {
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

// normalizeStatusKey converts a raw status string ("in_progress", "In Progress",
// "IN_PROGRESS") to canonical camelCase ("inProgress"). Splits on "_", "-",
// " ", and camelCase boundaries, then reassembles. Generic — no knowledge of
// any specific status field's allowed values; callers must validate the
// result against the workflow catalog.
func normalizeStatusKey(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var words []string
	for _, p := range parts {
		words = append(words, splitCamelCase(p)...)
	}
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	for i, w := range words {
		if i == 0 {
			b.WriteString(strings.ToLower(w))
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]))
		b.WriteString(strings.ToLower(w[1:]))
	}
	return b.String()
}

func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}
	var words []string
	start := 0
	for i := 1; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' && s[i-1] >= 'a' && s[i-1] <= 'z' {
			words = append(words, s[start:i])
			start = i
		}
	}
	words = append(words, s[start:])
	return words
}
