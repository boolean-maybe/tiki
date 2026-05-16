package config

// ASCII art logo rendering with gradient coloring for the header.

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/theme"
)

const artDots = "▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒\n▒ ● ● ● ▓ ● ▓ ● ▓ ● ▓ ● ▒\n▒ ▓ ● ▓ ▓ ● ▓ ● ● ▓ ▓ ● ▒\n▒ ▓ ● ▓ ▓ ● ▓ ● ▓ ● ▓ ● ▒\n▒ ▓ ● ▓ ▓ ● ▓ ● ▓ ● ▓ ● ▒\n▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒"

// GetArtTView returns the art logo formatted for tview (with tview color codes).
// Colors are sourced from the active theme's logo roles.
func GetArtTView() string {
	roles := theme.Roles()
	dotColor := roles.LogoDot().Hex()
	shadeColor := roles.LogoShade().Hex()
	borderColor := roles.LogoBorder().Hex()

	lines := strings.Split(artDots, "\n")
	var result strings.Builder

	for _, line := range lines {
		for _, char := range line {
			var color string
			switch char {
			case '●':
				color = dotColor
			case '▓':
				color = shadeColor
			case '▒':
				color = borderColor
			default:
				result.WriteRune(char)
				continue
			}
			fmt.Fprintf(&result, "[%s]%c", color, char)
		}
		result.WriteString("[white]\n")
	}
	return result.String()
}
