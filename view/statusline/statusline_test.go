package statusline

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/rivo/tview"
)

func testColors() *config.ColorConfig {
	return &config.ColorConfig{
		StatuslineBg:        "#normal_bg",
		StatuslineFg:        "#normal_fg",
		StatuslineAccentBg:  "#accent_bg",
		StatuslineAccentFg:  "#accent_fg",
		StatuslineMessageFg: "#msg_fg",
		StatuslineMessageBg: "#msg_bg",
	}
}

func TestSortedSegments(t *testing.T) {
	stats := map[string]model.StatValue{
		"C": {Value: "third", Priority: 30},
		"A": {Value: "first", Priority: 10},
		"B": {Value: "second", Priority: 20},
	}

	segments := sortedSegments(stats)

	if len(segments) != 3 {
		t.Fatalf("len = %d, want 3", len(segments))
	}
	if segments[0].value != "first" || segments[1].value != "second" || segments[2].value != "third" {
		t.Errorf("order = [%s, %s, %s], want [first, second, third]",
			segments[0].value, segments[1].value, segments[2].value)
	}
}

func TestSortedSegments_empty(t *testing.T) {
	segments := sortedSegments(map[string]model.StatValue{})
	if len(segments) != 0 {
		t.Errorf("len = %d, want 0", len(segments))
	}
}

func TestSegmentsVisibleLen(t *testing.T) {
	tests := []struct {
		name     string
		segments []statSegment
		want     int
	}{
		{"empty", nil, 0},
		{"ascii", []statSegment{{value: "main", order: 1}}, 4 + 2 + 1}, // "main" = 4 display width
		{"cjk", []statSegment{{value: "日本語", order: 1}}, 6 + 2 + 1},    // 3 CJK chars = 6 display width
		{"multi", []statSegment{{value: "ab", order: 1}, {value: "cd", order: 2}}, // 2*(2+2+1) = 10
			(2 + 2 + 1) + (2 + 2 + 1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := segmentsVisibleLen(tt.segments)
			if got != tt.want {
				t.Errorf("segmentsVisibleLen() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestVisibleLen(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 7}, // 5 + 2
		{"cjk", "日本", 6},      // 4 + 2
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleLen(tt.msg)
			if got != tt.want {
				t.Errorf("visibleLen(%q) = %d, want %d", tt.msg, got, tt.want)
			}
		})
	}
}

func TestRenderLeftSegments_empty(t *testing.T) {
	sw := newTestWidget()
	result := sw.renderLeftSegments(nil, testColors())
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestRenderLeftSegments_singleSegment(t *testing.T) {
	sw := newTestWidget()
	colors := testColors()
	segments := []statSegment{{value: "v1.0", order: 1}}

	result := sw.renderLeftSegments(segments, colors)

	// first segment (index 0) uses accent colors
	if !strings.Contains(result, "[#accent_fg:#accent_bg] v1.0 ") {
		t.Errorf("first segment should use accent colors, got %q", result)
	}
	// separator: fg=accent_bg (current), bg="-" (last segment)
	if !strings.Contains(result, "[#accent_bg:-]"+separatorRight) {
		t.Errorf("separator should transition to terminal default, got %q", result)
	}
	// ends with color reset
	if !strings.HasSuffix(result, "[-:-]") {
		t.Errorf("should end with color reset, got %q", result)
	}
}

func TestRenderLeftSegments_twoSegments(t *testing.T) {
	sw := newTestWidget()
	colors := testColors()
	segments := []statSegment{
		{value: "v1.0", order: 1},
		{value: "main", order: 2},
	}

	result := sw.renderLeftSegments(segments, colors)

	// first segment (index 0): accent
	if !strings.Contains(result, "[#accent_fg:#accent_bg] v1.0 ") {
		t.Errorf("first segment should use accent, got %q", result)
	}
	// separator between 1st and 2nd: fg=accent_bg, bg=normal_bg
	if !strings.Contains(result, "[#accent_bg:#normal_bg]"+separatorRight) {
		t.Errorf("separator should transition accent→normal, got %q", result)
	}
	// second segment (index 1): normal
	if !strings.Contains(result, "[#normal_fg:#normal_bg] main ") {
		t.Errorf("second segment should use normal colors, got %q", result)
	}
}

func TestRenderRightSegments_empty(t *testing.T) {
	sw := newTestWidget()
	result := sw.renderRightSegments(nil, testColors())
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestRenderRightSegments_singleSegment(t *testing.T) {
	sw := newTestWidget()
	colors := testColors()
	segments := []statSegment{{value: "42", order: 1}}

	result := sw.renderRightSegments(segments, colors)

	// index 0 (even) → accent colors
	if !strings.Contains(result, "[#accent_fg:#accent_bg] 42 ") {
		t.Errorf("segment 0 should use accent colors, got %q", result)
	}
	// separator: fg=accent_bg, bg="-" (terminal default, first segment)
	if !strings.Contains(result, "[#accent_bg:-]"+separatorLeft) {
		t.Errorf("separator should be accent→terminal, got %q", result)
	}
}

func TestRenderRightSegments_twoSegments(t *testing.T) {
	sw := newTestWidget()
	colors := testColors()
	segments := []statSegment{
		{value: "42", order: 1},
		{value: "10", order: 2},
	}

	result := sw.renderRightSegments(segments, colors)

	// index 0 (even) → accent
	if !strings.Contains(result, "[#accent_fg:#accent_bg] 42 ") {
		t.Errorf("segment 0 should use accent, got %q", result)
	}
	// index 1 (odd) → normal
	if !strings.Contains(result, "[#normal_fg:#normal_bg] 10 ") {
		t.Errorf("segment 1 should use normal, got %q", result)
	}
	// separator between 0→1: fg=normal_bg, bg=accent_bg (prev segment)
	if !strings.Contains(result, "[#normal_bg:#accent_bg]"+separatorLeft) {
		t.Errorf("separator between segments should show normal→accent transition, got %q", result)
	}
}

func TestRenderRightSegments_threeSegments(t *testing.T) {
	sw := newTestWidget()
	colors := testColors()
	segments := []statSegment{
		{value: "a", order: 1},
		{value: "b", order: 2},
		{value: "c", order: 3},
	}

	result := sw.renderRightSegments(segments, colors)

	// index 0 → accent, index 1 → normal, index 2 → accent
	if !strings.Contains(result, "[#accent_fg:#accent_bg] a ") {
		t.Error("segment 0 should use accent")
	}
	if !strings.Contains(result, "[#normal_fg:#normal_bg] b ") {
		t.Error("segment 1 should use normal")
	}
	if !strings.Contains(result, "[#accent_fg:#accent_bg] c ") {
		t.Error("segment 2 should use accent")
	}
}

func TestRenderMessage_empty(t *testing.T) {
	sw := newTestWidget()
	result := sw.renderMessage("", testColors())
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestRenderMessage_nonEmpty(t *testing.T) {
	sw := newTestWidget()
	colors := testColors()
	result := sw.renderMessage("error occurred", colors)

	if !strings.Contains(result, "[#msg_fg:#msg_bg] error occurred ") {
		t.Errorf("message should use message colors, got %q", result)
	}
	if !strings.HasSuffix(result, "[-:-]") {
		t.Errorf("should end with color reset, got %q", result)
	}
}

func TestRightSegmentColors(t *testing.T) {
	colors := testColors()

	bg0, fg0 := segmentColors(0, colors)
	if bg0 != "#accent_bg" || fg0 != "#accent_fg" {
		t.Errorf("index 0: got (%s, %s), want accent", bg0, fg0)
	}

	bg1, fg1 := segmentColors(1, colors)
	if bg1 != "#normal_bg" || fg1 != "#normal_fg" {
		t.Errorf("index 1: got (%s, %s), want normal", bg1, fg1)
	}

	bg2, fg2 := segmentColors(2, colors)
	if bg2 != "#accent_bg" || fg2 != "#accent_fg" {
		t.Errorf("index 2: got (%s, %s), want accent", bg2, fg2)
	}
}

func newTestWidget() *StatuslineWidget {
	cfg := model.NewStatuslineConfig()
	return &StatuslineWidget{TextView: tview.NewTextView(), config: cfg}
}
