package barchart

import (
	"testing"
)

func TestValueToBrailleHeightEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		value       float64
		maxValue    float64
		chartHeight int
		want        int
	}{
		{"zero chartHeight", 50, 100, 0, 0},
		{"zero maxValue", 50, 0, 10, 0},
		{"zero value", 0, 100, 10, 0},
		{"small value clamps to 1", 0.1, 100, 10, 1},
		{"full bar value equals maxValue", 100, 100, 10, 40},
		{"value exceeds maxValue clamps", 200, 100, 10, 40},
		{"half value", 50, 100, 10, 20},
		{"negative value treated as zero", -5, 100, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueToBrailleHeight(tt.value, tt.maxValue, tt.chartHeight)
			if got != tt.want {
				t.Errorf("valueToBrailleHeight(%v, %v, %v) = %v, want %v",
					tt.value, tt.maxValue, tt.chartHeight, got, tt.want)
			}
		})
	}
}

func TestBrailleUnitsForRowEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		totalUnits int
		row        int
		want       int
	}{
		{"zero totalUnits", 0, 0, 0},
		{"negative totalUnits", -1, 0, 0},
		{"row above filled area", 4, 1, 0},
		{"partial last row", 5, 1, 1},
		{"full row capped at 4", 8, 0, 4},
		{"first row full", 4, 0, 4},
		{"row 0 partial", 2, 0, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := brailleUnitsForRow(tt.totalUnits, tt.row)
			if got != tt.want {
				t.Errorf("brailleUnitsForRow(%d, %d) = %d, want %d",
					tt.totalUnits, tt.row, got, tt.want)
			}
		})
	}
}

func TestBrailleColumnMaskLevels(t *testing.T) {
	tests := []struct {
		name        string
		level       int
		rightColumn bool
		want        uint8
	}{
		{"zero level left", 0, false, 0},
		{"zero level right", 0, true, 0},
		// left column dots: 0x40 (dot 7), 0x04 (dot 3), 0x02 (dot 2), 0x01 (dot 1) — bottom to top
		{"left level 2", 2, false, 0x40 | 0x04},
		{"left level 3", 3, false, 0x40 | 0x04 | 0x02},
		{"left level 4 all dots", 4, false, 0x40 | 0x04 | 0x02 | 0x01},
		{"left level 5 clamps to 4", 5, false, 0x40 | 0x04 | 0x02 | 0x01},
		// right column dots: 0x80 (dot 8), 0x20 (dot 6), 0x10 (dot 5), 0x08 (dot 4) — bottom to top
		{"right level 2", 2, true, 0x80 | 0x20},
		{"right level 4 all dots", 4, true, 0x80 | 0x20 | 0x10 | 0x08},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := brailleColumnMask(tt.level, tt.rightColumn)
			if got != tt.want {
				t.Errorf("brailleColumnMask(%d, %v) = 0x%02X, want 0x%02X",
					tt.level, tt.rightColumn, got, tt.want)
			}
		})
	}
}

func TestBrailleRuneForCountsValues(t *testing.T) {
	tests := []struct {
		name       string
		leftCount  int
		rightCount int
		want       rune
	}{
		{"both zero empty braille", 0, 0, 0x2800},
		{"both full all dots", 4, 4, 0x28FF},
		// left=1 → mask 0x40; right=0 → mask 0x00; rune = 0x2840
		{"left 1 right 0", 1, 0, 0x2840},
		// left=0 → 0x00; right=1 → 0x80; rune = 0x2880
		{"left 0 right 1", 0, 1, 0x2880},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := brailleRuneForCounts(tt.leftCount, tt.rightCount)
			if got != tt.want {
				t.Errorf("brailleRuneForCounts(%d, %d) = U+%04X, want U+%04X",
					tt.leftCount, tt.rightCount, got, tt.want)
			}
		})
	}
}

func TestClampRowIndex(t *testing.T) {
	tests := []struct {
		name     string
		rowIndex int
		total    int
		want     int
	}{
		{"zero total", 5, 0, 0},
		{"negative total", 5, -1, 0},
		{"negative index clamps to 0", -3, 10, 0},
		{"index equals total clamps to total-1", 10, 10, 9},
		{"index exceeds total clamps", 15, 10, 9},
		{"valid index unchanged", 5, 10, 5},
		{"zero index valid", 0, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampRowIndex(tt.rowIndex, tt.total)
			if got != tt.want {
				t.Errorf("clampRowIndex(%d, %d) = %d, want %d",
					tt.rowIndex, tt.total, got, tt.want)
			}
		})
	}
}

func TestDominantBarForCell(t *testing.T) {
	// heights slice used in tests: index 0 = left, index 1 = right
	heights := []int{8, 12}

	tests := []struct {
		name       string
		leftIndex  int
		rightIndex int
		leftUnits  int
		rightUnits int
		leftCount  int
		rightCount int
		row        int
		heights    []int
		wantIndex  int // -1 means no bar
	}{
		{
			name:      "both counts zero",
			leftIndex: 0, rightIndex: 1,
			leftUnits: 8, rightUnits: 12,
			leftCount: 0, rightCount: 0,
			row: 0, heights: heights,
			wantIndex: -1,
		},
		{
			name:      "right count greater than left",
			leftIndex: 0, rightIndex: 1,
			leftUnits: 8, rightUnits: 12,
			leftCount: 2, rightCount: 4,
			row: 0, heights: heights,
			wantIndex: 1,
		},
		{
			name:      "left count greater than right",
			leftIndex: 0, rightIndex: 1,
			leftUnits: 8, rightUnits: 12,
			leftCount: 4, rightCount: 2,
			row: 0, heights: heights,
			wantIndex: 0,
		},
		{
			name:      "tie right units greater favors right",
			leftIndex: 0, rightIndex: 1,
			leftUnits: 4, rightUnits: 8,
			leftCount: 3, rightCount: 3,
			row: 0, heights: heights,
			wantIndex: 1,
		},
		{
			name:      "tie left units greater or equal favors left",
			leftIndex: 0, rightIndex: 1,
			leftUnits: 8, rightUnits: 4,
			leftCount: 3, rightCount: 3,
			row: 0, heights: heights,
			wantIndex: 0,
		},
		{
			// rightCount > leftCount but rightIndex OOB → falls through default →
			// rightUnits > leftUnits but rightIndex still OOB → returns leftIndex
			name:      "right index out of bounds falls through to left",
			leftIndex: 0, rightIndex: 99,
			leftUnits: 4, rightUnits: 8,
			leftCount: 2, rightCount: 4,
			row: 0, heights: heights,
			wantIndex: 0,
		},
		{
			name:      "left index out of bounds right wins",
			leftIndex: 99, rightIndex: 1,
			leftUnits: 4, rightUnits: 8,
			leftCount: 2, rightCount: 4,
			row: 0, heights: heights,
			wantIndex: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, _, _ := dominantBarForCell(
				tt.leftIndex, tt.rightIndex,
				tt.leftUnits, tt.rightUnits,
				tt.leftCount, tt.rightCount,
				tt.row, tt.heights,
			)
			if gotIndex != tt.wantIndex {
				t.Errorf("dominantBarForCell() barIndex = %d, want %d", gotIndex, tt.wantIndex)
			}
		})
	}
}
