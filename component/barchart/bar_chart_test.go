package barchart

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestComputeBarLayoutShrinksToFit(t *testing.T) {
	bw, gap, content := computeBarLayout(20, 4, 5, 2)
	if bw != 3 || gap != 2 || content != 18 {
		t.Fatalf("unexpected layout: barWidth=%d gap=%d content=%d", bw, gap, content)
	}

	bw, gap, content = computeBarLayout(5, 5, 2, 1)
	if bw != 1 || gap != 0 || content != 5 {
		t.Fatalf("layout should shrink to minimal sizes, got barWidth=%d gap=%d content=%d", bw, gap, content)
	}
}

func TestComputeMaxVisibleBars(t *testing.T) {
	if got := computeMaxVisibleBars(10, 3, 1); got != 2 {
		t.Fatalf("expected 2 bars to fit, got %d", got)
	}

	if got := computeMaxVisibleBars(3, 2, 0); got != 1 {
		t.Fatalf("with tight width expect at least one bar, got %d", got)
	}
}

func TestValueToHeight(t *testing.T) {
	tests := []struct {
		name        string
		value       float64
		maxValue    float64
		chartHeight int
		want        int
	}{
		{name: "rounds up", value: 50, maxValue: 100, chartHeight: 5, want: 3},
		{name: "clamps to height", value: 200, maxValue: 100, chartHeight: 4, want: 4},
		{name: "ensures visibility", value: 1, maxValue: 100, chartHeight: 5, want: 1},
		{name: "zero value", value: 0, maxValue: 100, chartHeight: 5, want: 0},
	}

	for _, tt := range tests {
		got := valueToHeight(tt.value, tt.maxValue, tt.chartHeight)
		if got != tt.want {
			t.Fatalf("%s: got %d want %d", tt.name, got, tt.want)
		}
	}
}

func TestInterpolateRGB(t *testing.T) {
	got := interpolateRGB([3]int{0, 0, 0}, [3]int{100, 200, 250}, 0.5)
	want := [3]int{50, 100, 125}
	if got != want {
		t.Fatalf("interpolateRGB returned %v, want %v", got, want)
	}
}

func TestBarFillColorPrefersCustom(t *testing.T) {
	theme := DefaultTheme()
	bar := Bar{
		Value:    10,
		Color:    tcell.ColorRed,
		UseColor: true,
	}
	color := barFillColor(bar, 0, 3, theme)
	if color != tcell.ColorRed {
		t.Fatalf("expected custom color to be used, got %v", color)
	}

	theme.BarGradientFrom = [3]int{0, 0, 0}
	theme.BarGradientTo = [3]int{255, 0, 0}
	bar.UseColor = false
	color = barFillColor(bar, 2, 3, theme)
	expected := tcell.NewRGBColor(255, 0, 0)
	if color != expected {
		t.Fatalf("expected gradient end color, got %v", color)
	}
}

func TestValueToBrailleHeight(t *testing.T) {
	if got := valueToBrailleHeight(50, 100, 2); got != 4 {
		t.Fatalf("expected scaled height of 4, got %d", got)
	}

	if got := valueToBrailleHeight(1, 100, 2); got != 1 {
		t.Fatalf("expected minimum visible height of 1, got %d", got)
	}
}

func TestBrailleUnitsForRow(t *testing.T) {
	if got := brailleUnitsForRow(6, 0); got != 4 {
		t.Fatalf("row 0 should show 4 units, got %d", got)
	}
	if got := brailleUnitsForRow(6, 1); got != 2 {
		t.Fatalf("row 1 should show remaining 2 units, got %d", got)
	}
	if got := brailleUnitsForRow(6, 2); got != 0 {
		t.Fatalf("row 2 should be empty, got %d", got)
	}
}

func TestBrailleColumnMaskOrder(t *testing.T) {
	if got := brailleColumnMask(1, false); got != 0x40 {
		t.Fatalf("bottom-only left column should map to 0x40, got 0x%x", got)
	}

	if got := brailleColumnMask(2, true); got != 0xA0 {
		t.Fatalf("two dots on right column should map to 0xa0, got 0x%x", got)
	}

	// full columns should be filled bottom-to-top
	if got := brailleRuneForCounts(4, 0); got != rune(0x2800+0x47) {
		t.Fatalf("full left column should produce mask 0x47, got 0x%x", got-0x2800)
	}
}
