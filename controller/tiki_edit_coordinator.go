package controller

import (
	"errors"
	"strings"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/service"

	"github.com/gdamore/tcell/v2"
)

// TikiEditCoordinator owns tiki edit lifecycle: preparing the view, wiring handlers,
// and implementing commit/cancel and field navigation policy.
type TikiEditCoordinator struct {
	navController *NavigationController
	editSession   *TikiEditSession

	preparedView View
	descOnly     bool
	tagsOnly     bool
}

func NewTikiEditCoordinator(navController *NavigationController, editSession *TikiEditSession) *TikiEditCoordinator {
	return &TikiEditCoordinator{
		navController: navController,
		editSession:   editSession,
	}
}

// Prepare wires handlers and starts an edit session for the provided view instance.
// It is safe to call repeatedly; preparation is cached per active view instance.
func (c *TikiEditCoordinator) Prepare(activeView View, params model.TikiEditParams) {
	if activeView == nil {
		return
	}
	if c.preparedView == activeView {
		return
	}

	if params.TikiID != "" {
		c.editSession.SetCurrentTiki(params.TikiID)
	}
	if params.Draft != nil {
		c.editSession.SetDraft(params.Draft)
	} else {
		c.editSession.ClearDraft()
	}

	c.descOnly = params.DescOnly
	c.tagsOnly = params.TagsOnly
	c.prepareView(activeView, params.Focus)
	c.preparedView = activeView
}

func (c *TikiEditCoordinator) HandleKey(activeView View, event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyCtrlS:
		return c.handleSaveKey(activeView)
	case tcell.KeyTab:
		if c.descOnly || c.tagsOnly {
			return false // let textarea handle Tab as literal tab
		}
		return c.FocusNextField(activeView)
	case tcell.KeyBacktab:
		if c.descOnly || c.tagsOnly {
			return false
		}
		return c.FocusPrevField(activeView)
	case tcell.KeyLeft:
		if nav, ok := activeView.(RecurrencePartNavigable); ok {
			if nav.MoveRecurrencePartLeft() {
				c.updateFieldHint(activeView)
				return true
			}
		}
		return false
	case tcell.KeyRight:
		if nav, ok := activeView.(RecurrencePartNavigable); ok {
			if nav.MoveRecurrencePartRight() {
				c.updateFieldHint(activeView)
				return true
			}
		}
		return false
	case tcell.KeyEscape:
		return c.CancelAndClose()
	case tcell.KeyUp:
		return c.CycleFieldValueUp(activeView)
	case tcell.KeyDown:
		return c.CycleFieldValueDown(activeView)
	default:
		return false
	}
}

func (c *TikiEditCoordinator) FocusNextField(activeView View) bool {
	fieldFocusable, ok := activeView.(FieldFocusableView)
	if !ok {
		return false
	}
	result := fieldFocusable.FocusNextField()
	c.updateFieldHint(activeView)
	return result
}

func (c *TikiEditCoordinator) FocusPrevField(activeView View) bool {
	fieldFocusable, ok := activeView.(FieldFocusableView)
	if !ok {
		return false
	}
	result := fieldFocusable.FocusPrevField()
	c.updateFieldHint(activeView)
	return result
}

func (c *TikiEditCoordinator) CycleFieldValueUp(activeView View) bool {
	if cyclable, ok := activeView.(ValueCyclableView); ok {
		return cyclable.CycleFieldValueUp()
	}
	return false
}

func (c *TikiEditCoordinator) CycleFieldValueDown(activeView View) bool {
	if cyclable, ok := activeView.(ValueCyclableView); ok {
		return cyclable.CycleFieldValueDown()
	}
	return false
}

// TagsTextAreaSavable is the optional view-side hook used by handleSaveKey
// to dispatch Ctrl+S on a focused tags grid editor through the wired
// onTagsSave handler. The handler in turn drives CommitNoClose (in
// non-tagsOnly mode) so the metadata-grid tags edit doesn't pop the view.
type TagsTextAreaSavable interface {
	SaveTagsFromTextArea()
}

// DescriptionTextAreaSavable mirrors TagsTextAreaSavable for descriptions.
// In non-descOnly mode, Ctrl+S on a focused description editor triggers
// CommitNoClose via the wired handler instead of CommitAndClose.
type DescriptionTextAreaSavable interface {
	SaveDescriptionFromTextArea()
}

// handleSaveKey implements the field-aware Ctrl+S routing. Tags-only and
// desc-only modes commit-and-close as before; in normal grid mode,
// Ctrl+S on a focused tags or description editor invokes the matching
// SaveXFromTextArea hook (which fires the wired CommitNoClose handler);
// every other focused field falls through to the value-on-change pattern
// of CommitAndClose committing the whole session.
func (c *TikiEditCoordinator) handleSaveKey(activeView View) bool {
	if c.tagsOnly || c.descOnly {
		return c.CommitAndClose(activeView)
	}
	if focused, ok := activeView.(FieldFocusableView); ok {
		switch focused.GetFocusedField() {
		case model.EditFieldTags:
			if tv, ok := activeView.(TagsTextAreaSavable); ok {
				tv.SaveTagsFromTextArea()
				return true
			}
		case model.EditFieldDescription:
			if dv, ok := activeView.(DescriptionTextAreaSavable); ok {
				dv.SaveDescriptionFromTextArea()
				return true
			}
		}
	}
	return c.CommitAndClose(activeView)
}

func (c *TikiEditCoordinator) CommitAndClose(activeView View) bool {
	if !c.commit(activeView) {
		return false
	}
	c.clearFieldHint()
	c.navController.HandleBack()
	return true
}

func (c *TikiEditCoordinator) CommitNoClose(activeView View) bool {
	if !c.commit(activeView) {
		return false
	}

	// Re-start edit session with newly saved tiki
	tikiID := c.editSession.currentTikiID
	if editingTiki := c.editSession.StartEditSession(tikiID); editingTiki != nil {
		// Refresh view with new editing copy
		if refreshable, ok := activeView.(interface{ Refresh() }); ok {
			refreshable.Refresh()
		}
	}
	return true
}

func (c *TikiEditCoordinator) CancelAndClose() bool {
	// Cancel edit session (discards changes) and clear any draft.
	c.editSession.CancelEditSession()
	c.editSession.ClearDraft()
	c.clearFieldHint()
	c.navController.HandleBack()
	return true
}

func (c *TikiEditCoordinator) commit(activeView View) bool {
	editorView, ok := activeView.(TikiEditView)
	if !ok {
		return false
	}

	// Check validation state - do not save if invalid
	if validator, ok := activeView.(interface {
		IsValid() bool
		ValidationErrors() []string
	}); ok {
		if !validator.IsValid() {
			if sl := c.editSession.statusline; sl != nil {
				if errs := validator.ValidationErrors(); len(errs) > 0 {
					sl.SetMessage(strings.Join(errs, "; "), model.MessageLevelError, true)
				}
			}
			return false
		}
	}

	// Update in-memory editing copy with latest widget values
	c.editSession.SaveTitle(editorView.GetEditedTitle())
	c.editSession.SaveDescription(editorView.GetEditedDescription())
	c.editSession.SaveTags(editorView.GetEditedTags())

	// Commit the edit session (writes to disk)
	if err := c.editSession.CommitEditSession(); err != nil {
		if sl := c.editSession.statusline; sl != nil {
			sl.SetMessage(rejectionMessage(err), model.MessageLevelError, true)
		}
		return false
	}
	return true
}

// updateFieldHint shows or clears a statusline hint based on the focused field.
func (c *TikiEditCoordinator) updateFieldHint(activeView View) {
	sl := c.editSession.statusline
	if sl == nil {
		return
	}
	fieldFocusable, ok := activeView.(FieldFocusableView)
	if !ok {
		return
	}
	switch fieldFocusable.GetFocusedField() {
	case model.EditFieldStatus, model.EditFieldType, model.EditFieldPriority,
		model.EditFieldAssignee, model.EditFieldPoints, model.EditFieldDue:
		sl.SetMessage("↑↓ change value", model.MessageLevelInfo, false)
	case model.EditFieldRecurrence:
		if nav, ok := activeView.(RecurrencePartNavigable); ok && nav.IsRecurrenceValueFocused() {
			sl.SetMessage("← edit pattern  ↑↓ change value", model.MessageLevelInfo, false)
		} else {
			sl.SetMessage("↑↓ change pattern  → edit value", model.MessageLevelInfo, false)
		}
	default:
		sl.ClearMessage()
	}
}

// clearFieldHint removes any statusline hint set by updateFieldHint.
func (c *TikiEditCoordinator) clearFieldHint() {
	if sl := c.editSession.statusline; sl != nil {
		sl.ClearMessage()
	}
}

func (c *TikiEditCoordinator) prepareView(activeView View, focus model.EditField) {
	app := c.navController.GetApp()

	// Start edit session for existing tikis (creates in-memory copy)
	// Draft tikis already have an in-memory copy via draftTiki
	if _, ok := activeView.(TikiEditView); ok {
		if tikiView, hasController := activeView.(interface{ SetTikiEditSession(*TikiEditSession) }); hasController {
			tikiView.SetTikiEditSession(c.editSession)
		}

		// Only start edit session for non-draft tikis
		if c.editSession.draftTiki == nil {
			tikiID := c.editSession.currentTikiID
			if tikiID != "" {
				c.editSession.StartEditSession(tikiID)
			}
		}
	}

	if titleEditableView, ok := activeView.(TitleEditableView); ok && !c.descOnly {
		// Explicit save on Enter (commits and closes)
		titleEditableView.SetTitleSaveHandler(func(_ string) {
			c.CommitAndClose(activeView)
		})
		titleEditableView.SetTitleCancelHandler(func() {
			c.CancelAndClose()
		})
	}

	if descEditableView, ok := activeView.(DescriptionEditableView); ok {
		if c.descOnly {
			// in desc-only mode, Ctrl+S from description saves and exits
			descEditableView.SetDescriptionSaveHandler(func(_ string) {
				c.CommitAndClose(activeView)
			})
		} else {
			descEditableView.SetDescriptionSaveHandler(func(_ string) {
				c.CommitNoClose(activeView)
			})
		}
		descEditableView.SetDescriptionCancelHandler(func() {
			c.CancelAndClose()
		})
	}

	if tagsEditableView, ok := activeView.(TagsEditableView); ok {
		if c.tagsOnly {
			tagsEditableView.SetTagsSaveHandler(func(_ string) {
				c.CommitAndClose(activeView)
			})
		} else {
			// metadata-grid tags edit: save into the in-flight session
			// without popping the view so the user can keep editing.
			tagsEditableView.SetTagsSaveHandler(func(_ string) {
				c.CommitNoClose(activeView)
			})
		}
		tagsEditableView.SetTagsCancelHandler(func() {
			c.CancelAndClose()
		})
	}

	if !c.descOnly {
		if statusEditableView, ok := activeView.(StatusEditableView); ok {
			statusEditableView.SetStatusSaveHandler(func(statusDisplay string) {
				c.editSession.SaveStatus(statusDisplay)
			})
		}

		if typeEditableView, ok := activeView.(TypeEditableView); ok {
			typeEditableView.SetTypeSaveHandler(func(typeDisplay string) {
				c.editSession.SaveType(typeDisplay)
			})
		}

		if priorityEditableView, ok := activeView.(PriorityEditableView); ok {
			priorityEditableView.SetPrioritySaveHandler(func(priority string) {
				c.editSession.SavePriority(priority)
			})
		}

		if assigneeEditableView, ok := activeView.(AssigneeEditableView); ok {
			assigneeEditableView.SetAssigneeSaveHandler(func(assignee string) {
				c.editSession.SaveAssignee(assignee)
			})
		}

		if pointsEditableView, ok := activeView.(PointsEditableView); ok {
			pointsEditableView.SetPointsSaveHandler(func(points int) {
				c.editSession.SavePoints(points)
			})
		}

		if dueEditableView, ok := activeView.(DueEditableView); ok {
			dueEditableView.SetDueSaveHandler(func(dateStr string) {
				c.editSession.SaveDue(dateStr)
			})
		}

		if recurrenceEditableView, ok := activeView.(RecurrenceEditableView); ok {
			recurrenceEditableView.SetRecurrenceSaveHandler(func(cron string) {
				c.editSession.SaveRecurrence(cron)
			})
		}

		if workflowEnumView, ok := activeView.(WorkflowEnumEditableView); ok {
			workflowEnumView.SetWorkflowEnumSaveHandler(func(name, canonicalKey string) {
				c.editSession.SaveWorkflowEnum(name, canonicalKey)
			})
		}
	}

	// In tags-only mode, skip title focus entirely — go straight to tags textarea
	if c.tagsOnly {
		if tagsEditableView, ok := activeView.(TagsEditableView); ok {
			if tags := tagsEditableView.ShowTagsEditor(); tags != nil {
				app.SetFocus(tags)
				return
			}
		}
		return
	}

	// In desc-only mode, skip title focus entirely — go straight to description
	if c.descOnly {
		if fieldFocusable, ok := activeView.(FieldFocusableView); ok {
			fieldFocusable.SetFocusedField(model.EditFieldDescription)
		}
		if descEditableView, ok := activeView.(DescriptionEditableView); ok {
			if desc := descEditableView.ShowDescriptionEditor(); desc != nil {
				app.SetFocus(desc)
				return
			}
		}
		return
	}

	// Initialize with title field focused by default (or specified focus field)
	if fieldFocusable, ok := activeView.(FieldFocusableView); ok {
		fieldFocusable.SetFocusedField(model.EditFieldTitle)
	}

	if focus == model.EditFieldDescription {
		if fieldFocusable, ok := activeView.(FieldFocusableView); ok {
			fieldFocusable.SetFocusedField(model.EditFieldDescription)
		}
		if descEditableView, ok := activeView.(DescriptionEditableView); ok {
			if desc := descEditableView.ShowDescriptionEditor(); desc != nil {
				app.SetFocus(desc)
				return
			}
		}
	}

	if titleEditableView, ok := activeView.(TitleEditableView); ok {
		if title := titleEditableView.ShowTitleEditor(); title != nil {
			app.SetFocus(title)
			return
		}
	}

	// fallback to next field focus cycle
	_ = c.FocusNextField(activeView)
}

// rejectionMessage extracts a clean user-facing message from an error.
// For RejectionError: returns just the rejection reasons.
// For other errors: unwraps to the root cause to strip wrapper prefixes
// like "failed to update tiki: failed to save tiki:".
func rejectionMessage(err error) string {
	var re *service.RejectionError
	if errors.As(err, &re) {
		reasons := make([]string, len(re.Rejections))
		for i, r := range re.Rejections {
			reasons[i] = r.Reason
		}
		return strings.Join(reasons, "; ")
	}
	// unwrap to the innermost error for a clean message
	for {
		inner := errors.Unwrap(err)
		if inner == nil {
			break
		}
		err = inner
	}
	return err.Error()
}
