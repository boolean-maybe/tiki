package config

// ASCII art logo rendering with gradient coloring for the header.

import (
	"fmt"
	"strings"
)

//nolint:unused
const artFire = "▓▓▓▓▓▓╗ ▓▓  ▓▓  ▓▓  ▓▓\n╚═▒▒═╝ ▒▒  ▒▒ ▒▒   ▒▒\n  ▒▒   ▒▒  ▒▒▒▒    ▒▒\n  ░░   ░░  ░░ ░░   ░░\n  ╚═╝   ╚═╝ ╚═╝  ╚═╝ ╚═╝"

const artDots = "▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒\n▒ ● ● ● ▓ ● ▓ ● ▓ ● ▓ ● ▒\n▒ ▓ ● ▓ ▓ ● ▓ ● ● ▓ ▓ ● ▒\n▒ ▓ ● ▓ ▓ ● ▓ ● ▓ ● ▓ ● ▒\n▒ ▓ ● ▓ ▓ ● ▓ ● ▓ ● ▓ ● ▒\n▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒ ▒"

// fireGradient is the color scheme for artFire (yellow → orange → red)
//
//nolint:unused
var fireGradient = []string{"#FFDC00", "#FFAA00", "#FF7800", "#FF5000", "#B42800"}

// dotsGradient is the color scheme for artDots (bright cyan → blue gradient)
// Each character type gets a different color:
// ● (dot) = bright cyan (text)
// ▓ (dark shade) = medium blue (near)
// ▒ (medium shade) = dark blue (far)
var dotsGradient = []string{"#40E0D0", "#4682B4", "#324664"}

// var currentArt = artFire
// var currentGradient = fireGradient
var currentArt = artDots
var currentGradient = dotsGradient

// GetArtTView returns the art logo formatted for tview (with tview color codes)
// uses the current gradient colors
func GetArtTView() string {
	if currentArt == artDots {
		// For dots art, color by character type, not by row
		return getDotsArtTView()
	}

	// For other art, color by row
	lines := strings.Split(currentArt, "\n")
	var result strings.Builder

	for i, line := range lines {
		// pick color based on line index (cycle if more lines than colors)
		colorIdx := i
		if colorIdx >= len(currentGradient) {
			colorIdx = len(currentGradient) - 1
		}
		color := currentGradient[colorIdx]
		fmt.Fprintf(&result, "[%s]%s[white]\n", color, line)
	}
	return result.String()
}

// getDotsArtTView colors the dots art by character type
func getDotsArtTView() string {
	lines := strings.Split(artDots, "\n")
	var result strings.Builder

	// dotsGradient: [0]=● (text), [1]=▓ (near), [2]=▒ (far)
	for _, line := range lines {
		for _, char := range line {
			var color string
			switch char {
			case '●':
				color = dotsGradient[0] // bright cyan
			case '▓':
				color = dotsGradient[1] // medium blue
			case '▒':
				color = dotsGradient[2] // dark blue
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

// GetFireIcon returns fire icon with tview color codes
func GetFireIcon() string {
	return "[#FFDC00]      ░ ▒ ░        \n[#FFAA00]   ▒▓██▓█▒░       \n[#FF7800]  ░▓████▓██▒░      \n[#FF5000]   ▒▓██▓▓▒░       \n[#B42800]     ▒▓░          \n[white]\n"
}
