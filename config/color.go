package config

// Unified color type that stores a single color and produces tcell, hex, and tview tag forms.

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// Color is a unified color representation backed by tcell.Color.
// Zero value wraps tcell.ColorDefault (transparent/inherit).
type Color struct {
	color tcell.Color
}

// NewColor creates a Color from a tcell.Color value.
func NewColor(c tcell.Color) Color {
	return Color{color: c}
}

// NewColorHex creates a Color from a hex string like "#rrggbb" or "rrggbb".
func NewColorHex(hex string) Color {
	return Color{color: tcell.GetColor(hex)}
}

// NewColorRGB creates a Color from individual R, G, B components (0-255).
func NewColorRGB(r, g, b int32) Color {
	return Color{color: tcell.NewRGBColor(r, g, b)}
}

// DefaultColor returns a Color wrapping tcell.ColorDefault (transparent/inherit).
func DefaultColor() Color {
	return Color{color: tcell.ColorDefault}
}

// TCell returns the underlying tcell.Color for use with tview widget APIs.
func (c Color) TCell() tcell.Color {
	return c.color
}

// RGB returns the red, green, blue components of the color.
func (c Color) RGB() (int32, int32, int32) {
	return c.color.RGB()
}

// Hex returns the color as a "#rrggbb" hex string.
// Returns "-" for ColorDefault (tview's convention for default/transparent).
func (c Color) Hex() string {
	if c.color == tcell.ColorDefault {
		return "-"
	}
	r, g, b := c.color.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// Tag returns a ColorTag builder for constructing tview color tags.
func (c Color) Tag() ColorTag {
	return ColorTag{fg: c}
}

// IsDefault returns true if this is the default/transparent color.
func (c Color) IsDefault() bool {
	return c.color == tcell.ColorDefault
}

// ColorTag is a composable builder for tview [fg:bg:attr] color tags.
// Use Color.Tag() to create one, then chain Bold() / WithBg() as needed.
type ColorTag struct {
	fg   Color
	bg   *Color
	bold bool
}

// Bold returns a new ColorTag with the bold attribute set.
func (t ColorTag) Bold() ColorTag {
	t.bold = true
	return t
}

// WithBg returns a new ColorTag with the given background color.
func (t ColorTag) WithBg(c Color) ColorTag {
	t.bg = &c
	return t
}

// String renders the tview color tag string.
//
// Examples:
//
//	Color.Tag().String()             → "[#rrggbb]"
//	Color.Tag().Bold().String()      → "[#rrggbb::b]"
//	Color.Tag().WithBg(bg).String()  → "[#rrggbb:#rrggbb]"
func (t ColorTag) String() string {
	fg := t.fg.Hex()

	hasBg := t.bg != nil
	if !hasBg && !t.bold {
		return "[" + fg + "]"
	}

	bg := "-"
	if hasBg {
		bg = t.bg.Hex()
	}

	attr := ""
	if t.bold {
		attr = "b"
	}

	return "[" + fg + ":" + bg + ":" + attr + "]"
}
