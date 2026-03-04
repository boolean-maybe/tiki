package util

import "testing"

func TestGeneratePointsVisual(t *testing.T) {
	const blueColor = "[#508cff]"
	const grayColor = "[#5f6982]"
	const resetColor = "[-]"

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
			want:      grayColor + "❘❘❘❘❘❘❘❘❘❘" + resetColor,
		},
		{
			name:      "half points",
			points:    5,
			maxPoints: 10,
			want:      blueColor + "❚❚❚❚❚" + grayColor + "❘❘❘❘❘" + resetColor,
		},
		{
			name:      "max points",
			points:    10,
			maxPoints: 10,
			want:      blueColor + "❚❚❚❚❚❚❚❚❚❚" + resetColor,
		},
		{
			name:      "overflow clamped to max",
			points:    20,
			maxPoints: 10,
			want:      blueColor + "❚❚❚❚❚❚❚❚❚❚" + resetColor,
		},
		{
			name:      "negative clamped to zero",
			points:    -5,
			maxPoints: 10,
			want:      grayColor + "❘❘❘❘❘❘❘❘❘❘" + resetColor,
		},
		{
			name:      "scaled with different max (3 of 15)",
			points:    3,
			maxPoints: 15,
			want:      blueColor + "❚❚" + grayColor + "❘❘❘❘❘❘❘❘" + resetColor,
		},
		{
			name:      "scaled with different max (8 of 20)",
			points:    8,
			maxPoints: 20,
			want:      blueColor + "❚❚❚❚" + grayColor + "❘❘❘❘❘❘" + resetColor,
		},
		{
			name:      "rounding down (7 of 15)",
			points:    7,
			maxPoints: 15,
			want:      blueColor + "❚❚❚❚" + grayColor + "❘❘❘❘❘❘" + resetColor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePointsVisual(tt.points, tt.maxPoints, blueColor, grayColor)
			if got != tt.want {
				t.Errorf("GeneratePointsVisual(%d, %d) = %q, want %q", tt.points, tt.maxPoints, got, tt.want)
			}
		})
	}
}
