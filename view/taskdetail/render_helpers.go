package taskdetail

import (
	"fmt"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

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

// RenderStatusText renders a status field as read-only text
func RenderStatusText(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldStatus
	statusStr, _, _ := tk.StringField(tikipkg.FieldStatus)
	statusDisplay := taskpkg.StatusDisplay(taskpkg.Status(statusStr))

	labelTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor).Tag().String()
	valueTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor).Tag().String()

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelTag, "Status:", valueTag, statusDisplay)
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderTypeText renders a type field as read-only text
func RenderTypeText(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldType
	typeStr, _, _ := tk.StringField(tikipkg.FieldType)
	typeDisplay := taskpkg.TypeDisplay(taskpkg.Type(typeStr))
	if typeStr == "" {
		typeDisplay = ctx.Colors.TaskDetailPlaceholderColor.Tag().String() + "(none)[-]"
	}

	labelTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor).Tag().String()
	valueTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor).Tag().String()

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelTag, "Type:", valueTag, typeDisplay)
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderPriorityText renders a priority field as read-only text
func RenderPriorityText(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldPriority
	priority, present, _ := tk.IntField(tikipkg.FieldPriority)

	labelTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor).Tag().String()
	valueTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor).Tag().String()

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	priorityStr := "─"
	if present {
		priorityStr = taskpkg.PriorityDisplay(priority)
	}
	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelTag, "Priority:", valueTag, priorityStr)
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderAssigneeText renders an assignee field as read-only text
func RenderAssigneeText(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldAssignee
	assignee, _, _ := tk.StringField(tikipkg.FieldAssignee)

	labelTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor).Tag().String()
	valueTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor).Tag().String()

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelTag, "Assignee:", valueTag, tview.Escape(defaultString(assignee, "Unassigned")))
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderPointsText renders a points field as read-only text
func RenderPointsText(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldPoints
	points, _, _ := tk.IntField(tikipkg.FieldPoints)

	labelTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailLabelText, ctx.Colors.TaskDetailEditDimLabelColor).Tag().String()
	valueTag := getDimOrFullColor(ctx.Mode, focused, ctx.Colors.TaskDetailValueText, ctx.Colors.TaskDetailEditDimValueColor).Tag().String()

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Colors)
	}

	text := fmt.Sprintf("%s%s%-10s%s%d", focusMarker, labelTag, "Points:", valueTag, points)
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderTitleText renders a title as read-only text
func RenderTitleText(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldTitle
	var titleTag string
	if ctx.Mode == RenderModeEdit && !focused {
		titleTag = ctx.Colors.TaskDetailEditDimTextColor.Tag().String()
	} else {
		titleTag = ctx.Colors.TaskDetailTitleText.Tag().Bold().String()
	}
	valueTag := ctx.Colors.TaskDetailValueText.Tag().String()
	titleText := fmt.Sprintf("%s%s%s", titleTag, tview.Escape(tk.Title), valueTag)
	titleBox := tview.NewTextView().
		SetDynamicColors(true).
		SetText(titleText)
	titleBox.SetBorderPadding(0, 0, 0, 0)
	return titleBox
}

// RenderTagsColumn renders the tags column with a label row on top.
func RenderTagsColumn(tk *tikipkg.Tiki) tview.Primitive {
	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	if len(tags) == 0 {
		return tview.NewBox()
	}
	colors := config.GetColors()
	label := tview.NewTextView().SetDynamicColors(true).SetText(fmt.Sprintf("%sTags", colors.TaskDetailLabelText.Tag().String()))
	label.SetBorderPadding(0, 0, 0, 0)

	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.SetBorderPadding(0, 0, 1, 1)
	col.AddItem(label, 1, 0, false)
	col.AddItem(component.NewWordList(tags), 0, 1, false)
	return col
}

// RenderDependsOnColumn renders the "Depends On" column showing upstream dependencies.
// Returns nil when the task has no dependencies, so the caller can skip adding it.
func RenderDependsOnColumn(tk *tikipkg.Tiki, taskStore store.Store) tview.Primitive {
	deps, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn)
	if len(deps) == 0 {
		return nil
	}
	var resolved []*tikipkg.Tiki
	for _, id := range deps {
		if dep := taskStore.GetTiki(id); dep != nil {
			resolved = append(resolved, dep)
		}
	}
	if len(resolved) == 0 {
		return nil
	}

	colors := config.GetColors()
	label := tview.NewTextView().SetDynamicColors(true).SetText(fmt.Sprintf("%sDepends On", colors.TaskDetailLabelText.Tag().String()))
	label.SetBorderPadding(0, 0, 0, 0)

	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.SetBorderPadding(0, 0, 1, 1)
	col.AddItem(label, 1, 0, false)
	col.AddItem(component.NewTaskList(config.TaskListMetadataMaxRows).SetTasks(resolved), 0, 1, false)
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
	const floor = 7
	longest := len(label)
	for _, tag := range tags {
		if len(tag) > longest {
			longest = len(tag)
		}
	}
	return max(longest, floor)
}

// BuildSectionInputs creates the standard section input list for a tiki,
// using the constants from config/dimensions.go.
func BuildSectionInputs(tk *tikipkg.Tiki, hasBlocks bool) []SectionInput {
	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	deps, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn)
	return []SectionInput{
		{ID: SectionStatusGroup, Width: config.MetadataSectionMinWidth, HasContent: true},
		{ID: SectionPeopleGroup, Width: config.MetadataSectionMinWidth, HasContent: true},
		{ID: SectionDueGroup, Width: config.MetadataSectionMinWidth, HasContent: true},
		{ID: SectionTags, Width: tagsMinWidth(tags), HasContent: len(tags) > 0},
		{ID: SectionDependsOn, Width: config.MetadataDepMinWidth, HasContent: len(deps) > 0},
		{ID: SectionBlocks, Width: config.MetadataBlkMinWidth, HasContent: hasBlocks},
	}
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
	display := taskpkg.RecurrenceDisplay(taskpkg.Recurrence(recurrenceStr))
	text := fmt.Sprintf("%s%-12s%s%s",
		colors.TaskDetailEditDimLabelColor.Tag().String(), "Recurrence:", colors.TaskDetailValueText.Tag().String(), display)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}
