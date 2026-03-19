package barchart

import (
	"testing"
)

func TestValueToHeightEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		value       float64
		maxValue    float64
		chartHeight int
		want        int
	}{
		{"zero chartHeight", 50, 100, 0, 0},
		{"negative chartHeight", 50, 100, -1, 0},
		{"zero maxValue", 50, 0, 10, 0},
		{"negative maxValue", 50, -1, 10, 0},
		{"zero value", 0, 100, 10, 0},
		{"small value rounds to 0 clamps to 1", 0.4, 100, 10, 1},
		{"value equals maxValue", 100, 100, 10, 10},
		{"value exceeds maxValue clamps", 200, 100, 10, 10},
		{"mid-range", 50, 100, 10, 5},
		{"negative value treated as zero", -5, 100, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueToHeight(tt.value, tt.maxValue, tt.chartHeight)
			if got != tt.want {
				t.Errorf("valueToHeight(%v, %v, %v) = %v, want %v",
					tt.value, tt.maxValue, tt.chartHeight, got, tt.want)
			}
		})
	}
}

func TestComputeBarLayoutEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		totalWidth       int
		barCount         int
		desiredBarWidth  int
		desiredGap       int
		wantBarWidth     int
		wantGapWidth     int
		wantContentWidth int
	}{
		{"zero totalWidth", 0, 5, 2, 1, 0, 0, 0},
		{"negative totalWidth", -1, 5, 2, 1, 0, 0, 0},
		{"zero barCount", 80, 0, 2, 1, 0, 0, 0},
		{"content fits exactly", 80, 5, 2, 1, 2, 1, 14},
		{"single bar no gap needed", 80, 1, 2, 1, 2, 1, 2},
		{"negative desiredGap clamped to 0", 80, 5, 2, -3, 2, 0, 10},
		{"barWidth below 1 clamps to 1", 80, 5, 0, 1, 1, 1, 9},
		{"bars too wide shrinks barWidth", 10, 5, 10, 0, 2, 0, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBar, gotGap, gotContent := computeBarLayout(tt.totalWidth, tt.barCount, tt.desiredBarWidth, tt.desiredGap)
			if gotBar != tt.wantBarWidth || gotGap != tt.wantGapWidth || gotContent != tt.wantContentWidth {
				t.Errorf("computeBarLayout(%d, %d, %d, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tt.totalWidth, tt.barCount, tt.desiredBarWidth, tt.desiredGap,
					gotBar, gotGap, gotContent,
					tt.wantBarWidth, tt.wantGapWidth, tt.wantContentWidth)
			}
		})
	}
}

func TestComputeMaxVisibleBarsEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		totalWidth int
		barWidth   int
		gapWidth   int
		want       int
	}{
		{"zero totalWidth", 0, 3, 1, 0},
		{"negative totalWidth", -1, 3, 1, 0},
		{"zero barWidth", 80, 0, 1, 0},
		{"negative barWidth", 80, -1, 1, 0},
		{"normal case", 80, 3, 1, 20},
		{"barWidth larger than totalWidth returns 1", 5, 10, 1, 1},
		{"zero gap", 12, 3, 0, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeMaxVisibleBars(tt.totalWidth, tt.barWidth, tt.gapWidth)
			if got != tt.want {
				t.Errorf("computeMaxVisibleBars(%d, %d, %d) = %d, want %d",
					tt.totalWidth, tt.barWidth, tt.gapWidth, got, tt.want)
			}
		})
	}
}

func TestMaxBarValue(t *testing.T) {
	tests := []struct {
		name string
		bars []Bar
		want float64
	}{
		{"empty slice", []Bar{}, 0.0},
		{"all zeros", []Bar{{Value: 0}, {Value: 0}}, 0.0},
		{"single bar", []Bar{{Value: 42}}, 42.0},
		{"multiple bars", []Bar{{Value: 10}, {Value: 30}, {Value: 20}}, 30.0},
		{"negative values ignored", []Bar{{Value: -5}, {Value: -1}}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxBarValue(tt.bars)
			if got != tt.want {
				t.Errorf("maxBarValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{"zero width", "hello", 0, ""},
		{"negative width", "hello", -1, ""},
		{"shorter than width unchanged", "hi", 10, "hi"},
		{"exact width unchanged", "hello", 5, "hello"},
		{"truncated at boundary", "hello", 3, "hel"},
		{"multi-byte rune boundary", "héllo", 3, "hél"},
		{"empty string", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateRunes(tt.text, tt.width)
			if got != tt.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
			}
		})
	}
}

func TestHasLabels(t *testing.T) {
	tests := []struct {
		name string
		bars []Bar
		want bool
	}{
		{"empty slice", []Bar{}, false},
		{"all empty labels", []Bar{{Label: ""}, {Label: ""}}, false},
		{"one non-empty label", []Bar{{Label: ""}, {Label: "X"}}, true},
		{"all non-empty labels", []Bar{{Label: "A"}, {Label: "B"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasLabels(tt.bars)
			if got != tt.want {
				t.Errorf("hasLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}
