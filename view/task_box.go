package view

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/util"
	"github.com/boolean-maybe/tiki/util/gradient"
)

// TaskBox provides a reusable task card widget used in board and backlog views.

// applyFrameStyle applies selected/unselected styling to a frame
func applyFrameStyle(frame *tview.Frame, selected bool, colors *config.ColorConfig) {
	if selected {
		frame.SetBorderColor(colors.TaskBoxSelectedBorder.TCell())
	} else {
		frame.SetBorderColor(colors.TaskBoxUnselectedBorder.TCell())
		if !colors.TaskBoxUnselectedBackground.IsDefault() {
			frame.SetBackgroundColor(colors.TaskBoxUnselectedBackground.TCell())
		}
	}
}

// tikiTypeEmoji returns the emoji for the tiki's type field.
func tikiTypeEmoji(tk *tikipkg.Tiki) string {
	s, _, _ := tk.StringField(tikipkg.FieldType)
	return task.TypeEmoji(task.Type(s))
}

// tikiPriorityEmoji returns the emoji label for the tiki's priority field,
// or "─" when the field is absent (no priority set). This prevents absent
// priority from rendering as high-priority red (PriorityLabel(0) → 🔴).
func tikiPriorityEmoji(tk *tikipkg.Tiki) string {
	n, present, _ := tk.IntField(tikipkg.FieldPriority)
	if !present {
		return "─"
	}
	return task.PriorityLabel(n)
}

// tikiPoints returns the numeric points stored in Fields, or 0 if absent.
func tikiPoints(tk *tikipkg.Tiki) int {
	n, _, _ := tk.IntField(tikipkg.FieldPoints)
	return n
}

// tikiTags returns the tags slice stored in Fields, or nil if absent.
func tikiTags(tk *tikipkg.Tiki) []string {
	ss, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	return ss
}

// buildCompactTaskContent builds the content string for compact task display
func buildCompactTaskContent(tk *tikipkg.Tiki, colors *config.ColorConfig, availableWidth int) string {
	emoji := tikiTypeEmoji(tk)
	idGradient := gradient.RenderAdaptiveGradientText(tk.ID, colors.TaskBoxIDColor, colors.FallbackTaskIDColor)
	truncatedTitle := tview.Escape(util.TruncateText(tk.Title, availableWidth))
	priorityEmoji := tikiPriorityEmoji(tk)
	pointsVisual := util.GeneratePointsVisual(tikiPoints(tk), config.GetMaxPoints(), colors.PointsFilledColor, colors.PointsUnfilledColor)

	titleTag := colors.TaskBoxTitleColor.Tag().String()
	labelTag := colors.TaskBoxLabelColor.Tag().String()

	return fmt.Sprintf("%s %s\n%s%s[-]\n%spriority[-] %s  %spoints[-] %s%s[-]",
		emoji, idGradient,
		titleTag, truncatedTitle,
		labelTag, priorityEmoji,
		labelTag, labelTag, pointsVisual)
}

// buildExpandedTaskContent builds the content string for expanded task display
func buildExpandedTaskContent(tk *tikipkg.Tiki, colors *config.ColorConfig, availableWidth int) string {
	emoji := tikiTypeEmoji(tk)
	idGradient := gradient.RenderAdaptiveGradientText(tk.ID, colors.TaskBoxIDColor, colors.FallbackTaskIDColor)
	truncatedTitle := tview.Escape(util.TruncateText(tk.Title, availableWidth))

	// Extract first 3 lines of body (description)
	descLines := strings.Split(tk.Body, "\n")
	descLine1 := ""
	descLine2 := ""
	descLine3 := ""

	if len(descLines) > 0 {
		descLine1 = tview.Escape(util.TruncateText(descLines[0], availableWidth))
	}
	if len(descLines) > 1 {
		descLine2 = tview.Escape(util.TruncateText(descLines[1], availableWidth))
	}
	if len(descLines) > 2 {
		descLine3 = tview.Escape(util.TruncateText(descLines[2], availableWidth))
	}

	titleTag := colors.TaskBoxTitleColor.Tag().String()
	labelTag := colors.TaskBoxLabelColor.Tag().String()
	descTag := colors.TaskBoxDescriptionColor.Tag().String()
	tagValueTag := colors.TaskBoxTagValueColor.Tag().String()

	// Build tags string
	tagsStr := ""
	if tags := tikiTags(tk); len(tags) > 0 {
		tagsStr = labelTag + "Tags:[-] " + tagValueTag + tview.Escape(util.TruncateText(strings.Join(tags, ", "), availableWidth-6)) + "[-]"
	}

	// Build priority/points line
	priorityEmoji := tikiPriorityEmoji(tk)
	pointsVisual := util.GeneratePointsVisual(tikiPoints(tk), config.GetMaxPoints(), colors.PointsFilledColor, colors.PointsUnfilledColor)
	priorityPointsStr := fmt.Sprintf("%spriority[-] %s  %spoints[-] %s%s[-]",
		labelTag, priorityEmoji,
		labelTag, labelTag, pointsVisual)

	return fmt.Sprintf("%s %s\n%s%s[-]\n%s%s[-]\n%s%s[-]\n%s%s[-]\n%s\n%s",
		emoji, idGradient,
		titleTag, truncatedTitle,
		descTag, descLine1,
		descTag, descLine2,
		descTag, descLine3,
		tagsStr, priorityPointsStr)
}

// CreateCompactTaskBox creates a compact styled task box widget (3 lines)
func CreateCompactTaskBox(tk *tikipkg.Tiki, selected bool, colors *config.ColorConfig) *tview.Frame {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(false)

	textView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		availableWidth := width - config.TaskBoxPaddingCompact
		if availableWidth < config.TaskBoxMinWidth {
			availableWidth = config.TaskBoxMinWidth
		}
		content := buildCompactTaskContent(tk, colors, availableWidth)
		textView.SetText(content)
		return x, y, width, height
	})

	frame := tview.NewFrame(textView).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true)
	applyFrameStyle(frame, selected, colors)

	return frame
}

// CreateExpandedTaskBox creates an expanded styled task box widget (7 lines)
func CreateExpandedTaskBox(tk *tikipkg.Tiki, selected bool, colors *config.ColorConfig) *tview.Frame {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(false)

	textView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		availableWidth := width - config.TaskBoxPaddingExpanded // less overhead for multiline
		if availableWidth < config.TaskBoxMinWidth {
			availableWidth = config.TaskBoxMinWidth
		}
		content := buildExpandedTaskContent(tk, colors, availableWidth)
		textView.SetText(content)
		return x, y, width, height
	})

	frame := tview.NewFrame(textView).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true)
	applyFrameStyle(frame, selected, colors)

	return frame
}
