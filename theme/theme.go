package theme

import "sync/atomic"

// Theme holds the resolved role values for an active theme. All fields are
// unexported; external code accesses roles via the getter methods on *Theme.
type Theme struct {
	// Text
	textPrimary, textSecondary, textMuted, textLabel, textValue, textHint Role
	// Border
	borderFocus, borderIdle Role
	// Surface
	surfaceTransparent, surfaceSelection, surfaceCanvas Role
	// Accent / focus
	highlight, accentAction, accentTag Role
	// Status
	statusDanger, statusWarn, statusOk Role
	// Statusline (pair roles + a single fill)
	statuslineMain, statuslineAccent, statuslineInfo, statuslineError PairRole
	statuslineFill                                                    Role
	// Tiki-specific
	tikiID Role
	// Logo
	logoDot, logoShade, logoBorder Role
	// Pair-list (Go API only)
	pluginCaptions PairListRole
}

// --- single-color getters ---

func (t *Theme) TextPrimary() Role        { return t.textPrimary }
func (t *Theme) TextSecondary() Role      { return t.textSecondary }
func (t *Theme) TextMuted() Role          { return t.textMuted }
func (t *Theme) TextLabel() Role          { return t.textLabel }
func (t *Theme) TextValue() Role          { return t.textValue }
func (t *Theme) TextHint() Role           { return t.textHint }
func (t *Theme) BorderFocus() Role        { return t.borderFocus }
func (t *Theme) BorderIdle() Role         { return t.borderIdle }
func (t *Theme) SurfaceTransparent() Role { return t.surfaceTransparent }
func (t *Theme) SurfaceSelection() Role   { return t.surfaceSelection }
func (t *Theme) SurfaceCanvas() Role      { return t.surfaceCanvas }
func (t *Theme) Highlight() Role          { return t.highlight }
func (t *Theme) AccentAction() Role       { return t.accentAction }
func (t *Theme) AccentTag() Role          { return t.accentTag }
func (t *Theme) StatusDanger() Role       { return t.statusDanger }
func (t *Theme) StatusWarn() Role         { return t.statusWarn }
func (t *Theme) StatusOk() Role           { return t.statusOk }
func (t *Theme) StatuslineFill() Role     { return t.statuslineFill }
func (t *Theme) TikiID() Role             { return t.tikiID }
func (t *Theme) LogoDot() Role            { return t.logoDot }
func (t *Theme) LogoShade() Role          { return t.logoShade }
func (t *Theme) LogoBorder() Role         { return t.logoBorder }

// --- pair getters ---

func (t *Theme) StatuslineMain() PairRole   { return t.statuslineMain }
func (t *Theme) StatuslineAccent() PairRole { return t.statuslineAccent }
func (t *Theme) StatuslineInfo() PairRole   { return t.statuslineInfo }
func (t *Theme) StatuslineError() PairRole  { return t.statuslineError }

// --- pair-list getters ---

func (t *Theme) PluginCaptions() PairListRole { return t.pluginCaptions }

// --- global accessor / mutator ---

var globalTheme atomic.Pointer[Theme]

// Roles returns the active Theme. Panics if SetTheme was never called.
// Bootstrap (internal/bootstrap/init.go) is responsible for calling SetTheme
// before any view code calls Roles().
func Roles() *Theme {
	t := globalTheme.Load()
	if t == nil {
		panic("theme.Roles() called before theme.SetTheme(): bootstrap order bug")
	}
	return t
}

// SetTheme atomically swaps the active theme. Safe for runtime theme switches.
func SetTheme(t *Theme) {
	globalTheme.Store(t)
}
