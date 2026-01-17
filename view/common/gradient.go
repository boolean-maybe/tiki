package common

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
)

// RenderGradientText creates a gradient colored text string
func RenderGradientText(text string, gradient config.Gradient) string {
	result := ""
	runes := []rune(text)
	n := len(runes)
	if n > 0 {
		start := gradient.Start
		end := gradient.End

		r1, g1, b1 := start[0], start[1], start[2]
		r2, g2, b2 := end[0], end[1], end[2]

		for i, char := range runes {
			t := 0.0
			if n > 1 {
				t = float64(i) / float64(n-1)
			}
			r := int(float64(r1) + t*(float64(r2)-float64(r1)))
			g := int(float64(g1) + t*(float64(g2)-float64(g1)))
			b := int(float64(b1) + t*(float64(b2)-float64(b1)))

			result += fmt.Sprintf("[#%02x%02x%02x::b]%c", r, g, b, char)
		}
	}
	return result
}
