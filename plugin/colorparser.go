package plugin

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// parseColor parses a color string (hex or named) into a tcell.Color
func parseColor(s string, defaultColor tcell.Color) tcell.Color {
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultColor
	}

	// tcell.GetColor handles both hex colors (#rrggbb) and named colors
	color := tcell.GetColor(s)
	if color == tcell.ColorDefault {
		// Try with # prefix if not present
		if !strings.HasPrefix(s, "#") {
			color = tcell.GetColor("#" + s)
		}
	}

	if color == tcell.ColorDefault {
		return defaultColor
	}

	return color
}
