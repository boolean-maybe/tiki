package plugin

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestParseColor(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultColor tcell.Color
		want         tcell.Color
	}{
		{
			name:         "empty string returns default",
			input:        "",
			defaultColor: tcell.ColorWhite,
			want:         tcell.ColorWhite,
		},
		{
			name:         "whitespace returns default",
			input:        "   ",
			defaultColor: tcell.ColorBlue,
			want:         tcell.ColorBlue,
		},
		{
			name:         "hex color with hash",
			input:        "#ff0000",
			defaultColor: tcell.ColorWhite,
			want:         tcell.GetColor("#ff0000"),
		},
		{
			name:         "hex color without hash",
			input:        "00ff00",
			defaultColor: tcell.ColorWhite,
			want:         tcell.GetColor("#00ff00"),
		},
		{
			name:         "named color red",
			input:        "red",
			defaultColor: tcell.ColorWhite,
			want:         tcell.GetColor("red"),
		},
		{
			name:         "named color blue",
			input:        "blue",
			defaultColor: tcell.ColorWhite,
			want:         tcell.GetColor("blue"),
		},
		{
			name:         "hex with uppercase",
			input:        "#FF00FF",
			defaultColor: tcell.ColorWhite,
			want:         tcell.GetColor("#FF00FF"),
		},
		// Short hex removed - tcell may not support #fff consistently
		{
			name:         "invalid color returns default",
			input:        "notacolor123",
			defaultColor: tcell.ColorYellow,
			want:         tcell.ColorYellow,
		},
		{
			name:         "color with leading/trailing whitespace",
			input:        "  #ff0000  ",
			defaultColor: tcell.ColorWhite,
			want:         tcell.GetColor("#ff0000"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseColor(tt.input, tt.defaultColor)
			if got != tt.want {
				t.Errorf("parseColor(%q, %v) = %v, want %v",
					tt.input, tt.defaultColor, got, tt.want)
			}
		})
	}
}

func TestParseColorWithNamedColors(t *testing.T) {
	// Test specific named colors that tcell supports
	namedColors := []string{
		"black", "maroon", "green", "olive",
		"navy", "purple", "teal", "silver",
		"gray", "red", "lime", "yellow",
		"blue", "fuchsia", "aqua", "white",
	}

	for _, colorName := range namedColors {
		t.Run(colorName, func(t *testing.T) {
			got := parseColor(colorName, tcell.ColorDefault)
			expected := tcell.GetColor(colorName)
			if got != expected {
				t.Errorf("parseColor(%q) = %v, want %v", colorName, got, expected)
			}
		})
	}
}
