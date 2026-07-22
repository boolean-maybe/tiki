package statusline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/theme"
	"github.com/rivo/tview"
)

// testRoles loads the canonical dark theme and reuses its hex values for
// the segment-color expectations. Going through the real theme path keeps
// the tests honest after the role-based refactor; the alternative —
// constructing a hand-crafted *theme.Theme with internal getters — would
// require exposing setters from the theme package.
func testRoles() *theme.Theme {
	return theme.LoadByName("dark")
}

// hex returns "[<fg>:<bg>]" tag prefix for the given (fg, bg) role pair as
// a string, matching what the statusline render code emits.
func segHex(fg, bg theme.Role) string {
	return fmt.Sprintf("[%s:%s]", fg.Hex(), bg.Hex())
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
	result := sw.renderLeftSegments(nil, testRoles())
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestRenderLeftSegments_singleSegment(t *testing.T) {
	sw := newTestWidget()
	roles := testRoles()
	segments := []statSegment{{value: "v1.0", order: 1}}

	result := sw.renderLeftSegments(segments, roles)

	accentTag := segHex(roles.StatuslineAccent().Fg(), roles.StatuslineAccent().Bg())
	if !strings.Contains(result, accentTag+" v1.0 ") {
		t.Errorf("first segment should use accent colors, got %q", result)
	}
	// separator: fg=accent_bg, bg=fill (last segment)
	sepTag := fmt.Sprintf("[%s:%s]", roles.StatuslineAccent().Bg().Hex(), roles.StatuslineFill().Hex())
	if !strings.Contains(result, sepTag+separatorRight) {
		t.Errorf("separator should transition to fill background, got %q", result)
	}
	if !strings.HasSuffix(result, "[-:-]") {
		t.Errorf("should end with color reset, got %q", result)
	}
}

func TestRenderLeftSegments_twoSegments(t *testing.T) {
	sw := newTestWidget()
	roles := testRoles()
	segments := []statSegment{
		{value: "v1.0", order: 1},
		{value: "main", order: 2},
	}

	result := sw.renderLeftSegments(segments, roles)

	accentTag := segHex(roles.StatuslineAccent().Fg(), roles.StatuslineAccent().Bg())
	if !strings.Contains(result, accentTag+" v1.0 ") {
		t.Errorf("first segment should use accent, got %q", result)
	}
	// separator between 1st and 2nd: fg=accent_bg, bg=normal_bg
	sep1 := fmt.Sprintf("[%s:%s]", roles.StatuslineAccent().Bg().Hex(), roles.StatuslineMain().Bg().Hex())
	if !strings.Contains(result, sep1+separatorRight) {
		t.Errorf("separator should transition accent→normal, got %q", result)
	}
	normalTag := segHex(roles.StatuslineMain().Fg(), roles.StatuslineMain().Bg())
	if !strings.Contains(result, normalTag+" main ") {
		t.Errorf("second segment should use normal colors, got %q", result)
	}
}

func TestRenderRightSegments_empty(t *testing.T) {
	sw := newTestWidget()
	result := sw.renderRightSegments(nil, testRoles())
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestRenderRightSegments_singleSegment(t *testing.T) {
	sw := newTestWidget()
	roles := testRoles()
	segments := []statSegment{{value: "42", order: 1}}

	result := sw.renderRightSegments(segments, roles)

	accentTag := segHex(roles.StatuslineAccent().Fg(), roles.StatuslineAccent().Bg())
	if !strings.Contains(result, accentTag+" 42 ") {
		t.Errorf("segment 0 should use accent colors, got %q", result)
	}
	sepTag := fmt.Sprintf("[%s:%s]", roles.StatuslineAccent().Bg().Hex(), roles.StatuslineFill().Hex())
	if !strings.Contains(result, sepTag+separatorLeft) {
		t.Errorf("separator should be accent→fill, got %q", result)
	}
}

func TestRenderRightSegments_twoSegments(t *testing.T) {
	sw := newTestWidget()
	roles := testRoles()
	segments := []statSegment{
		{value: "42", order: 1},
		{value: "10", order: 2},
	}

	result := sw.renderRightSegments(segments, roles)

	accentTag := segHex(roles.StatuslineAccent().Fg(), roles.StatuslineAccent().Bg())
	if !strings.Contains(result, accentTag+" 42 ") {
		t.Errorf("segment 0 should use accent, got %q", result)
	}
	normalTag := segHex(roles.StatuslineMain().Fg(), roles.StatuslineMain().Bg())
	if !strings.Contains(result, normalTag+" 10 ") {
		t.Errorf("segment 1 should use normal, got %q", result)
	}
	sep := fmt.Sprintf("[%s:%s]", roles.StatuslineMain().Bg().Hex(), roles.StatuslineAccent().Bg().Hex())
	if !strings.Contains(result, sep+separatorLeft) {
		t.Errorf("separator between segments should show normal→accent transition, got %q", result)
	}
}

func TestRenderRightSegments_threeSegments(t *testing.T) {
	sw := newTestWidget()
	roles := testRoles()
	segments := []statSegment{
		{value: "a", order: 1},
		{value: "b", order: 2},
		{value: "c", order: 3},
	}

	result := sw.renderRightSegments(segments, roles)

	accentTag := segHex(roles.StatuslineAccent().Fg(), roles.StatuslineAccent().Bg())
	normalTag := segHex(roles.StatuslineMain().Fg(), roles.StatuslineMain().Bg())
	if !strings.Contains(result, accentTag+" a ") {
		t.Error("segment 0 should use accent")
	}
	if !strings.Contains(result, normalTag+" b ") {
		t.Error("segment 1 should use normal")
	}
	if !strings.Contains(result, accentTag+" c ") {
		t.Error("segment 2 should use accent")
	}
}

func TestRenderMessage_empty(t *testing.T) {
	sw := newTestWidget()
	result := sw.renderMessage("", model.MessageLevelInfo, testRoles())
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestRenderMessage_info(t *testing.T) {
	sw := newTestWidget()
	roles := testRoles()
	result := sw.renderMessage("tiki saved", model.MessageLevelInfo, roles)

	infoTag := segHex(roles.StatuslineInfo().Fg(), roles.StatuslineInfo().Bg())
	if !strings.Contains(result, infoTag+" tiki saved ") {
		t.Errorf("info message should use info colors, got %q", result)
	}
	if !strings.HasSuffix(result, "[-:-]") {
		t.Errorf("should end with color reset, got %q", result)
	}
}

func TestRenderMessage_error(t *testing.T) {
	sw := newTestWidget()
	roles := testRoles()
	result := sw.renderMessage("validation failed", model.MessageLevelError, roles)

	errorTag := segHex(roles.StatuslineError().Fg(), roles.StatuslineError().Bg())
	if !strings.Contains(result, errorTag+" validation failed ") {
		t.Errorf("error message should use error colors, got %q", result)
	}
	if !strings.HasSuffix(result, "[-:-]") {
		t.Errorf("should end with color reset, got %q", result)
	}
}

func TestRightSegmentColors(t *testing.T) {
	roles := testRoles()

	bg0, fg0 := segmentColors(0, roles)
	if bg0.Hex() != roles.StatuslineAccent().Bg().Hex() || fg0.Hex() != roles.StatuslineAccent().Fg().Hex() {
		t.Errorf("index 0: got (%s, %s), want accent", bg0.Hex(), fg0.Hex())
	}

	bg1, fg1 := segmentColors(1, roles)
	if bg1.Hex() != roles.StatuslineMain().Bg().Hex() || fg1.Hex() != roles.StatuslineMain().Fg().Hex() {
		t.Errorf("index 1: got (%s, %s), want normal", bg1.Hex(), fg1.Hex())
	}

	bg2, fg2 := segmentColors(2, roles)
	if bg2.Hex() != roles.StatuslineAccent().Bg().Hex() || fg2.Hex() != roles.StatuslineAccent().Fg().Hex() {
		t.Errorf("index 2: got (%s, %s), want accent", bg2.Hex(), fg2.Hex())
	}
}

func newTestWidget() *StatuslineWidget {
	cfg := model.NewStatuslineConfig()
	return &StatuslineWidget{TextView: tview.NewTextView(), config: cfg}
}

func TestStatuslineWidget_ProgressPreemptsMessage(t *testing.T) {
	sw := newTestWidget()
	sw.config.SetMessage("copied to clipboard", model.MessageLevelInfo, false)
	sw.config.SetProgress(1, 4)

	mid := sw.messageSectionForTest(testRoles())
	if strings.Contains(mid, "copied to clipboard") {
		t.Fatalf("message should be hidden while progress active: %q", mid)
	}
	if !strings.Contains(mid, "%") {
		t.Fatalf("determinate progress bar (with %%) not rendered: %q", mid)
	}
}

func TestStatusline_ProgressBarFillsMiddleZone(t *testing.T) {
	sw := newTestWidget()
	sw.config.SetProgress(0, 0) // indeterminate, active

	_, narrowLen := sw.messageSection(testRoles(), 20)
	_, wideLen := sw.messageSection(testRoles(), 60)

	if wideLen <= narrowLen {
		t.Fatalf("bar width should grow with available width: narrow=%d wide=%d", narrowLen, wideLen)
	}
}

func TestStatusline_DeterminatePercentStaysInsideZone(t *testing.T) {
	// cover the widest suffix (" 100%") too: pctReserve must fit it.
	cases := []struct{ done, total int }{{1, 2}, {10, 10}}
	const avail = 30
	for _, c := range cases {
		sw := newTestWidget()
		sw.config.SetProgress(c.done, c.total)
		_, visLen := sw.messageSection(testRoles(), avail)
		if visLen > avail {
			t.Fatalf("%d/%d: bar+percent width %d exceeds zone %d", c.done, c.total, visLen, avail)
		}
	}
}

func TestStatuslineWidget_MessageWhenNoProgress(t *testing.T) {
	sw := newTestWidget()
	sw.config.SetMessage("copied to clipboard", model.MessageLevelInfo, false)

	mid := sw.messageSectionForTest(testRoles())
	if !strings.Contains(mid, "copied to clipboard") {
		t.Fatalf("message should render when no progress active: %q", mid)
	}
}
