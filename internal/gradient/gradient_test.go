package gradcore

import (
	"strings"
	"testing"
)

func TestInterpolateRGB_Endpoints(t *testing.T) {
	from := [3]int{0, 0, 0}
	to := [3]int{255, 255, 255}
	if got := InterpolateRGB(from, to, 0); got != from {
		t.Errorf("t=0: got %v, want %v", got, from)
	}
	if got := InterpolateRGB(from, to, 1); got != to {
		t.Errorf("t=1: got %v, want %v", got, to)
	}
	if got := InterpolateRGB(from, to, 0.5); got != [3]int{128, 128, 128} {
		t.Errorf("t=0.5: got %v, want [128 128 128]", got)
	}
}

func TestInterpolateRGB_ClampsT(t *testing.T) {
	from := [3]int{10, 10, 10}
	to := [3]int{200, 200, 200}
	if got := InterpolateRGB(from, to, -1); got != from {
		t.Errorf("t<0: got %v, want %v", got, from)
	}
	if got := InterpolateRGB(from, to, 2); got != to {
		t.Errorf("t>1: got %v, want %v", got, to)
	}
}

func TestClampRGB(t *testing.T) {
	if ClampRGB(-1) != 0 || ClampRGB(0) != 0 || ClampRGB(128) != 128 || ClampRGB(255) != 255 || ClampRGB(256) != 255 {
		t.Error("ClampRGB boundary failure")
	}
}

func TestLightenRGB(t *testing.T) {
	got := LightenRGB([3]int{100, 100, 100}, 0.5)
	want := [3]int{178, 178, 178} // 100 + round((255-100)*0.5) = 100 + 78 = 178
	if got != want {
		t.Errorf("LightenRGB(100,0.5) = %v, want %v", got, want)
	}
}

func TestDarkenRGB(t *testing.T) {
	got := DarkenRGB([3]int{200, 200, 200}, 0.5)
	want := [3]int{100, 100, 100} // 200 * 0.5 = 100
	if got != want {
		t.Errorf("DarkenRGB(200,0.5) = %v, want %v", got, want)
	}
}

func TestDeriveDarkened_ZeroReturnsBlackPair(t *testing.T) {
	s, e := DeriveDarkened([3]int{0, 0, 0}, 0.5)
	if s != ([3]int{0, 0, 0}) || e != ([3]int{0, 0, 0}) {
		t.Errorf("got (%v,%v), want black pair", s, e)
	}
}

func TestDeriveDarkened_NonZeroReturnsDarkenedThenBase(t *testing.T) {
	// matches the historical theme.gradientFromColor: start = darken(base,ratio),
	// end = base. A consumer painting "AB" therefore lerps from the darkened
	// start to the original color.
	s, e := DeriveDarkened([3]int{200, 200, 200}, 0.5)
	if s != [3]int{100, 100, 100} {
		t.Errorf("start should be darkened(base,0.5), got %v", s)
	}
	if e != [3]int{200, 200, 200} {
		t.Errorf("end should be base, got %v", e)
	}
}

func TestDeriveVibrant_ZeroReturnsBlackPair(t *testing.T) {
	s, e := DeriveVibrant([3]int{0, 0, 0}, 1.6)
	if s != ([3]int{0, 0, 0}) || e != ([3]int{0, 0, 0}) {
		t.Errorf("got (%v,%v), want black pair", s, e)
	}
}

func TestRenderTaggedGradient_PerRuneTags(t *testing.T) {
	out := RenderTaggedGradient("ab", [3]int{0, 0, 0}, [3]int{255, 255, 255})
	if strings.Count(out, "[#") != 2 {
		t.Errorf("expected 2 color tags, got %q", out)
	}
}

func TestRenderTaggedGradient_Empty(t *testing.T) {
	if got := RenderTaggedGradient("", [3]int{0, 0, 0}, [3]int{255, 255, 255}); got != "" {
		t.Errorf("empty input should produce empty output, got %q", got)
	}
}

func TestRenderTaggedGradient_SingleRune(t *testing.T) {
	out := RenderTaggedGradient("x", [3]int{0, 0, 0}, [3]int{255, 255, 255})
	// single rune means t=0 — color is the start
	if !strings.Contains(out, "[#000000]x") {
		t.Errorf("expected start-color tag for single rune, got %q", out)
	}
}
