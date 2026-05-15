package taskdetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/rivo/tview"
)

func unitHeight(string, int) int { return 1 }

func TestGridContainer_RebuildOnWidthChange(t *testing.T) {
	spec := singleColumnSpec([]string{"a", "b"})
	primitives := []tview.Primitive{
		tview.NewTextView(),
		tview.NewTextView(),
	}
	g := newGridContainer(spec, primitives, unitHeight)

	g.rebuild(120)
	if g.lastWidth != 120 {
		t.Errorf("after rebuild(120): lastWidth = %d, want 120", g.lastWidth)
	}
	g.rebuild(60)
	if g.lastWidth != 60 {
		t.Errorf("after rebuild(60): lastWidth = %d, want 60", g.lastWidth)
	}
	// Same-width rebuild remains valid (idempotent).
	g.rebuild(60)
	if g.lastWidth != 60 {
		t.Errorf("after idempotent rebuild: lastWidth = %d, want 60", g.lastWidth)
	}
}

func TestGridContainer_EmptySpec(t *testing.T) {
	g := newGridContainer(singleColumnSpec(nil), nil, unitHeight)
	g.rebuild(80)
	// No panic, lastWidth tracked.
	if g.lastWidth != 80 {
		t.Errorf("empty spec rebuild: lastWidth = %d, want 80", g.lastWidth)
	}
}

// TestRenderLiteralCaption pins the literal-caption primitive shape:
// a TextView whose unformatted text equals the literal string. The
// dim-label color tag is included in the formatted text but stripped
// when GetText(false) drops the tags.
func TestRenderLiteralCaption(t *testing.T) {
	colors := config.GetColors()
	prim := renderLiteralCaption("Status:", colors)
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("renderLiteralCaption returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(true)
	if !strings.Contains(got, "Status:") {
		t.Errorf("literal text not found in rendered view: got %q, want substring %q", got, "Status:")
	}
}

// TestRenderLiteralCaption_ExpandsRoleMarkup pins that `<role>` color
// markup in a caption resolves to a tview color tag, that the literal
// text after the token survives, and that the role span closes with the
// `[-]` reset emitted by workflow.ExpandVisual.
func TestRenderLiteralCaption_ExpandsRoleMarkup(t *testing.T) {
	colors := config.GetColors()
	dangerTag := colors.DangerColor.Tag().String()

	prim := renderLiteralCaption("<danger>!!!", colors)
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("renderLiteralCaption returned %T, want *tview.TextView", prim)
	}
	got := tv.GetText(false) // keep style tags (stripAllTags=false)
	if !strings.Contains(got, dangerTag) {
		t.Errorf("expected danger color tag %q in rendered text, got %q", dangerTag, got)
	}
	if !strings.Contains(got, "!!!") {
		t.Errorf("expected literal text after role token in rendered text, got %q", got)
	}
	if !strings.Contains(got, "[-]") {
		t.Errorf("expected reset tag '[-]' after role span, got %q", got)
	}
}

// TestGridContainer_LiteralAnchorRendersAsCaption parses a grid that mixes
// literal captions with field anchors, builds primitives for each, and
// verifies rebuild lays them out in the expected slot order without
// panicking on either kind.
func TestGridContainer_LiteralAnchorRendersAsCaption(t *testing.T) {
	colors := config.GetColors()
	spec, err := gridlayout.ParseGrid([][]string{
		{"Status:", "status", "Tags:", "tags"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	primitives := make([]tview.Primitive, len(spec.Anchors))
	for i, a := range spec.Anchors {
		if a.Kind == gridlayout.AnchorLiteral {
			primitives[i] = renderLiteralCaption(a.Text, colors)
		} else {
			primitives[i] = tview.NewTextView().SetText("[val:" + a.Name + "]")
		}
	}
	g := newGridContainer(spec, primitives, unitHeight)
	g.rebuild(120)
	if g.lastWidth != 120 {
		t.Errorf("after rebuild: lastWidth = %d, want 120", g.lastWidth)
	}
}

// TestGridContainer_AnchorPlacementHeight verifies that:
//   - Literal anchors always get height 1, regardless of heightOf.
//   - Field anchors get their natural heightOf result, NOT the solver's
//     grown row-band sum. This is the Part D row-packing fix — short
//     field anchors must not get padded when a sibling column grows the
//     row band for a multi-line value.
func TestGridContainer_AnchorPlacementHeight(t *testing.T) {
	spec, err := gridlayout.ParseGrid([][]string{
		{"Status:", "status", "Tags:", "tags"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Simulate "tags wraps to 3 lines" while other fields are single-row.
	heightOf := func(name string, w int) int {
		if name == "tags" {
			return 3
		}
		return 1
	}
	g := newGridContainer(spec, make([]tview.Primitive, len(spec.Anchors)), heightOf)
	plan := SolveGridLayout(120, spec, heightOf)

	cases := []struct {
		idx        int
		wantHeight int
		why        string
	}{
		{0, 1, "literal caption Status: always 1 row"},
		{1, 1, "status is single-row even though tags grew the row band"},
		{2, 1, "literal caption Tags: always 1 row"},
		{3, 3, "tags has natural height 3"},
	}
	for _, c := range cases {
		a := spec.Anchors[c.idx]
		got := g.anchorPlacementHeight(a, plan)
		if got != c.wantHeight {
			t.Errorf("anchor[%d] (%s): got height %d, want %d (%s)",
				c.idx, a.Name+a.Text, got, c.wantHeight, c.why)
		}
	}
}
