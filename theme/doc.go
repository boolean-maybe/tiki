// Package theme owns the color system. Every UI color in tiki is addressed by
// a semantic role (e.g. text.muted, status.ok, border.focus) — never by raw hex
// or by named tcell color. External code obtains roles from theme.Roles() and
// renders them through Role.Tag() / Role.TCell() / Role.Hex().
//
// # Why roles
//
// A role decouples the UI layer's "what should this look like" question from
// the theme layer's "what colors does this theme use" answer. View code asks
// for highlight or text.muted; the active theme decides which hex value backs
// each role. This makes per-theme palettes self-contained (their colors stay in
// theme-native names like Dracula.Pink or Nord.PolarNight3) and lets the role
// vocabulary evolve without touching every view.
//
// # The vocabulary (markup-resolvable names)
//
// Workflow YAML can reference these names with <role.name> markup. Every name
// here is also a Go method on *Theme (e.g. text.primary → TextPrimary()).
//
//	text.primary     — primary readable text on the default background
//	text.secondary   — slightly de-emphasized text (tiki box titles, etc.)
//	text.muted       — captions, descriptions, placeholders
//	text.label       — green label color in detail views
//	text.value       — cool gray for field values
//	text.hint        — autocomplete hint affordance (currently == text.muted)
//
//	border.focus     — selected/focused element borders
//	border.idle      — unselected/idle borders
//
//	surface.transparent — inherit terminal bg
//	surface.selection   — selected row/tag background
//	surface.canvas      — content canvas bg (inherits terminal)
//
//	highlight        — focus markers, key shortcuts, comment authors
//	accent.action    — view-scoped action keys (e.g. n=new, e=edit)
//	accent.tag       — tag-value text in compact tiki boxes
//
//	status.danger    — critical/error/blocker
//	status.warn      — warning/header info labels/plugin keys
//	status.ok        — healthy/success
//
//	statusline.main.fg / .bg
//	statusline.accent.fg / .bg
//	statusline.info.fg / .bg
//	statusline.error.fg / .bg
//	statusline.fill
//
//	deps-editor.surface — dependency editor caption bg
//
//	logo.dot         — bright dots in header logo art
//	logo.shade       — mid-tone shade in header logo art
//	logo.border      — dark border in header logo art
//
// # Legacy aliases (preserved for existing workflow.yaml compatibility)
//
//	muted → text.muted        info → status.warn       text → text.primary
//	accent → text.label       action → accent.action   danger → status.danger
//	highlight → highlight     warn → status.warn       ok → status.ok
//
// # Compound roles (Go API only — not markup-resolvable as single names)
//
//	PluginCaptions()          — PairListRole indexed by plugin config slot
//
// # Paint system
//
// Roles answer "what hex backs this slot." The Paint system answers "how should
// this slot render — solid, or as a derived gradient?" A Paint pairs a base
// role with an optional modifier; the modifier selects a gradient derivation
// algorithm applied to the role's solid color at render time. Themes therefore
// declare only solid colors — there is no gradient palette.
//
// Two interfaces, one for each output format:
//
//   - Paint.PaintString(s) — returns s wrapped in tview color tags. Used by
//     workflow.ExpandVisual when expanding `<role>` / `<role.modifier>` markup
//     into a colorized string.
//   - PositionPaint.ColorAt(t) — returns a tcell.Color for a normalized
//     position t in [0,1]. Used by screen-cell painters (e.g.
//     GradientCaptionRow) that draw per-column colors directly.
//
// Entry points:
//
//   - (*Theme).PaintResolver() returns a closure mapping (role, modifier) to
//     a Paint. workflow.ExpandVisual is the primary caller.
//   - PaintForRolePosition(base, modifier) returns a PositionPaint for screen-
//     cell consumers that already hold a resolved Role.
//
// The solid-vs-gradient branch lives entirely inside solidPaint and
// gradientPaint in paint.go. View code never inspects whether a role is
// "supposed to be" a gradient; it just calls PaintString or ColorAt.
//
// The vocabulary of valid modifiers is KnownModifierNames in this package —
// currently `accent` (lighten the base by 20%) and `lift` (boost saturation
// 1.6x). The set is closed; workflow.ValidateVisualMarkup rejects unknown
// modifiers at load time.
//
// Disambiguation: when a markup token contains dots, the suffix following the
// last dot is checked against KnownModifierNames. If it is a known modifier,
// the prefix is the role and the suffix is the modifier (text.muted.accent →
// role text.muted, modifier accent). Otherwise the whole token is the role
// name (text.muted → role text.muted, no modifier). An init() invariant in
// paint.go panics at startup if any role name ends in a known modifier
// suffix, which would make the split ambiguous.
//
// When gradcore.UseGradients is false (8/16-color terminals), gradientPaint
// degrades to solid output backed by the base role.
//
// # Boundary
//
// Outside this package, raw tcell color constructors (tcell.GetColor,
// tcell.NewRGBColor, tcell.Color<Name>) are forbidden by a golangci-lint
// forbidigo rule, with exceptions for plugin/colorparser (low-level helper)
// and *_test.go files. See .golangci.yml.
//
// # Threading
//
// theme.Roles() and theme.SetTheme() are safe for concurrent use; the active
// theme pointer is swapped atomically. A theme switch at runtime (e.g. via a
// /theme command) just calls SetTheme — no caller invalidation needed.
package theme
