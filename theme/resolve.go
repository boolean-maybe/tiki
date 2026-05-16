package theme

// ResolveByName returns the Role bound to a canonical role name or one of the
// preserved aliases. Returns (nil, false) for unknown names.
//
// Canonical names use dotted hierarchical form (e.g. "text.muted", "border.focus").
// Aliases preserve the pre-refactor short names so existing workflow.yaml renders
// identically — the alias table matches the OLD config.ResolveRole mapping.
func (t *Theme) ResolveByName(name string) (Role, bool) {
	switch name {
	// canonical: text
	case "text.primary":
		return t.textPrimary, true
	case "text.secondary":
		return t.textSecondary, true
	case "text.muted":
		return t.textMuted, true
	case "text.label":
		return t.textLabel, true
	case "text.value":
		return t.textValue, true
	case "text.hint":
		return t.textHint, true

	// canonical: border
	case "border.focus":
		return t.borderFocus, true
	case "border.idle":
		return t.borderIdle, true

	// canonical: surface
	case "surface.transparent":
		return t.surfaceTransparent, true
	case "surface.selection":
		return t.surfaceSelection, true
	case "surface.canvas":
		return t.surfaceCanvas, true

	// canonical: accent / focus
	case "highlight":
		return t.highlight, true
	case "accent.action":
		return t.accentAction, true
	case "accent.tag":
		return t.accentTag, true
	case "tiki.id":
		return t.tikiID, true

	// canonical: status
	case "status.danger":
		return t.statusDanger, true
	case "status.warn":
		return t.statusWarn, true
	case "status.ok":
		return t.statusOk, true

	// canonical: statusline pair sides
	case "statusline.main.fg":
		return t.statuslineMain.Fg(), true
	case "statusline.main.bg":
		return t.statuslineMain.Bg(), true
	case "statusline.accent.fg":
		return t.statuslineAccent.Fg(), true
	case "statusline.accent.bg":
		return t.statuslineAccent.Bg(), true
	case "statusline.info.fg":
		return t.statuslineInfo.Fg(), true
	case "statusline.info.bg":
		return t.statuslineInfo.Bg(), true
	case "statusline.error.fg":
		return t.statuslineError.Fg(), true
	case "statusline.error.bg":
		return t.statuslineError.Bg(), true
	case "statusline.fill":
		return t.statuslineFill, true

	// canonical: plugin-specific
	case "deps-editor.surface":
		return t.depsEditorSurface, true

	// canonical: logo
	case "logo.dot":
		return t.logoDot, true
	case "logo.shade":
		return t.logoShade, true
	case "logo.border":
		return t.logoBorder, true

	// legacy aliases — match config.ResolveRole's old 9-name vocabulary
	case "muted":
		return t.textMuted, true
	case "accent":
		return t.textLabel, true
	case "info":
		return t.statusWarn, true
	case "action":
		return t.accentAction, true
	case "text":
		return t.textPrimary, true
	case "danger":
		return t.statusDanger, true
	case "warn":
		return t.statusWarn, true
	case "ok":
		return t.statusOk, true
	}
	return nil, false
}

// PaintResolver returns a closure that maps (role, modifier) to a Paint.
// Used by workflow.ExpandVisual at render time. modifier is the optional
// suffix following the last dot in `<role.modifier>` markup; pass "" for
// solid (no modifier). Reports ok=false for unknown role or modifier names.
func (t *Theme) PaintResolver() func(role, modifier string) (Paint, bool) {
	return func(role, modifier string) (Paint, bool) {
		base, ok := t.ResolveByName(role)
		if !ok {
			return nil, false
		}
		return paintFor(base, modifier)
	}
}

// canonicalRoleNames is the full list of dotted-hierarchical role names that
// ResolveByName accepts. These are the markup-resolvable names documented in
// theme/doc.go and validated at workflow load time by workflow.ValidRoles.
var canonicalRoleNames = []string{
	"text.primary", "text.secondary", "text.muted", "text.label",
	"text.value", "text.hint",
	"border.focus", "border.idle",
	"surface.transparent", "surface.selection", "surface.canvas",
	"highlight", "accent.action", "accent.tag", "tiki.id",
	"status.danger", "status.warn", "status.ok",
	"statusline.main.fg", "statusline.main.bg",
	"statusline.accent.fg", "statusline.accent.bg",
	"statusline.info.fg", "statusline.info.bg",
	"statusline.error.fg", "statusline.error.bg",
	"statusline.fill",
	"deps-editor.surface",
	"logo.dot", "logo.shade", "logo.border",
}

// legacyAliasNames are the pre-refactor short names ResolveByName still
// accepts so existing workflow.yaml files render identically.
var legacyAliasNames = []string{
	"muted", "accent", "info", "action", "text",
	"danger", "warn", "ok",
	// "highlight" appears in canonicalRoleNames; not duplicated here.
}

// KnownRoleNames returns every role name ResolveByName accepts — canonical
// dotted names plus legacy aliases. Callers needing to validate user-supplied
// role names at load time (e.g. workflow.ValidRoles) should derive their
// vocabulary from this list so the resolver and the validator stay in sync.
func KnownRoleNames() []string {
	out := make([]string, 0, len(canonicalRoleNames)+len(legacyAliasNames))
	out = append(out, canonicalRoleNames...)
	out = append(out, legacyAliasNames...)
	return out
}
