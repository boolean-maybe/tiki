package view

import "testing"

func TestComputeLaneBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		weights    []int
		numLanes   int
		totalWidth int
		wantStarts []int
		wantEnds   []int
	}{
		{
			"equal weights",
			[]int{1, 1, 1}, 3, 90,
			[]int{0, 30, 60}, []int{30, 60, 90},
		},
		{
			"nil weights",
			nil, 3, 90,
			[]int{0, 30, 60}, []int{30, 60, 90},
		},
		{
			"proportional 25-50-25",
			[]int{25, 50, 25}, 3, 100,
			[]int{0, 25, 75}, []int{25, 75, 100},
		},
		{
			"single lane",
			[]int{1}, 1, 80,
			[]int{0}, []int{80},
		},
		{
			"last lane absorbs rounding",
			[]int{1, 1, 1}, 3, 100,
			// 100/3=33, 66, last gets 100
			[]int{0, 33, 66}, []int{33, 66, 100},
		},
		{
			"two lanes unequal",
			[]int{1, 3}, 2, 80,
			[]int{0, 20}, []int{20, 80},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			starts, ends := computeLaneBoundaries(tt.weights, tt.numLanes, tt.totalWidth)
			if len(starts) != tt.numLanes || len(ends) != tt.numLanes {
				t.Fatalf("len(starts)=%d, len(ends)=%d, want %d", len(starts), len(ends), tt.numLanes)
			}
			for i := range starts {
				if starts[i] != tt.wantStarts[i] {
					t.Errorf("starts[%d] = %d, want %d (full: %v)", i, starts[i], tt.wantStarts[i], starts)
					break
				}
			}
			for i := range ends {
				if ends[i] != tt.wantEnds[i] {
					t.Errorf("ends[%d] = %d, want %d (full: %v)", i, ends[i], tt.wantEnds[i], ends)
					break
				}
			}
			// verify contiguity: each start == previous end
			for i := 1; i < tt.numLanes; i++ {
				if starts[i] != ends[i-1] {
					t.Errorf("gap between lane %d and %d: end=%d, start=%d", i-1, i, ends[i-1], starts[i])
				}
			}
			// last end == totalWidth
			if ends[tt.numLanes-1] != tt.totalWidth {
				t.Errorf("last end = %d, want %d", ends[tt.numLanes-1], tt.totalWidth)
			}
		})
	}
}
