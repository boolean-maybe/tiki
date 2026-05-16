package theme

import (
	"fmt"
	"strings"
	"testing"

	gradcore "github.com/boolean-maybe/tiki/internal/gradient"
)

func TestSolidPaint_PaintString(t *testing.T) {
	role := newColorRole(NewColorHex("#ff0000"))
	p := solidPaint{role: role}
	got := p.PaintString("hello")
	want := role.Tag() + "hello[-]"
	if got != want {
		t.Errorf("solidPaint.PaintString = %q, want %q", got, want)
	}
}

func TestSolidPaint_PaintString_Empty(t *testing.T) {
	role := newColorRole(NewColorHex("#ff0000"))
	p := solidPaint{role: role}
	got := p.PaintString("")
	if got != "" {
		t.Errorf("solidPaint.PaintString(\"\") = %q, want empty", got)
	}
}

func TestSolidPaint_ColorAt(t *testing.T) {
	role := newColorRole(NewColorHex("#abcdef"))
	p := solidPaint{role: role}
	for _, ti := range []float64{0, 0.5, 1, -1, 2} {
		if got := p.ColorAt(ti); got != role.TCell() {
			t.Errorf("solidPaint.ColorAt(%v) = %v, want %v", ti, got, role.TCell())
		}
	}
}

func TestSolidPaint_SatisfiesBothInterfaces(t *testing.T) {
	role := newColorRole(NewColorHex("#123456"))
	var _ Paint = solidPaint{role: role}
	var _ PositionPaint = solidPaint{role: role}
}

func TestGradientPaint_PaintString_PerRuneTags(t *testing.T) {
	base := newColorRole(NewColorHex("#0080ff"))
	p := gradientPaint{base: base, algo: algoAccent}
	gradcore.UseGradients.Store(true)
	defer gradcore.UseGradients.Store(false)
	got := p.PaintString("ab")
	openCount := strings.Count(got, "[#")
	if openCount != 2 {
		t.Errorf("gradientPaint.PaintString: got %d color tags, want 2; output=%q", openCount, got)
	}
	if !strings.HasSuffix(got, "[-]") {
		t.Errorf("gradientPaint.PaintString: missing trailing reset; output=%q", got)
	}
}

func TestGradientPaint_DisabledGradient_DegradesToSolid(t *testing.T) {
	base := newColorRole(NewColorHex("#0080ff"))
	p := gradientPaint{base: base, algo: algoAccent}
	gradcore.UseGradients.Store(false)
	defer gradcore.UseGradients.Store(false)
	got := p.PaintString("x")
	want := base.Tag() + "x[-]"
	if got != want {
		t.Errorf("gradientPaint.PaintString (gradients off) = %q, want %q", got, want)
	}
}

// TestAlgoAccent_ByteOutput pins the byte-for-byte output of the accent
// modifier against the spec rule: start = darken(base, 0.2), end = base.
// Locks the historical theme.gradientFromColor(c, 0.2) semantics that the
// spec mandates.
func TestAlgoAccent_ByteOutput(t *testing.T) {
	gradcore.UseGradients.Store(true)
	defer gradcore.UseGradients.Store(false)
	// pick a base whose darken-by-20% does not collide with base
	base := newColorRole(NewColorHex("#0080ff"))
	br, bg, bb := base.TCell().RGB()
	baseRGB := [3]int{int(br), int(bg), int(bb)}
	startRGB := gradcore.DarkenRGB(baseRGB, 0.2)
	// two runes, t=0 → start, t=1 → end (= base)
	want := fmt.Sprintf("[#%02x%02x%02x]A[#%02x%02x%02x]B[-]",
		startRGB[0], startRGB[1], startRGB[2],
		baseRGB[0], baseRGB[1], baseRGB[2])
	p := gradientPaint{base: base, algo: algoAccent}
	got := p.PaintString("AB")
	if got != want {
		t.Errorf("algoAccent byte output:\n got: %q\nwant: %q", got, want)
	}
}

// TestAlgoLift_ByteOutput pins the byte-for-byte output of the lift modifier
// against the historical GradientFromColorVibrant(c, 1.6) semantics:
// start = base, end = clamp(base * 1.6, 255). Note the inverse ordering
// relative to algoAccent — lift's vivid endpoint is the *end*.
func TestAlgoLift_ByteOutput(t *testing.T) {
	gradcore.UseGradients.Store(true)
	defer gradcore.UseGradients.Store(false)
	base := newColorRole(NewColorHex("#0080ff"))
	br, bg, bb := base.TCell().RGB()
	baseRGB := [3]int{int(br), int(bg), int(bb)}
	_, endRGB := gradcore.DeriveVibrant(baseRGB, 1.6)
	want := fmt.Sprintf("[#%02x%02x%02x]A[#%02x%02x%02x]B[-]",
		baseRGB[0], baseRGB[1], baseRGB[2],
		endRGB[0], endRGB[1], endRGB[2])
	p := gradientPaint{base: base, algo: algoLift}
	got := p.PaintString("AB")
	if got != want {
		t.Errorf("algoLift byte output:\n got: %q\nwant: %q", got, want)
	}
}
