package theme

import (
	"math"

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

// GradientRole is a start→end color sweep used by gradient renderers.
type GradientRole interface {
	Start() (r, g, b int)
	End() (r, g, b int)
	InterpolateTag(t float64) string
	InterpolateTCell(t float64) tcell.Color
	FallbackRole() Role
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

type gradientRole struct {
	g        Gradient
	fallback Role
}

func (r gradientRole) Start() (int, int, int) { return r.g.Start[0], r.g.Start[1], r.g.Start[2] }
func (r gradientRole) End() (int, int, int)   { return r.g.End[0], r.g.End[1], r.g.End[2] }

func (r gradientRole) interpolate(t float64) (int32, int32, int32) {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	// Linear interpolation with proper rounding — must match util/gradient.InterpolateRGB
	// byte-for-byte. Truncation (int(float64(...))) drifts by ±1 on certain channels
	// and breaks the no-visible-change guarantee for some palettes.
	rgb := [3]int{
		int(math.Round(float64(r.g.Start[0]) + t*float64(r.g.End[0]-r.g.Start[0]))),
		int(math.Round(float64(r.g.Start[1]) + t*float64(r.g.End[1]-r.g.Start[1]))),
		int(math.Round(float64(r.g.Start[2]) + t*float64(r.g.End[2]-r.g.Start[2]))),
	}
	// #nosec G115 -- Start/End come from 0..255 byte-ranged gradient definitions,
	// and t is clamped to [0,1] above, so each component stays within int32 range.
	return int32(rgb[0]), int32(rgb[1]), int32(rgb[2])
}

func (r gradientRole) InterpolateTag(t float64) string {
	red, green, blue := r.interpolate(t)
	return Color{color: tcell.NewRGBColor(red, green, blue)}.Tag().String()
}

func (r gradientRole) InterpolateTCell(t float64) tcell.Color {
	red, green, blue := r.interpolate(t)
	return tcell.NewRGBColor(red, green, blue)
}

func (r gradientRole) FallbackRole() Role { return r.fallback }

func newGradientRole(g Gradient, fallback Role) GradientRole {
	return gradientRole{g: g, fallback: fallback}
}

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
