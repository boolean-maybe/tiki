// Package gradcore holds pure gradient math and the runtime-capability
// flags consumed by every gradient renderer in Tiki. It is a leaf with
// no dependencies on other Tiki packages. Higher packages adapt their own
// color types to the [3]int triples this package uses.
package gradcore

import (
	"fmt"
	"math"
	"strings"
	"sync/atomic"
)

// UseGradients controls whether gradients are rendered or solid colors used.
// Set during bootstrap based on terminal color capability. When false, all
// gradient consumers degrade to a solid color from the gradient's base role.
var UseGradients atomic.Bool

// UseWideGradients controls whether per-column screen-wide gradients are
// rendered. False forces gradient consumers that span the screen (caption
// rows) to use a flat color from the gradient instead — avoids visible
// banding on 256-color terminals where adjacent columns would quantize to
// the same palette index but produce a stepped appearance. UseGradients
// must also be true for wide gradients to render; on 8/16-color terminals
// (UseGradients=false), this flag has no effect.
//
// Set during bootstrap alongside UseGradients based on terminal color
// capability (typically true only for truecolor).
var UseWideGradients atomic.Bool

// InterpolateRGB performs linear RGB interpolation with proper rounding.
// t is clamped to [0,1].
func InterpolateRGB(from, to [3]int, t float64) [3]int {
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

// ClampRGB clamps a single channel to [0,255].
func ClampRGB(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// LightenRGB moves toward white by ratio in [0,1].
func LightenRGB(rgb [3]int, ratio float64) [3]int {
	return [3]int{
		ClampRGB(rgb[0] + int(math.Round(float64(255-rgb[0])*ratio))),
		ClampRGB(rgb[1] + int(math.Round(float64(255-rgb[1])*ratio))),
		ClampRGB(rgb[2] + int(math.Round(float64(255-rgb[2])*ratio))),
	}
}

// DarkenRGB moves toward black by ratio in [0,1].
func DarkenRGB(rgb [3]int, ratio float64) [3]int {
	return [3]int{
		ClampRGB(int(math.Round(float64(rgb[0]) * (1 - ratio)))),
		ClampRGB(int(math.Round(float64(rgb[1]) * (1 - ratio)))),
		ClampRGB(int(math.Round(float64(rgb[2]) * (1 - ratio)))),
	}
}

// DeriveDarkened returns (darken(base,ratio), base). The dark end of the
// gradient is the start; the original color is the end. When base is (0,0,0)
// the gradient collapses to (black, black) — callers can interpret that as
// "no useful gradient available" and degrade.
func DeriveDarkened(base [3]int, ratio float64) (start, end [3]int) {
	if base == [3]int{0, 0, 0} {
		return base, base
	}
	return DarkenRGB(base, ratio), base
}

// DeriveVibrant returns (base, boost(base,factor)). When base is (0,0,0)
// the gradient collapses to (black, black) — same fallback semantics as
// DeriveDarkened.
func DeriveVibrant(base [3]int, boost float64) (start, end [3]int) {
	if base == [3]int{0, 0, 0} {
		return base, base
	}
	boosted := [3]int{
		ClampRGB(int(math.Round(float64(base[0]) * boost))),
		ClampRGB(int(math.Round(float64(base[1]) * boost))),
		ClampRGB(int(math.Round(float64(base[2]) * boost))),
	}
	return base, boosted
}

// RenderTaggedGradient emits text with one `[#rrggbb]` color tag per rune
// for the (start,end) gradient. Returns "" for empty text. No cache (the
// caller may wrap this with caching if desired).
func RenderTaggedGradient(text string, start, end [3]int) string {
	if len(text) == 0 {
		return ""
	}
	var b strings.Builder
	runes := []rune(text)
	n := len(runes)
	b.Grow(n * 17)
	for i, ch := range runes {
		var t float64
		if n == 1 {
			t = 0
		} else {
			t = float64(i) / float64(n-1)
		}
		rgb := InterpolateRGB(start, end, t)
		fmt.Fprintf(&b, "[#%02x%02x%02x]%c", rgb[0], rgb[1], rgb[2], ch)
	}
	return b.String()
}
