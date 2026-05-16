package theme

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// smokeConstructors compile-checks every internal constructor in the skeleton.
// These constructors are not yet wired by external callers (palettes land in a
// later task), but each one is part of the package's intended API surface and
// must compile + return the expected interface.
func TestInternalSmokeConstructors(t *testing.T) {
	red := NewColor(tcell.ColorRed)
	blue := NewColor(tcell.ColorBlue)

	// colorRole via newColorRole.
	fg := newColorRole(red)
	bg := newColorRole(blue)

	// pairRole via newPairRole; exercise Fg/Bg/Tag.
	pair := newPairRole(fg, bg)
	if pair.Fg() != fg {
		t.Errorf("pair.Fg() mismatch")
	}
	if pair.Bg() != bg {
		t.Errorf("pair.Bg() mismatch")
	}
	if pair.Tag() == "" {
		t.Errorf("pair.Tag() empty")
	}

	// pairListRole via newPairListRole; exercise At and Len.
	list := newPairListRole([]PairRole{pair})
	if list.Len() != 1 {
		t.Errorf("list.Len() = %d, want 1", list.Len())
	}
	if list.At(0) != pair {
		t.Errorf("list.At(0) mismatch")
	}
	// empty list returns a default-default pair.
	emptyList := newPairListRole(nil)
	if emptyList.Len() != 0 {
		t.Errorf("emptyList.Len() = %d, want 0", emptyList.Len())
	}
	if got := emptyList.At(0); got == nil {
		t.Errorf("emptyList.At(0) returned nil")
	}

	// gradientRole via newGradientRole; exercise Start/End/InterpolateTag/InterpolateTCell/FallbackRole.
	grad := newGradientRole(Gradient{Start: [3]int{10, 20, 30}, End: [3]int{200, 210, 220}}, fg)
	if r, g, b := grad.Start(); r != 10 || g != 20 || b != 30 {
		t.Errorf("grad.Start() = (%d,%d,%d), want (10,20,30)", r, g, b)
	}
	if r, g, b := grad.End(); r != 200 || g != 210 || b != 220 {
		t.Errorf("grad.End() = (%d,%d,%d), want (200,210,220)", r, g, b)
	}
	if grad.InterpolateTag(0.5) == "" {
		t.Errorf("grad.InterpolateTag(0.5) empty")
	}
	_ = grad.InterpolateTCell(-0.5) // exercise clamp low
	_ = grad.InterpolateTCell(1.5)  // exercise clamp high
	if grad.FallbackRole() != fg {
		t.Errorf("grad.FallbackRole() mismatch")
	}

	// darkenRGB / gradientFromColor helpers.
	if got := darkenRGB([3]int{100, 200, 50}, 0.5); got != [3]int{50, 100, 25} {
		t.Errorf("darkenRGB() = %v, want [50 100 25]", got)
	}
	g := gradientFromColor(red, 0.5)
	if g.End == [3]int{0, 0, 0} {
		t.Errorf("gradientFromColor: End should equal red's RGB, got zero")
	}
}
