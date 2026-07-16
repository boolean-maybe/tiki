package statusline

import (
	"strings"
	"testing"
)

func TestRenderProgressBar_Determinate(t *testing.T) {
	tests := []struct {
		name          string
		done, total   int
		cols          int
		wantPctSuffix string
		wantAllFull   bool // every bar cell is ⣿
		wantAllEmpty  bool // every bar cell is ⣀
	}{
		{"zero", 0, 10, 8, " 0%", false, true},
		{"full", 10, 10, 8, " 100%", true, false},
		{"half", 5, 10, 8, " 50%", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderProgressBar(tt.done, tt.total, 0, tt.cols)
			if !strings.HasSuffix(got, tt.wantPctSuffix) {
				t.Fatalf("bar %q missing percent suffix %q", got, tt.wantPctSuffix)
			}
			bar := strings.TrimSuffix(got, tt.wantPctSuffix)
			if tt.wantAllEmpty && strings.ContainsRune(bar, '⣿') {
				t.Fatalf("0%% bar should have no full cells: %q", bar)
			}
			if tt.wantAllFull && strings.ContainsRune(bar, '⣀') {
				t.Fatalf("100%% bar should have no empty cells: %q", bar)
			}
		})
	}
}

func TestRenderProgressBar_Indeterminate_NoPercent(t *testing.T) {
	got := RenderProgressBar(0, 0, 3, 12)
	if strings.Contains(got, "%") {
		t.Fatalf("indeterminate bar must not show a percentage: %q", got)
	}
	if len([]rune(got)) == 0 {
		t.Fatal("indeterminate bar is empty")
	}
}

func TestRenderProgressBar_IndeterminateFrameMoves(t *testing.T) {
	a := RenderProgressBar(0, 0, 0, 12)
	b := RenderProgressBar(0, 0, 3, 12)
	if a == b {
		t.Fatal("indeterminate bar should differ as frame advances")
	}
}

func TestRenderProgressBar_MinWidth(t *testing.T) {
	// 1 column must not panic and must produce a single bar cell + percent.
	got := RenderProgressBar(1, 2, 0, 1)
	if !strings.HasSuffix(got, " 50%") {
		t.Fatalf("min-width bar %q missing percent", got)
	}
}
