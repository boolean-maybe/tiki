package theme

import (
	"github.com/gdamore/tcell/v2"
)

// Role is a single semantic color. External code obtains roles via theme.Roles()
// getters and renders them through Tag() / TCell() / Hex().
type Role interface {
	Tag() string
	BoldTag() string
	TaggedBg(bg Role) string
	TCell() tcell.Color
	Hex() string
	IsDefault() bool
}

// PairRole is a foreground+background pair (powerline cells, captions).
type PairRole interface {
	Fg() Role
	Bg() Role
	Tag() string
}

// PairListRole is an indexed family of pairs.
type PairListRole interface {
	At(i int) PairRole
	Len() int
}

// --- concrete impls (unexported) ---

type colorRole struct{ c Color }

func (r colorRole) Tag() string     { return r.c.Tag().String() }
func (r colorRole) BoldTag() string { return r.c.Tag().Bold().String() }
func (r colorRole) TaggedBg(bg Role) string {
	bgColor := Color{color: bg.TCell()}
	return r.c.Tag().WithBg(bgColor).String()
}
func (r colorRole) TCell() tcell.Color { return r.c.TCell() }
func (r colorRole) Hex() string        { return r.c.Hex() }
func (r colorRole) IsDefault() bool    { return r.c.IsDefault() }

func newColorRole(c Color) Role { return colorRole{c: c} }

// NewColorRoleAdapter wraps a theme.Color as a Role for use with the Paint
// system. Used by view code that holds a Color from older APIs and needs to
// hand it to a PositionPaint resolver.
func NewColorRoleAdapter(c Color) Role { return colorRole{c: c} }

type pairRole struct{ fg, bg Role }

func (p pairRole) Fg() Role { return p.fg }
func (p pairRole) Bg() Role { return p.bg }
func (p pairRole) Tag() string {
	fgColor := Color{color: p.fg.TCell()}
	bgColor := Color{color: p.bg.TCell()}
	return fgColor.Tag().WithBg(bgColor).String()
}

func newPairRole(fg, bg Role) PairRole { return pairRole{fg: fg, bg: bg} }

type pairListRole struct{ pairs []PairRole }

func (p pairListRole) At(i int) PairRole {
	if len(p.pairs) == 0 {
		return pairRole{fg: newColorRole(DefaultColor()), bg: newColorRole(DefaultColor())}
	}
	if i < 0 {
		i = 0
	}
	return p.pairs[i%len(p.pairs)]
}
func (p pairListRole) Len() int { return len(p.pairs) }

func newPairListRole(pairs []PairRole) PairListRole { return pairListRole{pairs: pairs} }
