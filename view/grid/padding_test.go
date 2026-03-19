package grid

import (
	"testing"
)

func TestPadToFullRows(t *testing.T) {
	tests := []struct {
		name         string
		items        []int
		rowCount     int
		expectedLen  int
		expectedTail []int // last N elements after padding
	}{
		{
			name:        "empty slice returns unchanged",
			items:       []int{},
			rowCount:    3,
			expectedLen: 0,
		},
		{
			name:        "already divisible — no padding added",
			items:       []int{1, 2, 3, 4, 5, 6},
			rowCount:    3,
			expectedLen: 6,
		},
		{
			name:         "7 items / rowCount 3 — pads to 9",
			items:        []int{1, 2, 3, 4, 5, 6, 7},
			rowCount:     3,
			expectedLen:  9,
			expectedTail: []int{0, 0}, // two zero-value ints appended
		},
		{
			name:         "rowCount 1 — never needs padding",
			items:        []int{1, 2, 3},
			rowCount:     1,
			expectedLen:  3,
			expectedTail: []int{3},
		},
		{
			name:         "1 item / rowCount 4 — pads to 4",
			items:        []int{42},
			rowCount:     4,
			expectedLen:  4,
			expectedTail: []int{0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadToFullRows(tt.items, tt.rowCount)

			if len(result) != tt.expectedLen {
				t.Errorf("len = %d, want %d", len(result), tt.expectedLen)
			}

			for i, want := range tt.expectedTail {
				idx := tt.expectedLen - len(tt.expectedTail) + i
				if result[idx] != want {
					t.Errorf("result[%d] = %v, want %v", idx, result[idx], want)
				}
			}
		})
	}
}
