package theme

import (
	"testing"

	gradcore "github.com/boolean-maybe/tiki/internal/gradient"
	"github.com/gdamore/tcell/v2"
)

// smokeConstructors compile-checks every internal constructor in the skeleton.
// These constructors are not yet wired by external callers (palettes land in a
// later tiki), but each one is part of the package's intended API surface and
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

	// exercise gradientPaint via Paint and PositionPaint interfaces.
	gradcore.UseGradients.Store(true)
	defer gradcore.UseGradients.Store(false)
	gp := gradientPaint{base: fg, algo: algoAccent}
	if gp.PaintString("ab") == "" {
		t.Errorf("gradientPaint.PaintString empty")
	}
	_ = gp.ColorAt(-0.5) // exercise clamp low (gradcore clamps internally)
	_ = gp.ColorAt(1.5)  // exercise clamp high
}
