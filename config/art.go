package config

// ASCII art logo rendering with gradient coloring for the header.

import (
	"fmt"
	"strings"
)

const artDots = "▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒\n▒ ● ● ● ▓ ● ▓ ● ▓ ● ▓ ● ▒\n▒ ▓ ● ▓ ▓ ● ▓ ● ● ▓ ▓ ● ▒\n▒ ▓ ● ▓ ▓ ● ▓ ● ▓ ● ▓ ● ▒\n▒ ▓ ● ▓ ▓ ● ▓ ● ▓ ● ▓ ● ▒\n▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒"

// GetArtTView returns the art logo formatted for tview (with tview color codes).
// Colors are sourced from the palette via ColorConfig.
func GetArtTView() string {
	colors := GetColors()
	dotColor := colors.LogoDotColor.Hex()
	shadeColor := colors.LogoShadeColor.Hex()
	borderColor := colors.LogoBorderColor.Hex()

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
