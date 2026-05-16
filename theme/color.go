package theme

// Color is a unified color representation backed by tcell.Color.
// Zero value wraps tcell.ColorDefault (transparent/inherit).
//
// Color is exported because a handful of callers legitimately hold concrete
// color values rather than roles — caption pairs in plugin definitions, palette
// fields, low-level gradient interpolation in internal/gradient, etc. Prefer
// Role for any new code: roles decouple "what should this look like" from
// "what hex does the active theme use" and stay correct when the theme is
// swapped at runtime. Reach for Color only when you genuinely need an inert
// color value disconnected from the active theme.

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type Color struct {
	color tcell.Color
}

func NewColor(c tcell.Color) Color {
	return Color{color: c}
}

func NewColorHex(hex string) Color {
	return Color{color: tcell.GetColor(hex)}
}

func NewColorRGB(r, g, b int32) Color {
	return Color{color: tcell.NewRGBColor(r, g, b)}
}

func DefaultColor() Color {
	return Color{color: tcell.ColorDefault}
}

func (c Color) TCell() tcell.Color {
	return c.color
}

func (c Color) RGB() (int32, int32, int32) {
	return c.color.RGB()
}

func (c Color) Hex() string {
	if c.color == tcell.ColorDefault {
		return "-"
	}
	r, g, b := c.color.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// tagColor returns the color's name (e.g. "green") if it has one, otherwise its hex.
// Named colors matter for tview: "[green]" resolves to the terminal's ANSI palette,
// which can differ from the literal hex equivalent "[#008000]". Preserving this
// behavior is required for byte-identical rendering after refactor.
func (c Color) tagColor() string {
	if c.color == tcell.ColorDefault {
		return "-"
	}
	if name := c.color.Name(); name != "" {
		return name
	}
	r, g, b := c.color.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func (c Color) Tag() ColorTag {
	return ColorTag{fg: c}
}

func (c Color) IsDefault() bool {
	return c.color == tcell.ColorDefault
}
