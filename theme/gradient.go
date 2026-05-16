package theme

// Gradient defines a start and end RGB color for a gradient transition.
type Gradient struct {
	Start [3]int // R, G, B (0-255)
	End   [3]int // R, G, B (0-255)
}

// darkenRGB returns a darkened version of an RGB triple. ratio 0 = no change, 1 = black.
//
//nolint:unused // wired in a later task; part of the theme skeleton API.
func darkenRGB(rgb [3]int, ratio float64) [3]int {
	return [3]int{
		int(float64(rgb[0]) * (1 - ratio)),
		int(float64(rgb[1]) * (1 - ratio)),
		int(float64(rgb[2]) * (1 - ratio)),
	}
}

// gradientFromColor derives a gradient from a single Color by darkening for the start.
//
//nolint:unused // wired in a later task; part of the theme skeleton API.
func gradientFromColor(c Color, darkenRatio float64) Gradient {
	r, g, b := c.RGB()
	end := [3]int{int(r), int(g), int(b)}
	return Gradient{Start: darkenRGB(end, darkenRatio), End: end}
}
