package view

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/util"
	"github.com/boolean-maybe/tiki/view/render"
	"github.com/boolean-maybe/tiki/workflow"
)

// TikiBox provides a reusable tiki card widget used in board and backlog views.

// applyFrameStyle applies selected/unselected styling to a frame
func applyFrameStyle(frame *tview.Frame, selected bool, roles *theme.Theme) {
	if selected {
		frame.SetBorderColor(roles.BorderFocus().TCell())
	} else {
		frame.SetBorderColor(roles.BorderIdle().TCell())
		if !roles.SurfaceTransparent().IsDefault() {
			frame.SetBackgroundColor(roles.SurfaceTransparent().TCell())
		}
	}
}

// tikiTypeEmoji returns the visual for the tiki's type field.
func tikiTypeEmoji(tk *tikipkg.Tiki) string {
	s, _, _ := tk.StringField(tikipkg.FieldType)
	return enumVisual(tikipkg.FieldType, s)
}

// tikiPriorityEmoji returns the visual for the tiki's priority field,
// or "─" when the field is absent or the value is unknown to the configured
// priority enum. Reading as a string mirrors priority's enum-only contract
// (Phase 3 conversion); empty key → no priority.
func tikiPriorityEmoji(tk *tikipkg.Tiki) string {
	key, present, _ := tk.StringField(tikipkg.FieldPriority)
	if !present || key == "" {
		return "─"
	}
	visual := enumVisual(tikipkg.FieldPriority, key)
	if visual == "" {
		return "─"
	}
	return visual
}

// enumVisual returns the visual markup string defined for a workflow enum
// value, or empty when the field is missing, not an enum, or the key is
// unknown. The returned string may contain `<role>` tokens that callers
// must pass through workflow.ExpandVisual before rendering.
func enumVisual(fieldName, key string) string {
	fd, ok := workflow.Field(fieldName)
	if !ok {
		return ""
	}
	v, found := fd.LookupEnum(key)
	if !found {
		return ""
	}
	return v.Visual
}

// tikiPointsVisual returns the rendered points visual (expanded tview tags)
// for the tiki's points enum value, or "─" when the field is absent or its
// value is unknown to the workflow-declared points enum.
func tikiPointsVisual(tk *tikipkg.Tiki, roles *theme.Theme) string {
	key, present, _ := tk.StringField(tikipkg.FieldPoints)
	if !present || key == "" {
		return "─"
	}
	markup := enumVisual(tikipkg.FieldPoints, key)
	if markup == "" {
		return "─"
	}
	expanded, err := workflow.ExpandVisual(markup, roles.PaintResolver())
	if err != nil {
		return "─"
	}
	return expanded
}

// expandTitleMarkup escapes a stored title (so a literal `[red]` stays
// inert) and then expands any `<role>` markup against the theme's role
// vocabulary. Fails closed to the plain escaped form on parse error so
// bad stored content can never crash the kanban card.
//
// Truncation note: the caller truncates the raw title by rune count
// before passing it here. Truncation is not markup-aware, so a chop
// that lands mid-token (e.g. "<highligh") would leave an unclosed `<`
// that ExpandVisual rejects, falling back to plain text and leaking
// the broken token to the user. trimUnclosedRoleToken drops any
// dangling `<...` fragment after the last unmatched `<` so the visible
// title shortens cleanly while keeping color on whatever did survive.
func expandTitleMarkup(raw string, roles *theme.Theme) string {
	escaped := tview.Escape(raw)
	cleaned := trimUnclosedRoleToken(escaped)
	expanded, err := workflow.ExpandVisual(cleaned, roles.PaintResolver())
	if err != nil {
		return cleaned
	}
	return expanded
}

// trimUnclosedRoleToken returns s with any unterminated `<...` fragment
// removed from the tail. The grammar from workflow.ExpandVisual: `<<`
// is a literal-angle escape (consumed in pairs); a lone `<` opens a
// role token that must be closed by `>` later in the string. If a chop
// (e.g. width-based truncation by the caller) lands inside an open
// role token, the loader's role-name parser fails — the visible result
// without this guard is the raw broken token leaking onto the card.
// Trimming at the unmatched `<` yields a clean shorter title.
func trimUnclosedRoleToken(s string) string {
	i := 0
	for i < len(s) {
		c := s[i]
		if c != '<' {
			i++
			continue
		}
		// `<<` → literal angle escape; skip both bytes.
		if i+1 < len(s) && s[i+1] == '<' {
			i += 2
			continue
		}
		// lone `<`: must be closed by a `>` later in the string.
		if strings.IndexByte(s[i+1:], '>') < 0 {
			return s[:i]
		}
		i++
	}
	return s
}

// tikiTags returns the tags slice stored in Fields, or nil if absent.
func tikiTags(tk *tikipkg.Tiki) []string {
	ss, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	return ss
}

// renderTikiIDGradient renders a tiki ID with the active theme's tiki.id
// role under the .accent modifier — a gentle gradient on capable terminals
// and a solid fallback otherwise. Thin wrapper over render.RenderTikiIDPaint
// kept here for call-site readability.
func renderTikiIDGradient(id string, roles *theme.Theme) string {
	return render.RenderTikiIDPaint(id, roles)
}

// buildCompactTikiContent builds the content string for compact tiki display.
// The stored title may contain `<role>` color markup (e.g. `<highlight>foo`);
// literal `[...]` tview-tag content stays escaped, while `<role>` tokens
// expand to the theme's role color and reset.
func buildCompactTikiContent(tk *tikipkg.Tiki, roles *theme.Theme, availableWidth int) string {
	emoji := tikiTypeEmoji(tk)
	idGradient := renderTikiIDGradient(tk.ID, roles)
	truncatedTitle := expandTitleMarkup(util.TruncateText(tk.Title, availableWidth), roles)
	priorityEmoji := tikiPriorityEmoji(tk)
	pointsVisual := tikiPointsVisual(tk, roles)

	titleTag := roles.TextSecondary().Tag()
	labelTag := roles.TextMuted().Tag()

	return fmt.Sprintf("%s %s\n%s%s[-]\n%spriority[-] %s  %spoints[-] %s[-]",
		emoji, idGradient,
		titleTag, truncatedTitle,
		labelTag, priorityEmoji,
		labelTag, pointsVisual)
}

// buildExpandedTikiContent builds the content string for expanded tiki display.
// Title supports `<role>` color markup like the compact variant; description
// lines are plain-escaped (description is multi-line markdown, rendered as
// preview text only).
func buildExpandedTikiContent(tk *tikipkg.Tiki, roles *theme.Theme, availableWidth int) string {
	emoji := tikiTypeEmoji(tk)
	idGradient := renderTikiIDGradient(tk.ID, roles)
	truncatedTitle := expandTitleMarkup(util.TruncateText(tk.Title, availableWidth), roles)

	// extract first 3 lines of body (description)
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

	titleTag := roles.TextSecondary().Tag()
	labelTag := roles.TextMuted().Tag()
	descTag := roles.TextMuted().Tag()
	tagValueTag := roles.AccentTag().Tag()

	// build tags string
	tagsStr := ""
	if tags := tikiTags(tk); len(tags) > 0 {
		tagsStr = labelTag + "Tags:[-] " + tagValueTag + tview.Escape(util.TruncateText(strings.Join(tags, ", "), availableWidth-6)) + "[-]"
	}

	// build priority/points line
	priorityEmoji := tikiPriorityEmoji(tk)
	pointsVisual := tikiPointsVisual(tk, roles)
	priorityPointsStr := fmt.Sprintf("%spriority[-] %s  %spoints[-] %s[-]",
		labelTag, priorityEmoji,
		labelTag, pointsVisual)

	return fmt.Sprintf("%s %s\n%s%s[-]\n%s%s[-]\n%s%s[-]\n%s%s[-]\n%s\n%s",
		emoji, idGradient,
		titleTag, truncatedTitle,
		descTag, descLine1,
		descTag, descLine2,
		descTag, descLine3,
		tagsStr, priorityPointsStr)
}

// CreateCompactTikiBox creates a compact styled tiki box widget (3 lines)
func CreateCompactTikiBox(tk *tikipkg.Tiki, selected bool, roles *theme.Theme) *tview.Frame {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(false)

	textView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		availableWidth := width - config.TikiBoxPaddingCompact
		if availableWidth < config.TikiBoxMinWidth {
			availableWidth = config.TikiBoxMinWidth
		}
		content := buildCompactTikiContent(tk, roles, availableWidth)
		textView.SetText(content)
		return x, y, width, height
	})

	frame := tview.NewFrame(textView).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true)
	applyFrameStyle(frame, selected, roles)

	return frame
}

// CreateExpandedTikiBox creates an expanded styled tiki box widget (7 lines)
func CreateExpandedTikiBox(tk *tikipkg.Tiki, selected bool, roles *theme.Theme) *tview.Frame {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(false)

	textView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		availableWidth := width - config.TikiBoxPaddingExpanded // less overhead for multiline
		if availableWidth < config.TikiBoxMinWidth {
			availableWidth = config.TikiBoxMinWidth
		}
		content := buildExpandedTikiContent(tk, roles, availableWidth)
		textView.SetText(content)
		return x, y, width, height
	})

	frame := tview.NewFrame(textView).SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBorder(true)
	applyFrameStyle(frame, selected, roles)

	return frame
}
