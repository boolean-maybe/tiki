package tikidetail

import (
	"fmt"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/view/render"
	"github.com/boolean-maybe/tiki/workflow"

	"github.com/rivo/tview"
)

// renderTikiIDGradient renders a tiki ID with the active theme's tiki.id
// role under the .accent modifier — gradient on capable terminals, solid
// fallback otherwise. Thin wrapper over render.RenderTikiIDPaint.
func renderTikiIDGradient(id string, roles *theme.Theme) string {
	return render.RenderTikiIDPaint(id, roles)
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
	expanded, err := workflow.ExpandVisual(escaped, roles.PaintResolver())
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

// RenderTitleText renders a title as read-only text. The stored title may
// contain `<role>` color markup (e.g. `<highlight>foo`); literal `[...]`
// tview-tag content renders verbatim. When role is non-empty, the role's
// resolved color replaces the default title styling entirely. When modifier
// is non-empty (one of theme.KnownModifierNames), the title is painted
// through the role+modifier Paint — yielding a per-rune gradient on capable
// terminals and degrading to a solid base color when gradients are off.
// Empty modifier preserves the legacy bare-tag path (no `[-]` reset),
// which is what every non-modifier call site has always emitted.
func RenderTitleText(tk *tikipkg.Tiki, ctx FieldRenderContext, role, modifier string) tview.Primitive {
	focused := ctx.Mode == RenderModeEdit && ctx.FocusedField == model.EditFieldTitle
	expanded := expandFieldText(tk.Title(), ctx.Roles)

	// modifier path: paint the expanded title through the resolver. The Paint
	// emits its own opening tag and a trailing [-] reset; no value tag is
	// appended because the reset would discard it on the next character.
	if role != "" && modifier != "" && ctx.Roles != nil {
		if paint, ok := ctx.Roles.PaintResolver()(role, modifier); ok {
			titleBox := tview.NewTextView().
				SetDynamicColors(true).
				SetText(paint.PaintString(expanded))
			titleBox.SetBorderPadding(0, 0, 0, 0)
			return titleBox
		}
		// resolver miss — fall through to the bare-tag default
	}

	var titleTag string
	switch {
	case role != "" && ctx.Roles != nil:
		if r, ok := ctx.Roles.ResolveByName(role); ok {
			titleTag = r.Tag()
		} else {
			titleTag = ctx.Roles.TextPrimary().BoldTag()
		}
	case ctx.Mode == RenderModeEdit && !focused:
		titleTag = ctx.Roles.TextMuted().Tag()
	default:
		titleTag = ctx.Roles.TextPrimary().BoldTag()
	}
	valueTag := ctx.Roles.TextValue().Tag()
	titleText := fmt.Sprintf("%s%s%s", titleTag, expanded, valueTag)
	titleBox := tview.NewTextView().
		SetDynamicColors(true).
		SetText(titleText)
	titleBox.SetBorderPadding(0, 0, 0, 0)
	return titleBox
}

// wordListColumn wraps a WordList of the given words in the standard list
// column frame (1-cell left/right padding). It is the generic primitive for a
// stringList field's value — a plain word-wrapping text column, no selection
// background, no per-row chrome. The padding is mirrored by listColumnPadding
// in the width measure, keeping measure and render in lockstep.
func wordListColumn(words []string) tview.Primitive {
	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.SetBorderPadding(0, 0, 1, 1)
	col.AddItem(component.NewWordList(words), 0, 1, false)
	return col
}

// tikiIDListColumn is the generic primitive for a tikiIdList field's value: a
// column of "ID title" rows, one per declared id. Each id is resolved against
// the store; unresolved ids render as a placeholder row so the rendered row
// count stays in lockstep with the height contract and the grid algorithm does
// not reserve dead rows. Driven purely by the id slice and store, with no
// reference to any particular field — any tikiIdList field uses it.
func tikiIDListColumn(ids []string, tikiStore store.Store) tview.Primitive {
	rendered := make([]*tikipkg.Tiki, 0, len(ids))
	for _, id := range ids {
		if dep := tikiStore.GetTiki(id); dep != nil {
			rendered = append(rendered, dep)
			continue
		}
		unresolved := tikipkg.New()
		unresolved.SetID(id)
		unresolved.SetTitle("(unresolved)")
		rendered = append(rendered, unresolved)
	}

	col := tview.NewFlex().SetDirection(tview.FlexRow)
	col.SetBorderPadding(0, 0, 1, 1)
	// SetSelectable(false): a metadata-grid value is static, not an interactive
	// list — without this the list highlights row 0 by default, painting a
	// stray selection background behind the first entry.
	list := component.NewTikiList(config.TikiListMetadataMaxRows).SetSelectable(false).SetTikis(rendered)
	col.AddItem(list, 0, 1, false)
	return col
}
