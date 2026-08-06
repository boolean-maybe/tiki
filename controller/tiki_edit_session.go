package controller

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	collectionutil "github.com/boolean-maybe/ruki/collections"
	"github.com/boolean-maybe/ruki/recurrence"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
	"github.com/boolean-maybe/tiki/workflow/value"

	"time"
)

// TikiEditSession handles tiki detail view actions: editing, field changes, comments.
type TikiEditSession struct {
	tikiStore     store.Store
	mutationGate  *service.TikiMutationGate
	navController *NavigationController
	statusline    *model.StatuslineConfig
	currentTikiID string
	draftTiki     *tikipkg.Tiki // For new tiki creation only
	editingTiki   *tikipkg.Tiki // In-memory copy being edited (existing tikis)
	originalMtime time.Time     // LoadedMtime when edit started
	registry      *ActionRegistry
	editRegistry  *ActionRegistry
	focusedField  model.EditField // currently focused field in edit mode
}

// NewTikiEditSession creates a new TikiEditSession for managing tiki detail operations.
// It initializes action registries for both detail and edit views.
func NewTikiEditSession(
	tikiStore store.Store,
	mutationGate *service.TikiMutationGate,
	navController *NavigationController,
	statusline *model.StatuslineConfig,
) *TikiEditSession {
	return &TikiEditSession{
		tikiStore:     tikiStore,
		mutationGate:  mutationGate,
		navController: navController,
		statusline:    statusline,
		registry:      TikiDetailViewActions(),
		editRegistry:  TikiEditViewActions(),
	}
}

// SetCurrentTiki sets the tiki ID for the currently viewed or edited tiki.
func (tc *TikiEditSession) SetCurrentTiki(tikiID string) {
	tc.currentTikiID = tikiID
}

// SetDraft sets a draft tiki for creation flow (not yet persisted).
func (tc *TikiEditSession) SetDraft(tk *tikipkg.Tiki) {
	tc.draftTiki = tk
	if tk != nil {
		tc.currentTikiID = tk.ID()
	}
}

// ClearDraft removes any in-progress draft.
func (tc *TikiEditSession) ClearDraft() {
	tc.draftTiki = nil
}

// StartEditSession creates an in-memory copy of the specified tiki for editing.
// It loads the tiki from the store and records its modification time for optimistic locking.
// Returns the editing copy, or nil if the tiki cannot be found.
func (tc *TikiEditSession) StartEditSession(tikiID string) *tikipkg.Tiki {
	tk := tc.tikiStore.GetTiki(tikiID)
	if tk == nil {
		return nil
	}

	tc.editingTiki = tk.Clone()
	tc.originalMtime = tk.LoadedMtime
	tc.currentTikiID = tikiID

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
// This clears the in-memory editing tiki and resets the current tiki ID.
func (tc *TikiEditSession) CancelEditSession() {
	tc.editingTiki = nil
	tc.originalMtime = time.Time{}
	tc.currentTikiID = ""
}

// CommitEditSession validates and persists changes from the current edit session.
// For draft tikis (new tiki creation), it validates, sets timestamps, and creates the file.
// For existing tikis, it checks for external modifications and updates the tiki in the store.
// Returns an error if validation fails or the tiki cannot be saved.
func (tc *TikiEditSession) CommitEditSession() error {
	// Handle draft tiki creation
	if tc.draftTiki != nil {
		setAuthorOnTiki(tc.draftTiki, tc.tikiStore)

		if err := tc.mutationGate.CreateTiki(context.Background(), tc.draftTiki); err != nil {
			slog.Error("failed to create draft tiki", "error", err)
			return fmt.Errorf("failed to create tiki: %w", err)
		}

		// Clear the draft
		tc.draftTiki = nil
		return nil
	}

	// Handle existing tiki updates
	if tc.editingTiki == nil {
		return nil // No active edit session, nothing to commit
	}

	// Check for conflicts (file was modified externally)
	currentTiki := tc.tikiStore.GetTiki(tc.currentTikiID)
	if currentTiki != nil && !currentTiki.LoadedMtime.Equal(tc.originalMtime) {
		// TODO: Better error handling - show error to user
		slog.Warn("tiki was modified externally", "tikiID", tc.currentTikiID)
		// For now, proceed with save (last write wins)
	}

	if err := tc.mutationGate.UpdateTiki(context.Background(), tc.editingTiki); err != nil {
		slog.Error("failed to update tiki", "tikiID", tc.currentTikiID, "error", err)
		return fmt.Errorf("failed to update tiki: %w", err)
	}

	// Clear the edit session
	tc.editingTiki = nil
	tc.originalMtime = time.Time{}

	return nil
}

// GetActionRegistry returns the actions for the tiki detail view
func (tc *TikiEditSession) GetActionRegistry() *ActionRegistry {
	return tc.registry
}

// GetEditActionRegistry returns the actions for the tiki edit view
func (tc *TikiEditSession) GetEditActionRegistry() *ActionRegistry {
	return tc.editRegistry
}

// HandleAction processes tiki detail view actions such as editing title or source.
// Returns true if the action was handled, false otherwise.
func (tc *TikiEditSession) HandleAction(actionID ActionID) bool {
	switch actionID {
	case ActionEditTitle:
		return tc.handleEditTitle()
	case ActionEditSource:
		return tc.handleEditSource()
	case ActionCloneTiki:
		return tc.handleCloneTiki()
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
	filePath := tk.Path()
	if filePath == "" {
		filePath = filepath.Join(config.GetDocDir(), tk.ID()+".md")
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
	if err := tc.tikiStore.ReloadTiki(tk.ID()); err != nil && tc.statusline != nil {
		tc.statusline.SetMessage("reload failed: "+err.Error(), model.MessageLevelError, true)
	}

	return true
}

// SaveTitle saves the new title to the current tiki (draft or editing).
// For draft tikis (new tiki creation), updates the draft; for editing tikis, updates the editing copy.
// Returns true if a tiki was updated, false if no tiki is being edited.
func (tc *TikiEditSession) SaveTitle(newTitle string) bool {
	if tc.draftTiki != nil {
		tc.draftTiki.SetTitle(newTitle)
		return true
	}
	if tc.editingTiki != nil {
		tc.editingTiki.SetTitle(newTitle)
		return true
	}
	return false
}

// SaveDescription saves the new description to the current tiki (draft or editing).
// For draft tikis (new tiki creation), updates the draft; for editing tikis, updates the editing copy.
// Returns true if a tiki was updated, false if no tiki is being edited.
func (tc *TikiEditSession) SaveDescription(newDescription string) bool {
	if tc.draftTiki != nil {
		tc.draftTiki.SetBody(newDescription)
		return true
	}
	if tc.editingTiki != nil {
		tc.editingTiki.SetBody(newDescription)
		return true
	}
	return false
}

// updateTikiField updates a field in either the draft tiki or editing tiki.
// It applies the setter function to the appropriate tiki based on priority:
// draft tiki (new tiki creation) takes priority over editing tiki (existing tiki edit).
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

// SaveWorkflowEnum saves a value to a workflow-declared enum field. The value
// must be a valid key for the configured enum, or empty to delete the field.
// Returns true on success.
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

// SaveWorkflowField persists a raw editor string to a workflow-declared field
// by its declared type. It is the generic counterpart to SaveWorkflowEnum and
// the save authority the detail controller wires for editable fields without
// reserved-field handling. Empty raw clears the field. Integers are unbounded
// because the schema declares no per-field range. Returns false on parse failure
// so a malformed value is rejected rather than silently written.
func (tc *TikiEditSession) SaveWorkflowField(name, raw string) bool {
	wfd, ok := workflow.Field(name)
	if !ok {
		slog.Warn("SaveWorkflowField: unknown field", "field", name)
		return false
	}
	switch wfd.Type {
	case workflow.TypeEnum:
		return tc.SaveWorkflowEnum(name, raw)
	case workflow.TypeInt:
		return tc.saveWorkflowInt(name, raw)
	case workflow.TypeBool:
		return tc.saveWorkflowBool(name, raw)
	case workflow.TypeDate:
		return tc.saveWorkflowDate(name, raw)
	case workflow.TypeTimestamp:
		return tc.saveWorkflowTimestamp(name, raw)
	case workflow.TypeRecurrence:
		return tc.saveWorkflowRecurrence(name, raw)
	case workflow.TypeListString:
		return tc.saveWorkflowStringList(name, raw)
	case workflow.TypeString, workflow.TypeUser:
		return tc.setOrDelete(name, raw, raw == "")
	}
	slog.Warn("SaveWorkflowField: unsupported type", "field", name, "type", wfd.Type)
	return false
}

func (tc *TikiEditSession) saveWorkflowInt(name, raw string) bool {
	if raw == "" {
		return tc.setOrDelete(name, 0, true)
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		slog.Warn("saveWorkflowInt: not an integer", "field", name, "value", raw)
		return false
	}
	return tc.setOrDelete(name, n, false)
}

func (tc *TikiEditSession) saveWorkflowBool(name, raw string) bool {
	if raw == "" {
		return tc.setOrDelete(name, false, true)
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		slog.Warn("saveWorkflowBool: not a boolean", "field", name, "value", raw)
		return false
	}
	return tc.setOrDelete(name, b, false)
}

func (tc *TikiEditSession) saveWorkflowDate(name, raw string) bool {
	parsed, ok := value.ParseDate(raw)
	if !ok {
		slog.Warn("saveWorkflowDate: cannot parse", "field", name, "value", raw)
		return false
	}
	return tc.setOrDelete(name, parsed, parsed.IsZero())
}

func (tc *TikiEditSession) saveWorkflowTimestamp(name, raw string) bool {
	if raw == "" {
		return tc.setOrDelete(name, time.Time{}, true)
	}
	t, ok := value.ParseDateTime(raw)
	if !ok {
		slog.Warn("saveWorkflowTimestamp: cannot parse", "field", name, "value", raw)
		return false
	}
	return tc.setOrDelete(name, t, t.IsZero())
}

func (tc *TikiEditSession) saveWorkflowRecurrence(name, raw string) bool {
	r := recurrence.Recurrence(raw)
	if !recurrence.IsValidRecurrence(r) {
		slog.Warn("saveWorkflowRecurrence: invalid recurrence", "field", name, "value", raw)
		return false
	}
	return tc.setOrDelete(name, string(r), r == recurrence.RecurrenceNone)
}

func (tc *TikiEditSession) saveWorkflowStringList(name, raw string) bool {
	values := collectionutil.NormalizeStringSet(strings.Fields(raw))
	return tc.setOrDelete(name, values, len(values) == 0)
}

// setOrDelete deletes the field when clear is true, else sets it to fieldValue,
// on whichever tiki the session is currently editing (draft or existing copy).
func (tc *TikiEditSession) setOrDelete(name string, fieldValue interface{}, clear bool) bool {
	return tc.updateTikiField(func(tk *tikipkg.Tiki) {
		if clear {
			tk.Delete(name)
			return
		}
		tk.Set(name, fieldValue)
	})
}

func (tc *TikiEditSession) handleCloneTiki() bool {
	// TODO: trigger tiki clone flow from detail view
	return true
}

// GetCurrentTiki returns the tiki being viewed or edited.
// Returns nil if no tiki is currently active.
func (tc *TikiEditSession) GetCurrentTiki() *tikipkg.Tiki {
	if tc.currentTikiID == "" {
		return nil
	}
	return tc.tikiStore.GetTiki(tc.currentTikiID)
}

// GetCurrentTikiID returns the ID of the current tiki
func (tc *TikiEditSession) GetCurrentTikiID() string {
	return tc.currentTikiID
}

// GetFocusedField returns the currently focused field in edit mode
func (tc *TikiEditSession) GetFocusedField() model.EditField {
	return tc.focusedField
}

// SetFocusedField sets the currently focused field in edit mode
func (tc *TikiEditSession) SetFocusedField(field model.EditField) {
	tc.focusedField = field
}

// AddComment adds a new comment to the current tiki with the specified author and text.
// Returns false if no tiki is currently active, true if the comment was added successfully.
func (tc *TikiEditSession) AddComment(author, text string) bool {
	if tc.currentTikiID == "" {
		return false
	}

	tk := tc.tikiStore.GetTiki(tc.currentTikiID)
	if tk == nil {
		err := fmt.Errorf("tiki not found: %s", tc.currentTikiID)
		slog.Error("failed to add comment", "tikiID", tc.currentTikiID, "error", err)
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
