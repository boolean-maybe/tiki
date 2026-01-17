package view

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/util"
	"github.com/boolean-maybe/tiki/view/common"
)

// TaskBox provides a reusable task card widget used in board and backlog views.

// applyFrameStyle applies selected/unselected styling to a frame
func applyFrameStyle(frame *tview.Frame, selected bool, colors *config.ColorConfig) {
	if selected {
		frame.SetBorderColor(colors.TaskBoxSelectedBorder)
	} else {
		frame.SetBorderColor(colors.TaskBoxUnselectedBorder)
		if colors.TaskBoxUnselectedBackground != tcell.ColorDefault {
			frame.SetBackgroundColor(colors.TaskBoxUnselectedBackground)
		}
	}
}

// buildCompactTaskContent builds the content string for compact task display
func buildCompactTaskContent(task *taskpkg.Task, colors *config.ColorConfig, availableWidth int) string {
	emoji := taskpkg.TypeEmoji(task.Type)
	idGradient := common.RenderGradientText(task.ID, colors.TaskBoxIDColor)
	truncatedTitle := util.TruncateText(task.Title, availableWidth)
	priorityEmoji := taskpkg.PriorityLabel(task.Priority)
	pointsVisual := util.GeneratePointsVisual(task.Points, config.GetMaxPoints())

	return fmt.Sprintf("%s %s\n%s%s[-]\n%spriority[-] %s  %spoints[-] %s%s[-]",
		emoji, idGradient,
		colors.TaskBoxTitleColor, truncatedTitle,
		colors.TaskBoxLabelColor, priorityEmoji,
		colors.TaskBoxLabelColor, colors.TaskBoxLabelColor, pointsVisual)
}

// buildExpandedTaskContent builds the content string for expanded task display
func buildExpandedTaskContent(task *taskpkg.Task, colors *config.ColorConfig, availableWidth int) string {
	emoji := taskpkg.TypeEmoji(task.Type)
	idGradient := common.RenderGradientText(task.ID, colors.TaskBoxIDColor)
	truncatedTitle := util.TruncateText(task.Title, availableWidth)

	// Extract first 3 lines of description
	descLines := strings.Split(task.Description, "\n")
	descLine1 := ""
	descLine2 := ""
	descLine3 := ""

	if len(descLines) > 0 {
		descLine1 = util.TruncateText(descLines[0], availableWidth)
	}
	if len(descLines) > 1 {
		descLine2 = util.TruncateText(descLines[1], availableWidth)
	}
	if len(descLines) > 2 {
		descLine3 = util.TruncateText(descLines[2], availableWidth)
	}

	// Build tags string
	tagsStr := ""
	if len(task.Tags) > 0 {
		tagsStr = colors.TaskBoxLabelColor + "Tags:[-] " + colors.TaskBoxTagValueColor + util.TruncateText(fmt.Sprintf("%v", task.Tags), availableWidth-6) + "[-]"
	}

	// Build priority/points line
	priorityEmoji := taskpkg.PriorityLabel(task.Priority)
	pointsVisual := util.GeneratePointsVisual(task.Points, config.GetMaxPoints())
	priorityPointsStr := fmt.Sprintf("%spriority[-] %s  %spoints[-] %s%s[-]",
		colors.TaskBoxLabelColor, priorityEmoji,
		colors.TaskBoxLabelColor, colors.TaskBoxLabelColor, pointsVisual)

	return fmt.Sprintf("%s %s\n%s%s[-]\n%s%s[-]\n%s%s[-]\n%s%s[-]\n%s\n%s",
		emoji, idGradient,
		colors.TaskBoxTitleColor, truncatedTitle,
		colors.TaskBoxDescriptionColor, descLine1,
		colors.TaskBoxDescriptionColor, descLine2,
		colors.TaskBoxDescriptionColor, descLine3,
		tagsStr, priorityPointsStr)
}

// CreateCompactTaskBox creates a compact styled task box widget (3 lines)
func CreateCompactTaskBox(task *taskpkg.Task, selected bool, colors *config.ColorConfig) *tview.Frame {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(false)

	textView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		availableWidth := width - config.TaskBoxPaddingCompact
		if availableWidth < config.TaskBoxMinWidth {
			availableWidth = config.TaskBoxMinWidth
		}
		content := buildCompactTaskContent(task, colors, availableWidth)
		textView.SetText(content)
		return x, y, width, height
	})

	frame := tview.NewFrame(textView).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true)
	applyFrameStyle(frame, selected, colors)

	return frame
}

// CreateExpandedTaskBox creates an expanded styled task box widget (7 lines)
func CreateExpandedTaskBox(task *taskpkg.Task, selected bool, colors *config.ColorConfig) *tview.Frame {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(false)

	textView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		availableWidth := width - config.TaskBoxPaddingExpanded // less overhead for multiline
		if availableWidth < config.TaskBoxMinWidth {
			availableWidth = config.TaskBoxMinWidth
		}
		content := buildExpandedTaskContent(task, colors, availableWidth)
		textView.SetText(content)
		return x, y, width, height
	})

	frame := tview.NewFrame(textView).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true)
	applyFrameStyle(frame, selected, colors)

	return frame
}
