package taskdetail

import (
	"fmt"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"

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

	text := fmt.Sprintf("%s%sStatus:   %s%s", focusMarker, labelColor, valueColor, statusDisplay)
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

	text := fmt.Sprintf("%s%sType:     %s%s", focusMarker, labelColor, valueColor, typeDisplay)
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

	text := fmt.Sprintf("%s%sPriority: %s%d", focusMarker, labelColor, valueColor, task.Priority)
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

	text := fmt.Sprintf("%s%sAssignee: %s%s", focusMarker, labelColor, valueColor, defaultString(task.Assignee, "Unassigned"))
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

	text := fmt.Sprintf("%s%sPoints:  %s%d", focusMarker, labelColor, valueColor, task.Points)
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderTitleText renders a title as read-only text
func RenderTitleText(task *taskpkg.Task, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldTitle
	titleColor := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailTitleText[:len(ctx.Colors.TaskDetailTitleText)-1]+"::b]", ctx.Colors.TaskDetailEditDimTextColor)
	titleText := fmt.Sprintf("%s%s%s", titleColor, task.Title, ctx.Colors.TaskDetailValueText)
	titleBox := tview.NewTextView().
		SetDynamicColors(true).
		SetText(titleText)
	titleBox.SetBorderPadding(0, 0, 0, 0)
	return titleBox
}

// RenderTagsColumn renders the tags column
func RenderTagsColumn(task *taskpkg.Task) tview.Primitive {
	if len(task.Tags) > 0 {
		wordList := component.NewWordList(task.Tags)
		return wordList
	}
	return tview.NewBox()
}

// RenderMetadataColumn3 renders the third metadata column (Author, Created, Updated)
func RenderMetadataColumn3(task *taskpkg.Task, colors *config.ColorConfig) *tview.Flex {
	createdAtStr := "Unknown"
	if !task.CreatedAt.IsZero() {
		createdAtStr = task.CreatedAt.Format("2006-01-02 15:04")
	}

	updatedAtStr := "Unknown"
	if !task.UpdatedAt.IsZero() {
		updatedAtStr = task.UpdatedAt.Format("2006-01-02 15:04")
	}

	authorText := fmt.Sprintf("%sAuthor:  %s%s",
		colors.TaskDetailEditDimLabelColor, colors.TaskDetailValueText, defaultString(task.CreatedBy, "Unknown"))
	authorView := tview.NewTextView().SetDynamicColors(true).SetText(authorText)
	authorView.SetBorderPadding(0, 0, 0, 0)

	createdText := fmt.Sprintf("%sCreated: %s%s",
		colors.TaskDetailEditDimLabelColor, colors.TaskDetailValueText, createdAtStr)
	createdView := tview.NewTextView().SetDynamicColors(true).SetText(createdText)
	createdView.SetBorderPadding(0, 0, 0, 0)

	updatedText := fmt.Sprintf("%sUpdated: %s%s",
		colors.TaskDetailEditDimLabelColor, colors.TaskDetailValueText, updatedAtStr)
	updatedView := tview.NewTextView().SetDynamicColors(true).SetText(updatedText)
	updatedView.SetBorderPadding(0, 0, 0, 0)

	col3 := tview.NewFlex().SetDirection(tview.FlexRow)
	col3.AddItem(authorView, 1, 0, false)
	col3.AddItem(createdView, 1, 0, false)
	col3.AddItem(updatedView, 1, 0, false)

	return col3
}
