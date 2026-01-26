package gradient

import (
	"fmt"
	"math"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/gdamore/tcell/v2"
)

// InterpolateRGB performs linear RGB interpolation with proper rounding.
// t should be in [0, 1] range (automatically clamped).
func InterpolateRGB(from, to [3]int, t float64) [3]int {
	// Clamp t to [0, 1]
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	return [3]int{
		int(math.Round(float64(from[0]) + t*float64(to[0]-from[0]))),
		int(math.Round(float64(from[1]) + t*float64(to[1]-from[1]))),
		int(math.Round(float64(from[2]) + t*float64(to[2]-from[2]))),
	}
}

// InterpolateColor is a convenience wrapper returning tcell.Color.
func InterpolateColor(gradient config.Gradient, t float64) tcell.Color {
	rgb := InterpolateRGB(gradient.Start, gradient.End, t)
	//nolint:gosec // G115: RGB values are 0-255, safe to convert to int32
	return tcell.NewRGBColor(int32(rgb[0]), int32(rgb[1]), int32(rgb[2]))
}

// ClampRGB ensures RGB value stays within [0, 255].
func ClampRGB(value int) int {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}

// LightenRGB increases brightness toward white by ratio [0, 1].
func LightenRGB(rgb [3]int, ratio float64) [3]int {
	return [3]int{
		ClampRGB(rgb[0] + int(math.Round(float64(255-rgb[0])*ratio))),
		ClampRGB(rgb[1] + int(math.Round(float64(255-rgb[1])*ratio))),
		ClampRGB(rgb[2] + int(math.Round(float64(255-rgb[2])*ratio))),
	}
}

// DarkenRGB decreases brightness toward black by ratio [0, 1].
func DarkenRGB(rgb [3]int, ratio float64) [3]int {
	return [3]int{
		ClampRGB(int(math.Round(float64(rgb[0]) * (1 - ratio)))),
		ClampRGB(int(math.Round(float64(rgb[1]) * (1 - ratio)))),
		ClampRGB(int(math.Round(float64(rgb[2]) * (1 - ratio)))),
	}
}

// RenderGradientText renders text with character-by-character gradient coloring.
func RenderGradientText(text string, gradient config.Gradient) string {
	if len(text) == 0 {
		return ""
	}

	var builder strings.Builder
	for i, char := range text {
		t := float64(i) / float64(len(text)-1)
		if len(text) == 1 {
			t = 0
		}
		rgb := InterpolateRGB(gradient.Start, gradient.End, t)
		fmt.Fprintf(&builder, "[#%02x%02x%02x]%c", rgb[0], rgb[1], rgb[2], char)
	}
	return builder.String()
}

// GradientFromColor derives a gradient by lightening the base color.
func GradientFromColor(primary tcell.Color, ratio float64, fallback config.Gradient) config.Gradient {
	r, g, b := primary.RGB()
	if r == 0 && g == 0 && b == 0 {
		return fallback
	}

	baseRGB := [3]int{int(r), int(g), int(b)}
	lighterRGB := LightenRGB(baseRGB, ratio)

	return config.Gradient{
		Start: baseRGB,
		End:   lighterRGB,
	}
}

// GradientFromColorVibrant derives a vibrant gradient by boosting RGB values.
func GradientFromColorVibrant(primary tcell.Color, boost float64, fallback config.Gradient) config.Gradient {
	r, g, b := primary.RGB()
	if r == 0 && g == 0 && b == 0 {
		return fallback
	}

	baseRGB := [3]int{int(r), int(g), int(b)}
	boostedRGB := [3]int{
		ClampRGB(int(math.Round(float64(baseRGB[0]) * boost))),
		ClampRGB(int(math.Round(float64(baseRGB[1]) * boost))),
		ClampRGB(int(math.Round(float64(baseRGB[2]) * boost))),
	}

	return config.Gradient{
		Start: baseRGB,
		End:   boostedRGB,
	}
}
