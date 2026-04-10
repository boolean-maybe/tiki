package config

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestNewColor(t *testing.T) {
	c := NewColor(tcell.ColorYellow)
	if c.TCell() != tcell.ColorYellow {
		t.Errorf("TCell() = %v, want %v", c.TCell(), tcell.ColorYellow)
	}
}

func TestNewColorHex(t *testing.T) {
	c := NewColorHex("#ff8000")
	r, g, b := c.RGB()
	if r != 255 || g != 128 || b != 0 {
		t.Errorf("RGB() = (%d, %d, %d), want (255, 128, 0)", r, g, b)
	}
}

func TestNewColorRGB(t *testing.T) {
	c := NewColorRGB(10, 20, 30)
	r, g, b := c.RGB()
	if r != 10 || g != 20 || b != 30 {
		t.Errorf("RGB() = (%d, %d, %d), want (10, 20, 30)", r, g, b)
	}
}

func TestDefaultColor(t *testing.T) {
	c := DefaultColor()
	if !c.IsDefault() {
		t.Error("DefaultColor().IsDefault() = false, want true")
	}
	if c.TCell() != tcell.ColorDefault {
		t.Errorf("TCell() = %v, want ColorDefault", c.TCell())
	}
}

func TestColor_Hex(t *testing.T) {
	tests := []struct {
		name string
		c    Color
		want string
	}{
		{"black", NewColorRGB(0, 0, 0), "#000000"},
		{"white", NewColorRGB(255, 255, 255), "#ffffff"},
		{"red", NewColorRGB(255, 0, 0), "#ff0000"},
		{"default", DefaultColor(), "-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.Hex(); got != tt.want {
				t.Errorf("Hex() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestColor_IsDefault(t *testing.T) {
	if NewColor(tcell.ColorWhite).IsDefault() {
		t.Error("white.IsDefault() = true, want false")
	}
	if !NewColor(tcell.ColorDefault).IsDefault() {
		t.Error("default.IsDefault() = false, want true")
	}
}

func TestColorTag_String(t *testing.T) {
	c := NewColorRGB(255, 128, 0)
	got := c.Tag().String()
	want := "[#ff8000]"
	if got != want {
		t.Errorf("Tag().String() = %q, want %q", got, want)
	}
}

func TestColorTag_Bold(t *testing.T) {
	c := NewColorRGB(255, 128, 0)
	got := c.Tag().Bold().String()
	want := "[#ff8000:-:b]"
	if got != want {
		t.Errorf("Tag().Bold().String() = %q, want %q", got, want)
	}
}

func TestColorTag_WithBg(t *testing.T) {
	fg := NewColorRGB(255, 255, 255)
	bg := NewColorHex("#3a5f8a")
	got := fg.Tag().WithBg(bg).String()
	want := "[#ffffff:#3a5f8a:]"
	if got != want {
		t.Errorf("Tag().WithBg().String() = %q, want %q", got, want)
	}
}

func TestColorTag_BoldWithBg(t *testing.T) {
	fg := NewColorRGB(255, 128, 0)
	bg := NewColorRGB(0, 0, 0)
	got := fg.Tag().Bold().WithBg(bg).String()
	want := "[#ff8000:#000000:b]"
	if got != want {
		t.Errorf("Tag().Bold().WithBg().String() = %q, want %q", got, want)
	}
}

func TestColorTag_WithBgBold(t *testing.T) {
	// order shouldn't matter
	fg := NewColorRGB(255, 128, 0)
	bg := NewColorRGB(0, 0, 0)
	got := fg.Tag().WithBg(bg).Bold().String()
	want := "[#ff8000:#000000:b]"
	if got != want {
		t.Errorf("Tag().WithBg().Bold().String() = %q, want %q", got, want)
	}
}

func TestColorTag_DefaultFg(t *testing.T) {
	c := DefaultColor()
	got := c.Tag().String()
	want := "[-]"
	if got != want {
		t.Errorf("DefaultColor().Tag().String() = %q, want %q", got, want)
	}
}

func TestColorTag_DefaultBg(t *testing.T) {
	fg := NewColorRGB(255, 0, 0)
	bg := DefaultColor()
	got := fg.Tag().WithBg(bg).String()
	want := "[#ff0000:-:]"
	if got != want {
		t.Errorf("Tag().WithBg(default).String() = %q, want %q", got, want)
	}
}

func TestColorHexRoundTrip(t *testing.T) {
	original := "#5e81ac"
	c := NewColorHex(original)
	got := c.Hex()
	if got != original {
		t.Errorf("hex round-trip: NewColorHex(%q).Hex() = %q", original, got)
	}
}
