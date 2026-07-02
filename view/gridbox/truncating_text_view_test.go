package gridbox

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// drawToString renders a single-row primitive of the given width and returns
// the visible runes of that row.
func drawToString(t *testing.T, p interface {
	SetRect(int, int, int, int)
	Draw(tcell.Screen)
}, w int) string {
	t.Helper()
	const h = 1
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(w, h)
	p.SetRect(0, 0, w, h)
	p.Draw(screen)
	var b strings.Builder
	for x := 0; x < w; x++ {
		r, _, _, _ := screen.GetContent(x, 0)
		if r == 0 {
			r = ' '
		}
		b.WriteRune(r)
	}
	return b.String()
}

func TestTruncatingTextView_AppendsEllipsisWhenClipped(t *testing.T) {
	tv := NewTruncatingTextView()
	tv.SetText("tags: backend, security, urgent, refactor")

	got := drawToString(t, tv, 18)
	if !strings.Contains(got, "…") {
		t.Errorf("expected an ellipsis when text exceeds width, got %q", got)
	}
	// the visible content must fit within the 18-cell width.
	line := strings.SplitN(got, "\n", 2)[0]
	if vis := len([]rune(strings.TrimRight(line, " "))); vis > 18 {
		t.Errorf("rendered %d visible cells, want <= 18: %q", vis, line)
	}
}

func TestTruncatingTextView_LeavesTrailingColumnGapWhenClipped(t *testing.T) {
	// when text is clipped, the last column must stay blank so the ellipsis
	// (and any flush text) does not butt against the box's right border.
	tv := NewTruncatingTextView()
	tv.SetText("tags: backend, security, urgent, refactor")

	const w = 18
	got := drawToString(t, tv, w)
	runes := []rune(strings.SplitN(got, "\n", 2)[0])
	if last := runes[w-1]; last != ' ' {
		t.Errorf("last column should be blank as a border gap, got %q in %q", string(last), got)
	}
	if secondLast := runes[w-2]; secondLast != '…' {
		t.Errorf("ellipsis should sit one column short of the edge, got %q in %q", string(secondLast), got)
	}
}

func TestTruncatingTextView_NoEllipsisWhenFits(t *testing.T) {
	tv := NewTruncatingTextView()
	tv.SetText("3 tasks")

	got := drawToString(t, tv, 20)
	if strings.Contains(got, "…") {
		t.Errorf("text fits; no ellipsis expected, got %q", got)
	}
	if !strings.Contains(got, "3 tasks") {
		t.Errorf("expected full text drawn, got %q", got)
	}
}

func TestTruncatingTextView_RedrawAtWiderWidthRestoresFullText(t *testing.T) {
	// truncation must not be destructive: drawing narrow then wide must show
	// the full text again (the wrapper holds the original, re-truncating each
	// Draw from the source rather than mutating it permanently).
	tv := NewTruncatingTextView()
	tv.SetText("backend security urgent")

	_ = drawToString(t, tv, 10) // narrow: truncates
	wide := drawToString(t, tv, 40)
	if strings.Contains(wide, "…") {
		t.Errorf("wide redraw should show full text, got %q", wide)
	}
	if !strings.Contains(wide, "backend security urgent") {
		t.Errorf("wide redraw missing full text, got %q", wide)
	}
}
