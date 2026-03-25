package statusline

import (
	"fmt"
	"sort"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"
)

const (
	separatorRight = "\u25B6" // ▶ (left-to-right powerline arrow)
	separatorLeft  = "\u25C0" // ◀ (right-to-left powerline arrow)
)

// StatuslineWidget renders a powerline-style status bar at the bottom of the screen.
// Subscribes to StatuslineConfig for all state.
type StatuslineWidget struct {
	*tview.TextView

	config     *model.StatuslineConfig
	listenerID int
	lastWidth  int
}

// NewStatuslineWidget creates a statusline that observes StatuslineConfig
func NewStatuslineWidget(cfg *model.StatuslineConfig) *StatuslineWidget {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetTextAlign(tview.AlignLeft)
	tv.SetWrap(false)

	sw := &StatuslineWidget{
		TextView: tv,
		config:   cfg,
	}

	sw.listenerID = cfg.AddListener(sw.rebuild)
	sw.rebuild()
	return sw
}

// Draw overrides to detect width changes and re-render with proper alignment
func (sw *StatuslineWidget) Draw(screen tcell.Screen) {
	_, _, width, _ := sw.GetRect()
	if width != sw.lastWidth {
		sw.lastWidth = width
		sw.render(width)
	}
	sw.TextView.Draw(screen)
}

// Cleanup removes the listener from StatuslineConfig
func (sw *StatuslineWidget) Cleanup() {
	sw.config.RemoveListener(sw.listenerID)
}

// rebuild is called when StatuslineConfig changes
func (sw *StatuslineWidget) rebuild() {
	if sw.lastWidth > 0 {
		sw.render(sw.lastWidth)
	}
}

// statSegment holds a single stat for rendering
type statSegment struct {
	value string
	order int
}

// render builds the powerline text for the given terminal width
func (sw *StatuslineWidget) render(width int) {
	if width <= 0 {
		sw.SetText("")
		return
	}

	colors := config.GetColors()

	// left stats (base + view left stats)
	leftSegments := sortedSegments(sw.config.GetLeftStats())
	left := sw.renderLeftSegments(leftSegments, colors)
	leftLen := segmentsVisibleLen(leftSegments)

	// right stats (view-provided, right-aligned)
	rightSegments := sortedSegments(sw.config.GetRightViewStats())
	rightStats := sw.renderRightSegments(rightSegments, colors)
	rightStatsLen := segmentsVisibleLen(rightSegments)

	// message (between left and right)
	msg, level, _ := sw.config.GetMessage()
	msgRendered := sw.renderMessage(msg, level, colors)
	msgLen := visibleLen(msg)

	// pad to fill the width with the fill background color
	padLen := width - leftLen - msgLen - rightStatsLen
	if padLen < 1 {
		padLen = 1
	}
	padding := fmt.Sprintf("[-:%s]%s[-:-]", colors.StatuslineFillBg, strings.Repeat(" ", padLen))

	sw.SetText(left + msgRendered + padding + rightStats)
}

// renderLeftSegments builds the powerline left section.
// Each segment: colored text, then a separator whose fg=this segment's bg
// and bg=next segment's bg (or default for the last one).
func (sw *StatuslineWidget) renderLeftSegments(segments []statSegment, colors *config.ColorConfig) string {
	if len(segments) == 0 {
		return ""
	}

	var b strings.Builder

	for i, seg := range segments {
		bg, fg := segmentColors(i, colors)

		// segment text: " value "
		fmt.Fprintf(&b, "[%s:%s] %s ", fg, bg, seg.value)

		// separator: fg = current bg (creates the arrow), bg = next segment's bg or fill
		nextBg := colors.StatuslineFillBg
		if i < len(segments)-1 {
			nextBg, _ = segmentColors(i+1, colors)
		}
		fmt.Fprintf(&b, "[%s:%s]%s", bg, nextBg, separatorRight)
	}

	// reset colors
	b.WriteString("[-:-]")
	return b.String()
}

// renderRightSegments builds the right-aligned powerline section with ◀ separators.
// Even-index segments use accent colors, odd-index use normal colors, creating visible arrows.
func (sw *StatuslineWidget) renderRightSegments(segments []statSegment, colors *config.ColorConfig) string {
	if len(segments) == 0 {
		return ""
	}

	var b strings.Builder

	for i, seg := range segments {
		bg, fg := segmentColors(i, colors)

		// separator before segment: fg = segment bg, bg = previous segment bg (or fill)
		prevBg := colors.StatuslineFillBg
		if i > 0 {
			prevBg, _ = segmentColors(i-1, colors)
		}
		fmt.Fprintf(&b, "[%s:%s]%s", bg, prevBg, separatorLeft)

		// segment text
		fmt.Fprintf(&b, "[%s:%s] %s ", fg, bg, seg.value)
	}

	// reset colors
	b.WriteString("[-:-]")
	return b.String()
}

// segmentColors returns (bg, fg) for a segment at the given index.
// Even indices use accent colors, odd indices use normal colors.
func segmentColors(index int, colors *config.ColorConfig) (string, string) {
	if index%2 == 0 {
		return colors.StatuslineAccentBg, colors.StatuslineAccentFg
	}
	return colors.StatuslineBg, colors.StatuslineFg
}

// renderMessage builds the message section with level-specific colors
func (sw *StatuslineWidget) renderMessage(msg string, level model.MessageLevel, colors *config.ColorConfig) string {
	if msg == "" {
		return ""
	}
	fg, bg := messageColors(level, colors)
	return fmt.Sprintf("[%s:%s] %s [-:-]", fg, bg, msg)
}

// messageColors returns (fg, bg) for the given message level
func messageColors(level model.MessageLevel, colors *config.ColorConfig) (string, string) {
	switch level {
	case model.MessageLevelError:
		return colors.StatuslineErrorFg, colors.StatuslineErrorBg
	default:
		return colors.StatuslineInfoFg, colors.StatuslineInfoBg
	}
}

// sortedSegments converts a stat map to a sorted slice of segments
func sortedSegments(stats map[string]model.StatValue) []statSegment {
	segments := make([]statSegment, 0, len(stats))
	for _, v := range stats {
		segments = append(segments, statSegment{value: v.Value, order: v.Priority})
	}
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].order < segments[j].order
	})
	return segments
}

// segmentsVisibleLen calculates the visible character count of rendered segments
func segmentsVisibleLen(segments []statSegment) int {
	sepWidth := runewidth.StringWidth(separatorRight)
	n := 0
	for _, seg := range segments {
		// " value " + separator
		n += runewidth.StringWidth(seg.value) + 2 + sepWidth
	}
	return n
}

// visibleLen returns the visible length of a message (without color tags)
func visibleLen(msg string) int {
	if msg == "" {
		return 0
	}
	// " msg " with padding
	return runewidth.StringWidth(msg) + 2
}
