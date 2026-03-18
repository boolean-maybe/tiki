package taskdetail

import (
	"fmt"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RenderMode indicates whether we're rendering for view or edit mode
type RenderMode int

const (
	RenderModeView RenderMode = iota
	RenderModeEdit
)

// FieldRenderContext provides context for rendering field primitives
type FieldRenderContext struct {
	Mode         RenderMode
	FocusedField model.EditField
	Colors       *config.ColorConfig
}

// getDimOrFullColor returns dim color if in edit mode and not focused, otherwise full color
func getDimOrFullColor(mode RenderMode, focused bool, fullColor string, dimColor string) string {
	if mode == RenderModeEdit && !focused {
		return dimColor
	}
	return fullColor
}

// getFocusMarker returns the focus marker string (arrow + text color) from colors config
func getFocusMarker(colors *config.ColorConfig) string {
	return colors.TaskDetailEditFocusMarker + "► " + colors.TaskDetailEditFocusText
}

// RenderStatusText renders a status field as read-only text
func RenderStatusText(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldStatus
	statusDisplay := taskpkg.StatusDisplay(task.Status)

	labelColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor)
	valueColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor)

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelColor, "Status:", valueColor, statusDisplay)
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderTypeText renders a type field as read-only text
func RenderTypeText(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldType
	typeDisplay := taskpkg.TypeDisplay(task.Type)
	if task.Type == "" {
		typeDisplay = "[gray](none)[-]"
	}

	labelColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor)
	valueColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor)

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelColor, "Type:", valueColor, typeDisplay)
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderPriorityText renders a priority field as read-only text
func RenderPriorityText(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldPriority

	labelColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor)
	valueColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor)

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelColor, "Priority:", valueColor, taskpkg.PriorityDisplay(task.Priority))
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderAssigneeText renders an assignee field as read-only text
func RenderAssigneeText(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldAssignee

	labelColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor)
	valueColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor)

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelColor, "Assignee:", valueColor, tview.Escape(defaultString(task.Assignee, "Unassigned")))
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderPointsText renders a points field as read-only text
func RenderPointsText(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldPoints

	labelColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor)
	valueColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor)

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%d", focusMarker, labelColor, "Points:", valueColor, task.Points)
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderTitleText renders a title as read-only text
func RenderTitleText(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldTitle
	titleColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailTitleText[:len(ctx.Colors.TaskDetailTitleText)-1]+"::b]", ctx.Colors.TaskDetailEditDimTextColor)
	titleText := fmt.Sprintf("%s%s%s", titleColor, tview.Escape(task.Title), ctx.Colors.TaskDetailValueText)
	titleBox := tview.NewTextView().
		SetDynamicColors(true).
		SetText(titleText)
	titleBox.SetBorderPadding(0, 0, 0, 0)
	return titleBox
}

// RenderTagsColumn renders the tags column with a label row on top.
func RenderTagsColumn(task *taskpkg.Task) tview.Primitive {
	if len(task.Tags) == 0 {
		return tview.NewBox()
	}
	colors := config.GetColors()
	label := tview.NewTextView().SetDynamicColors(true).SetText(fmt.Sprintf("%sTags", colors.TaskDetailLabelText))
	label.SetBorderPadding(0, 0, 0, 0)

	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.AddItem(label, 1, 0, false)
	col.AddItem(component.NewWordList(task.Tags), 0, 1, false)
	return col
}

// RenderDependsOnColumn renders the "Depends On" column showing upstream dependencies.
// Returns nil when the task has no dependencies, so the caller can skip adding it.
func RenderDependsOnColumn(task *taskpkg.Task, taskStore store.Store) tview.Primitive {
	if len(task.DependsOn) == 0 {
		return nil
	}
	var resolved []*taskpkg.Task
	for _, id := range task.DependsOn {
		if t := taskStore.GetTask(id); t != nil {
			resolved = append(resolved, t)
		}
	}
	if len(resolved) == 0 {
		return nil
	}

	colors := config.GetColors()
	label := tview.NewTextView().SetDynamicColors(true).SetText(fmt.Sprintf("%sDepends On", colors.TaskDetailLabelText))
	label.SetBorderPadding(0, 0, 0, 0)

	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.SetBorderPadding(0, 0, 1, 1)
	col.AddItem(label, 1, 0, false)
	col.AddItem(component.NewTaskList(config.TaskListMetadataMaxRows).SetTasks(resolved), 0, 1, false)
	return col
}

// RenderBlocksColumn renders the "Blocks" column showing downstream dependents.
// Returns nil when blocked is empty, so the caller can skip adding it.
func RenderBlocksColumn(blocked []*taskpkg.Task) tview.Primitive {
	if len(blocked) == 0 {
		return nil
	}

	colors := config.GetColors()
	label := tview.NewTextView().SetDynamicColors(true).SetText(fmt.Sprintf("%sBlocks", colors.TaskDetailLabelText))
	label.SetBorderPadding(0, 0, 0, 0)

	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.SetBorderPadding(0, 0, 1, 1)
	col.AddItem(label, 1, 0, false)
	col.AddItem(component.NewTaskList(config.TaskListMetadataMaxRows).SetTasks(blocked), 0, 1, false)
	return col
}

// RenderAuthorText renders the author field as read-only text
func RenderAuthorText(task *taskpkg.Task, colors *config.ColorConfig) tview.Primitive {
	text := fmt.Sprintf("%s%-10s%s%s",
		colors.TaskDetailEditDimLabelColor, "Author:", colors.TaskDetailValueText, tview.Escape(defaultString(task.CreatedBy, "Unknown")))
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderCreatedText renders the created-at field as read-only text
func RenderCreatedText(task *taskpkg.Task, colors *config.ColorConfig) tview.Primitive {
	createdAtStr := "Unknown"
	if !task.CreatedAt.IsZero() {
		createdAtStr = task.CreatedAt.Format("2006-01-02 15:04")
	}
	text := fmt.Sprintf("%s%-10s%s%s",
		colors.TaskDetailEditDimLabelColor, "Created:", colors.TaskDetailValueText, createdAtStr)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderUpdatedText renders the updated-at field as read-only text
func RenderUpdatedText(task *taskpkg.Task, colors *config.ColorConfig) tview.Primitive {
	updatedAtStr := "Unknown"
	if !task.UpdatedAt.IsZero() {
		updatedAtStr = task.UpdatedAt.Format("2006-01-02 15:04")
	}
	text := fmt.Sprintf("%s%-10s%s%s",
		colors.TaskDetailEditDimLabelColor, "Updated:", colors.TaskDetailValueText, updatedAtStr)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// responsiveMetadataRow is a tview.Flex that recalculates its layout when the
// terminal width changes, using the pure CalculateMetadataLayout algorithm.
type responsiveMetadataRow struct {
	*tview.Flex
	lastWidth  int
	inputs     []SectionInput
	primitives map[SectionID]tview.Primitive
}

// newResponsiveMetadataRow creates a responsive row from section inputs and their
// pre-rendered primitives.
func newResponsiveMetadataRow(inputs []SectionInput, primitives map[SectionID]tview.Primitive) *responsiveMetadataRow {
	r := &responsiveMetadataRow{
		Flex:       tview.NewFlex().SetDirection(tview.FlexColumn),
		inputs:     inputs,
		primitives: primitives,
	}
	return r
}

// Draw overrides Flex.Draw to detect width changes and rebuild the layout.
func (r *responsiveMetadataRow) Draw(screen tcell.Screen) {
	_, _, width, _ := r.GetRect()
	if width != r.lastWidth {
		r.rebuild(width)
	}
	r.Flex.Draw(screen)
}

func (r *responsiveMetadataRow) rebuild(width int) {
	r.lastWidth = width
	r.Clear()

	plan := CalculateMetadataLayout(width, r.inputs)
	for i, s := range plan.Sections {
		p, ok := r.primitives[s.ID]
		if !ok || p == nil {
			p = tview.NewBox()
		}
		r.AddItem(p, s.Width, 0, false)
		if i < len(plan.Gaps) {
			r.AddItem(tview.NewBox(), plan.Gaps[i], 0, false)
		}
	}
}

// tagsMinWidth computes the minimum width for the Tags section based on the
// longest individual tag. Since WordList wraps tags across multiple rows, the
// true minimum is the width of the widest tag (or the "Tags" label, whichever
// is larger), with a floor of 5 to avoid degenerate layouts.
func tagsMinWidth(tags []string) int {
	const label = "Tags"
	const floor = 5
	longest := len(label)
	for _, tag := range tags {
		if len(tag) > longest {
			longest = len(tag)
		}
	}
	return max(longest, floor)
}

// BuildSectionInputs creates the standard section input list for a task,
// using the constants from config/dimensions.go.
func BuildSectionInputs(task *taskpkg.Task, hasBlocks bool) []SectionInput {
	hasDue := !task.Due.IsZero() || task.Recurrence != ""
	return []SectionInput{
		{ID: SectionStatusGroup, Width: config.MetadataSectionMinWidth, HasContent: true},
		{ID: SectionPeopleGroup, Width: config.MetadataSectionMinWidth, HasContent: true},
		{ID: SectionDueGroup, Width: config.MetadataSectionMinWidth, HasContent: hasDue},
		{ID: SectionTags, Width: tagsMinWidth(task.Tags), HasContent: len(task.Tags) > 0},
		{ID: SectionDependsOn, Width: config.MetadataDepMinWidth, HasContent: len(task.DependsOn) > 0},
		{ID: SectionBlocks, Width: config.MetadataBlkMinWidth, HasContent: hasBlocks},
	}
}

// RenderDueText renders the due date field
func RenderDueText(task *taskpkg.Task, colors *config.ColorConfig) tview.Primitive {
	dueDisplay := "None"
	if !task.Due.IsZero() {
		dueDisplay = task.Due.Format("2006-01-02")
	}
	text := fmt.Sprintf("%s%-12s%s%s",
		colors.TaskDetailEditDimLabelColor, "Due:", colors.TaskDetailValueText, dueDisplay)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderRecurrenceText renders the recurrence field
func RenderRecurrenceText(task *taskpkg.Task, colors *config.ColorConfig) tview.Primitive {
	display := taskpkg.RecurrenceDisplay(task.Recurrence)
	text := fmt.Sprintf("%s%-12s%s%s",
		colors.TaskDetailEditDimLabelColor, "Recurrence:", colors.TaskDetailValueText, display)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}
