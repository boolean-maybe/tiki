package taskdetail

import (
	"fmt"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
	"github.com/boolean-maybe/tiki/workflow/value"

	"github.com/rivo/tview"
)

// expandFieldText escapes a user-controlled text value against tview's
// `[...]` dynamic-color markup, then expands any `{role}` markup against
// the active theme. Order matters: escape first so a stored `[red]`
// stays inert; then run ExpandVisual so deliberately-authored
// `{highlight}foo` resolves to a tview color tag. Fails closed to the
// plain escaped form on parse error (e.g. unclosed `{` or unknown role
// name) so bad stored data can never crash a render. Returns just the
// escaped form when colors is nil, supporting fallback call sites that
// lack a color config.
func expandFieldText(raw string, colors *config.ColorConfig) string {
	escaped := tview.Escape(raw)
	if colors == nil {
		return escaped
	}
	expanded, err := workflow.ExpandVisual(escaped, colors.RoleResolver())
	if err != nil {
		return escaped
	}
	return expanded
}

// RenderMode indicates whether we're rendering for view or edit mode
type RenderMode int

const (
	RenderModeView RenderMode = iota
	RenderModeEdit
)

// FieldRenderContext provides context for rendering field primitives.
// FieldName is set by renderConfiguredField so generic renderers can
// resolve the descriptor and avoid hardcoding the field they target.
// Store is set once per refresh by the host view so list renderers can
// resolve dependency tikis without globals.
type FieldRenderContext struct {
	Mode         RenderMode
	FocusedField model.EditField
	Colors       *config.ColorConfig
	FieldName    string
	Store        store.Store
}

// getDimOrFullColor returns dim color if in edit mode and not focused, otherwise full color
func getDimOrFullColor(mode RenderMode, focused bool, fullColor config.Color, dimColor config.Color) config.Color {
	if mode == RenderModeEdit && !focused {
		return dimColor
	}
	return fullColor
}

// getFocusMarker returns the focus marker string (arrow + text color) from colors config
func getFocusMarker(colors *config.ColorConfig) string {
	return colors.TaskDetailEditFocusMarker.Tag().String() + "► " + colors.TaskDetailEditFocusText.Tag().String()
}

// RenderAssigneeText renders an assignee field as read-only text. The
// stored value may contain `{role}` color markup; literal `[...]` content
// renders verbatim (escape-then-expand order).
func RenderAssigneeText(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldAssignee
	assignee, _, _ := tk.StringField(tikipkg.FieldAssignee)

	labelTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor).Tag().String()
	valueTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor).Tag().String()

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelTag, "Assignee:", valueTag, expandFieldText(defaultString(assignee, "Unassigned"), ctx.Colors))
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderPointsText renders a points field as read-only text. Points is now
// a workflow enum (e.g. 1/3/7/11); the stored value is a string key, shown
// here verbatim alongside the field label.
func RenderPointsText(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldPoints
	points, _, _ := tk.StringField(tikipkg.FieldPoints)

	labelTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor).Tag().String()
	valueTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor).Tag().String()

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelTag, "Points:", valueTag, tview.Escape(points))
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderTitleText renders a title as read-only text. The stored title may
// contain `{role}` color markup (e.g. `{highlight}foo`); literal `[...]`
// tview-tag content renders verbatim.
func RenderTitleText(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldTitle
	var titleTag string
	if ctx.Mode == RenderModeEdit && !focused {
		titleTag = ctx.Colors.TaskDetailEditDimTextColor.Tag().String()
	} else {
		titleTag = ctx.Colors.TaskDetailTitleText.Tag().Bold().String()
	}
	valueTag := ctx.Colors.TaskDetailValueText.Tag().String()
	titleText := fmt.Sprintf("%s%s%s", titleTag, expandFieldText(tk.Title, ctx.Colors), valueTag)
	titleBox := tview.NewTextView().
		SetDynamicColors(true).
		SetText(titleText)
	titleBox.SetBorderPadding(0, 0, 0, 0)
	return titleBox
}

// RenderTagsColumn renders the tags as a value-only word-wrapped list. The
// caption (if wanted) is placed by the layout author as a literal cell.
func RenderTagsColumn(tk *tikipkg.Tiki) tview.Primitive {
	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	if len(tags) == 0 {
		return tview.NewBox()
	}
	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.SetBorderPadding(0, 0, 1, 1)
	col.AddItem(component.NewWordList(tags), 0, 1, false)
	return col
}

// RenderDependsOnColumn renders the upstream dependencies as a value-only
// task list. Returns nil when the task has no dependencies. Unresolved IDs
// (declared but not in the store) render as placeholder rows carrying the
// raw ID as a synthetic tiki — keeps the rendered row count in lockstep
// with the height contract so the grid algorithm doesn't reserve dead rows.
func RenderDependsOnColumn(tk *tikipkg.Tiki, taskStore store.Store) tview.Primitive {
	deps, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn)
	if len(deps) == 0 {
		return nil
	}
	rendered := make([]*tikipkg.Tiki, 0, len(deps))
	for _, id := range deps {
		if dep := taskStore.GetTiki(id); dep != nil {
			rendered = append(rendered, dep)
			continue
		}
		rendered = append(rendered, &tikipkg.Tiki{ID: id, Title: "(unresolved)"})
	}

	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.SetBorderPadding(0, 0, 1, 1)
	col.AddItem(component.NewTaskList(config.TaskListMetadataMaxRows).SetTasks(rendered), 0, 1, false)
	return col
}

// RenderBlocksColumn renders the "Blocks" column showing downstream dependents.
// Returns nil when blocked is empty, so the caller can skip adding it.
func RenderBlocksColumn(blocked []*tikipkg.Tiki) tview.Primitive {
	if len(blocked) == 0 {
		return nil
	}

	colors := config.GetColors()
	label := tview.NewTextView().SetDynamicColors(true).SetText(fmt.Sprintf("%sBlocks", colors.TaskDetailLabelText.Tag().String()))
	label.SetBorderPadding(0, 0, 0, 0)

	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.SetBorderPadding(0, 0, 1, 1)
	col.AddItem(label, 1, 0, false)
	col.AddItem(component.NewTaskList(config.TaskListMetadataMaxRows).SetTasks(blocked), 0, 1, false)
	return col
}

// RenderAuthorText renders the author field as read-only text
func RenderAuthorText(tk *tikipkg.Tiki, colors *config.ColorConfig) tview.Primitive {
	createdBy, _, _ := tk.StringField("createdBy")
	text := fmt.Sprintf("%s%-10s%s%s",
		colors.TaskDetailEditDimLabelColor.Tag().String(), "Author:", colors.TaskDetailValueText.Tag().String(), tview.Escape(defaultString(createdBy, "Unknown")))
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderCreatedText renders the created-at field as read-only text
func RenderCreatedText(tk *tikipkg.Tiki, colors *config.ColorConfig) tview.Primitive {
	createdAtStr := "Unknown"
	if !tk.CreatedAt.IsZero() {
		createdAtStr = tk.CreatedAt.Format("2006-01-02 15:04")
	}
	text := fmt.Sprintf("%s%-10s%s%s",
		colors.TaskDetailEditDimLabelColor.Tag().String(), "Created:", colors.TaskDetailValueText.Tag().String(), createdAtStr)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderUpdatedText renders the updated-at field as read-only text
func RenderUpdatedText(tk *tikipkg.Tiki, colors *config.ColorConfig) tview.Primitive {
	updatedAtStr := "Unknown"
	if !tk.UpdatedAt.IsZero() {
		updatedAtStr = tk.UpdatedAt.Format("2006-01-02 15:04")
	}
	text := fmt.Sprintf("%s%-10s%s%s",
		colors.TaskDetailEditDimLabelColor.Tag().String(), "Updated:", colors.TaskDetailValueText.Tag().String(), updatedAtStr)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderDueText renders the due date field
func RenderDueText(tk *tikipkg.Tiki, colors *config.ColorConfig) tview.Primitive {
	due, _, _ := tk.TimeField(tikipkg.FieldDue)
	dueDisplay := "None"
	if !due.IsZero() {
		dueDisplay = due.Format("2006-01-02")
	}
	text := fmt.Sprintf("%s%-12s%s%s",
		colors.TaskDetailEditDimLabelColor.Tag().String(), "Due:", colors.TaskDetailValueText.Tag().String(), dueDisplay)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderRecurrenceText renders the recurrence field
func RenderRecurrenceText(tk *tikipkg.Tiki, colors *config.ColorConfig) tview.Primitive {
	recurrenceStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
	display := value.RecurrenceDisplay(value.Recurrence(recurrenceStr))
	text := fmt.Sprintf("%s%-12s%s%s",
		colors.TaskDetailEditDimLabelColor.Tag().String(), "Recurrence:", colors.TaskDetailValueText.Tag().String(), display)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}
