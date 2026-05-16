package taskdetail

import (
	"fmt"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/util/gradient"
	"github.com/boolean-maybe/tiki/workflow"
	"github.com/boolean-maybe/tiki/workflow/value"

	"github.com/rivo/tview"
)

// renderTikiIDGradient renders a tiki ID with the active theme's TikiID
// gradient (or solid fallback when gradients are disabled). Bridges the
// theme.GradientRole API to util/gradient.RenderAdaptiveGradientText so the
// pre-existing gradient cache + truecolor detection logic stays the single
// source of truth.
func renderTikiIDGradient(id string, roles *theme.Theme) string {
	g := roles.TikiIDGradient()
	sr, sg, sb := g.Start()
	er, eg, eb := g.End()
	cfgG := theme.Gradient{Start: [3]int{sr, sg, sb}, End: [3]int{er, eg, eb}}
	fr, fg, fb := g.FallbackRole().TCell().RGB()
	fallback := theme.NewColorRGB(fr, fg, fb)
	return gradient.RenderAdaptiveGradientText(id, cfgG, fallback)
}

// expandFieldText escapes a user-controlled text value against tview's
// `[...]` dynamic-color markup, then expands any `<role>` markup against
// the active theme. Order matters: escape first so a stored `[red]`
// stays inert; then run ExpandVisual so deliberately-authored
// `<highlight>foo` resolves to a tview color tag. Fails closed to the
// plain escaped form on parse error (e.g. unclosed `<` or unknown role
// name) so bad stored data can never crash a render. Returns just the
// escaped form when roles is nil, supporting fallback call sites that
// lack a theme context.
func expandFieldText(raw string, roles *theme.Theme) string {
	escaped := tview.Escape(raw)
	if roles == nil {
		return escaped
	}
	expanded, err := workflow.ExpandVisual(escaped, roles.RoleResolver())
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
	Roles        *theme.Theme
	FieldName    string
	Display      gridlayout.DisplayMode
	Store        store.Store
}

// getDimOrFullColor returns the dim role tag if in edit mode and not focused, otherwise full role tag.
func getDimOrFullColorTag(mode RenderMode, focused bool, fullRole theme.Role, dimRole theme.Role) string {
	if mode == RenderModeEdit && !focused {
		return dimRole.Tag()
	}
	return fullRole.Tag()
}

// getFocusMarker returns the focus marker string (arrow + text color) from the active theme
func getFocusMarker(roles *theme.Theme) string {
	return roles.Highlight().Tag() + "► " + roles.TextPrimary().Tag()
}

// RenderAssigneeText renders an assignee field as read-only text. The
// stored value may contain `<role>` color markup; literal `[...]` content
// renders verbatim (escape-then-expand order).
func RenderAssigneeText(tk *tikipkg.Tiki, ctx FieldRenderContext) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldAssignee
	assignee, _, _ := tk.StringField(tikipkg.FieldAssignee)

	labelTag := getDimOrFullColorTag(ctx.Mode, focused, ctx.Roles.TextLabel(), ctx.Roles.TextMuted())
	valueTag := getDimOrFullColorTag(ctx.Mode, focused, ctx.Roles.TextValue(), ctx.Roles.TextSecondary())

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Roles)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelTag, "Assignee:", valueTag, expandFieldText(defaultString(assignee, "Unassigned"), ctx.Roles))
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

	labelTag := getDimOrFullColorTag(ctx.Mode, focused, ctx.Roles.TextLabel(), ctx.Roles.TextMuted())
	valueTag := getDimOrFullColorTag(ctx.Mode, focused, ctx.Roles.TextValue(), ctx.Roles.TextSecondary())

	focusMarker := ""
	if focused && ctx.Mode == RenderModeEdit {
		focusMarker = getFocusMarker(ctx.Roles)
	}

	text := fmt.Sprintf("%s%s%-10s%s%s", focusMarker, labelTag, "Points:", valueTag, tview.Escape(points))
	textView := tview.NewTextView().SetDynamicColors(true).SetText(text)
	textView.SetBorderPadding(0, 0, 0, 0)

	return textView
}

// RenderTitleText renders a title as read-only text. The stored title may
// contain `<role>` color markup (e.g. `<highlight>foo`); literal `[...]`
// tview-tag content renders verbatim. When role is non-empty, the role's
// resolved color replaces the default title styling entirely.
func RenderTitleText(tk *tikipkg.Tiki, ctx FieldRenderContext, role string) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldTitle
	var titleTag string
	if role != "" && ctx.Roles != nil {
		resolver := ctx.Roles.RoleResolver()
		if tag, ok := resolver(role); ok {
			titleTag = tag
		} else {
			titleTag = ctx.Roles.TextPrimary().BoldTag()
		}
	} else if ctx.Mode == RenderModeEdit && !focused {
		titleTag = ctx.Roles.TextMuted().Tag()
	} else {
		titleTag = ctx.Roles.TextPrimary().BoldTag()
	}
	valueTag := ctx.Roles.TextValue().Tag()
	titleText := fmt.Sprintf("%s%s%s", titleTag, expandFieldText(tk.Title, ctx.Roles), valueTag)
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

	roles := theme.Roles()
	label := tview.NewTextView().SetDynamicColors(true).SetText(fmt.Sprintf("%sBlocks", roles.TextLabel().Tag()))
	label.SetBorderPadding(0, 0, 0, 0)

	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.SetBorderPadding(0, 0, 1, 1)
	col.AddItem(label, 1, 0, false)
	col.AddItem(component.NewTaskList(config.TaskListMetadataMaxRows).SetTasks(blocked), 0, 1, false)
	return col
}

// RenderAuthorText renders the author field as read-only text
func RenderAuthorText(tk *tikipkg.Tiki, roles *theme.Theme) tview.Primitive {
	createdBy, _, _ := tk.StringField("createdBy")
	text := fmt.Sprintf("%s%-10s%s%s",
		roles.TextMuted().Tag(), "Author:", roles.TextValue().Tag(), tview.Escape(defaultString(createdBy, "Unknown")))
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderCreatedText renders the created-at field as read-only text
func RenderCreatedText(tk *tikipkg.Tiki, roles *theme.Theme) tview.Primitive {
	createdAtStr := "Unknown"
	if !tk.CreatedAt.IsZero() {
		createdAtStr = tk.CreatedAt.Format("2006-01-02 15:04")
	}
	text := fmt.Sprintf("%s%-10s%s%s",
		roles.TextMuted().Tag(), "Created:", roles.TextValue().Tag(), createdAtStr)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderUpdatedText renders the updated-at field as read-only text
func RenderUpdatedText(tk *tikipkg.Tiki, roles *theme.Theme) tview.Primitive {
	updatedAtStr := "Unknown"
	if !tk.UpdatedAt.IsZero() {
		updatedAtStr = tk.UpdatedAt.Format("2006-01-02 15:04")
	}
	text := fmt.Sprintf("%s%-10s%s%s",
		roles.TextMuted().Tag(), "Updated:", roles.TextValue().Tag(), updatedAtStr)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderDueText renders the due date field
func RenderDueText(tk *tikipkg.Tiki, roles *theme.Theme) tview.Primitive {
	due, _, _ := tk.TimeField(tikipkg.FieldDue)
	dueDisplay := "None"
	if !due.IsZero() {
		dueDisplay = due.Format("2006-01-02")
	}
	text := fmt.Sprintf("%s%-12s%s%s",
		roles.TextMuted().Tag(), "Due:", roles.TextValue().Tag(), dueDisplay)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}

// RenderRecurrenceText renders the recurrence field
func RenderRecurrenceText(tk *tikipkg.Tiki, roles *theme.Theme) tview.Primitive {
	recurrenceStr, _, _ := tk.StringField(tikipkg.FieldRecurrence)
	display := value.RecurrenceDisplay(value.Recurrence(recurrenceStr))
	text := fmt.Sprintf("%s%-12s%s%s",
		roles.TextMuted().Tag(), "Recurrence:", roles.TextValue().Tag(), display)
	view := tview.NewTextView().SetDynamicColors(true).SetText(text)
	view.SetBorderPadding(0, 0, 0, 0)
	return view
}
