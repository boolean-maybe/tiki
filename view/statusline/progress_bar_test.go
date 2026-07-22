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
		wantAllFull   bool // every bar cell is █
		wantAllEmpty  bool // every bar cell is the track glyph ░
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
			if tt.wantAllEmpty && strings.ContainsRune(bar, '█') {
				t.Fatalf("0%% bar should have no full cells: %q", bar)
			}
			if tt.wantAllFull && strings.ContainsRune(bar, '░') {
				t.Fatalf("100%% bar should have no track cells: %q", bar)
			}
		})
	}
}

func TestRenderProgressBar_DeterminateLeadingEdge(t *testing.T) {
	// a ratio that lands mid-cell (35% of 10 cells → subUnits 14, not a clean
	// multiple of the ramp max) must produce a soft boundary cell — a ramp
	// glyph that is neither full █ nor track ░ — between fill and track.
	got := RenderProgressBar(35, 100, 0, 10)
	bar := strings.TrimSuffix(got, " 35%")
	shades := 0
	for _, r := range bar {
		if r != '█' && r != '░' {
			shades++
		}
	}
	if shades != 1 {
		t.Fatalf("expected exactly one soft boundary cell, got %d in %q", shades, bar)
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

func TestRenderProgressBar_CometHasBrightHead(t *testing.T) {
	// with the head on-bar, a full-brightness cell must be present.
	got := RenderProgressBar(0, 0, 4, 12)
	if !strings.ContainsRune(got, '█') {
		t.Fatalf("comet frame should contain a bright head █: %q", got)
	}
}

func TestRenderProgressBar_CometHasFadingTail(t *testing.T) {
	// with the comet fully on-bar, the head █ must be trailed by a fade —
	// at least one ▓ and one ▒ — distinguishing a comet from a flat segment.
	got := RenderProgressBar(0, 0, 6, 20)
	if !strings.ContainsRune(got, '▓') || !strings.ContainsRune(got, '▒') {
		t.Fatalf("comet should have a fading tail (▓ then ▒): %q", got)
	}
}

func TestRenderProgressBar_CometWrapsSmoothly(t *testing.T) {
	// over a full cycle every frame must render cols runes drawn only from the
	// ramp — no stray glyphs, no width drift at the wrap.
	const cols = 12
	const cometTailLen = 3
	allowed := map[rune]bool{'░': true, '▒': true, '▓': true, '█': true}
	for f := 0; f < cols+cometTailLen; f++ {
		got := RenderProgressBar(0, 0, f, cols)
		runes := []rune(got)
		if len(runes) != cols {
			t.Fatalf("frame %d: got %d runes, want %d (%q)", f, len(runes), cols, got)
		}
		for _, r := range runes {
			if !allowed[r] {
				t.Fatalf("frame %d: unexpected glyph %q in %q", f, r, got)
			}
		}
	}
}

func TestRenderProgressBar_MinWidth(t *testing.T) {
	// 1 column must not panic and must produce a single bar cell + percent.
	got := RenderProgressBar(1, 2, 0, 1)
	if !strings.HasSuffix(got, " 50%") {
		t.Fatalf("min-width bar %q missing percent", got)
	}
}
