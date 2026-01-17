package util

import "testing"

func TestGeneratePointsVisual(t *testing.T) {
	tests := []struct {
		name      string
		points    int
		maxPoints int
		want      string
	}{
		{
			name:      "zero points",
			points:    0,
			maxPoints: 10,
			want:      "◦◦◦◦◦◦◦◦◦◦",
		},
		{
			name:      "half points",
			points:    5,
			maxPoints: 10,
			want:      "●●●●●◦◦◦◦◦",
		},
		{
			name:      "max points",
			points:    10,
			maxPoints: 10,
			want:      "●●●●●●●●●●",
		},
		{
			name:      "overflow clamped to max",
			points:    20,
			maxPoints: 10,
			want:      "●●●●●●●●●●",
		},
		{
			name:      "negative clamped to zero",
			points:    -5,
			maxPoints: 10,
			want:      "◦◦◦◦◦◦◦◦◦◦",
		},
		{
			name:      "scaled with different max (3 of 15)",
			points:    3,
			maxPoints: 15,
			want:      "●●◦◦◦◦◦◦◦◦",
		},
		{
			name:      "scaled with different max (8 of 20)",
			points:    8,
			maxPoints: 20,
			want:      "●●●●◦◦◦◦◦◦",
		},
		{
			name:      "rounding down (7 of 15)",
			points:    7,
			maxPoints: 15,
			want:      "●●●●◦◦◦◦◦◦",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePointsVisual(tt.points, tt.maxPoints)
			if got != tt.want {
				t.Errorf("GeneratePointsVisual(%d, %d) = %q, want %q", tt.points, tt.maxPoints, got, tt.want)
			}
		})
	}
}
