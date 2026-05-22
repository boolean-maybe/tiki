package gridbox

import (
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func unitHeight(string, int) int { return 1 }

// singleColumnSpecForTest synthesizes a 1-column grid from a name list.
// Mirrors the test helper that previously lived in package tikidetail.
func singleColumnSpecForTest(names []string) gridlayout.GridSpec {
	if len(names) == 0 {
		return gridlayout.GridSpec{}
	}
	anchors := make([]gridlayout.Anchor, len(names))
	cells := make([][]gridlayout.Cell, len(names))
	for i, n := range names {
		anchors[i] = gridlayout.Anchor{
			Name: n, Row: i, Col: 0, RowSpan: 1, ColSpan: 1,
		}
		cells[i] = []gridlayout.Cell{gridlayout.FieldCell{Name: n}}
	}
	return gridlayout.GridSpec{
		Rows:      len(names),
		Cols:      1,
		Anchors:   anchors,
		Stretcher: []bool{false},
		Cells:     cells,
	}
}

func TestContainer_RebuildOnWidthChange(t *testing.T) {
	spec := singleColumnSpecForTest([]string{"a", "b"})
	primitives := []tview.Primitive{
		tview.NewTextView(),
		tview.NewTextView(),
	}
	g := NewContainer(spec, primitives, unitHeight)

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

func TestContainer_EmptySpec(t *testing.T) {
	g := NewContainer(singleColumnSpecForTest(nil), nil, unitHeight)
	g.rebuild(80)
	// No panic, lastWidth tracked.
	if g.lastWidth != 80 {
		t.Errorf("empty spec rebuild: lastWidth = %d, want 80", g.lastWidth)
	}
}

// TestContainer_HasFocusBeforeFirstDraw pins a subtle event-routing
// invariant: when a child primitive holds focus but Container's inner
// Flex has not yet been populated (rebuild() runs lazily on first Draw),
// Container must still report HasFocus() == true.
//
// Why this matters: tview.Application.Run gates the entire input handler
// on root.HasFocus(). The dispatch tree walks Pages → outer Flex → ...
// → Container, asking each level whether any descendant is focused. If
// Container returns false because g.Flex.items is empty (cold), the
// keystroke is silently dropped — even though the InputField below
// Container is the focused primitive. This is the bug behind the
// "first character swallowed when pressing n" report.
func TestContainer_HasFocusBeforeFirstDraw(t *testing.T) {
	spec := singleColumnSpecForTest([]string{"title"})
	input := tview.NewInputField()
	g := NewContainer(spec, []tview.Primitive{input}, unitHeight)

	// Simulate Application.SetFocus on the inner primitive without
	// having drawn the container yet. tview's SetFocus marks the
	// primitive as focused via Box.focus = true.
	input.Focus(func(p tview.Primitive) {})

	if !g.HasFocus() {
		t.Fatal("Container.HasFocus() = false before first Draw, want true (focused child must be reported)")
	}
}

// TestContainer_InputHandlerForwardsBeforeFirstDraw pins the matching
// invariant for InputHandler: the handler must forward keystrokes to
// whichever primitive in g.primitives currently holds focus, even when
// g.Flex.items is empty (cold container, before first Draw).
func TestContainer_InputHandlerForwardsBeforeFirstDraw(t *testing.T) {
	spec := singleColumnSpecForTest([]string{"title"})
	input := tview.NewInputField()
	g := NewContainer(spec, []tview.Primitive{input}, unitHeight)

	input.Focus(func(p tview.Primitive) {})

	handler := g.InputHandler()
	if handler == nil {
		t.Fatal("Container.InputHandler() returned nil")
	}
	ev := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	handler(ev, func(p tview.Primitive) {})

	if got := input.GetText(); got != "x" {
		t.Errorf("after forwarded 'x' keystroke: InputField text = %q, want %q", got, "x")
	}
}

// TestContainer_AnchorPlacementHeight_RowSpannedLiteral pins the height
// allocation for row-spanned literal anchors: the prose-block renderer
// (renderLiteralAnchor) needs vertical room equal to the declared RowSpan
// to draw multiple wrapped lines. Without this, the literal renders as a
// single line at the top of its band and the rest of the spanned rows go
// blank — which is exactly the bug visible in tiki-epic-detail-5.png.
func TestContainer_AnchorPlacementHeight_RowSpannedLiteral(t *testing.T) {
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorLiteral,
		Text:    "Lorem ipsum dolor sit",
		Row:     2,
		Col:     0,
		RowSpan: 3,
		ColSpan: 1,
	}
	spec := gridlayout.GridSpec{
		Rows:      5,
		Cols:      1,
		Anchors:   []gridlayout.Anchor{a},
		Stretcher: []bool{false},
	}
	g := NewContainer(spec, []tview.Primitive{tview.NewTextView()}, unitHeight)
	plan := gridlayout.Plan{
		Rows:         5,
		Cols:         1,
		ColumnWidths: []int{20},
		RowHeights:   []int{1, 1, 1, 1, 1},
		Dropped:      []bool{false},
	}
	got := g.anchorPlacementHeight(a, plan)
	if got != 3 {
		t.Errorf("row-spanned literal height = %d, want 3 (RowSpan)", got)
	}
}

// TestContainer_AnchorPlacementHeight_SingleRowLiteral pins the existing
// fixed-1 behavior for single-row literals, so the row-spanned override
// does not regress short captions like "Status:".
func TestContainer_AnchorPlacementHeight_SingleRowLiteral(t *testing.T) {
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorLiteral,
		Text:    "Status:",
		RowSpan: 1,
		ColSpan: 1,
	}
	spec := gridlayout.GridSpec{
		Rows:      1,
		Cols:      1,
		Anchors:   []gridlayout.Anchor{a},
		Stretcher: []bool{false},
	}
	g := NewContainer(spec, []tview.Primitive{tview.NewTextView()}, unitHeight)
	plan := gridlayout.Plan{
		Rows: 1, Cols: 1,
		ColumnWidths: []int{20},
		RowHeights:   []int{1},
		Dropped:      []bool{false},
	}
	got := g.anchorPlacementHeight(a, plan)
	if got != 1 {
		t.Errorf("single-row literal height = %d, want 1", got)
	}
}

// TestContainer_SpanningRowHonorsRowSpan pins that a row containing both
// a horizontal-span anchor (ColSpan>1) AND a vertical-span anchor
// (RowSpan>1) renders the row-spanned anchor with full vertical height.
//
// Before the fix: rebuild() partitioned strictly per-row. A row with any
// ColSpan>1 anchor went through addSpanningRow with height=plan.RowHeights[r]
// (always 1 row), and the next 1..RowSpan-1 rows fell into the next packed
// band where they painted spacers underneath the already-drawn literal.
// Net effect: the row-spanned anchor was clipped to a single line.
//
// After the fix: when a spanning-row contains row-spanned anchors, the
// row consumes max(RowSpan) rows of vertical space and the iteration in
// rebuild() advances past those rows so they aren't double-rendered.
func TestContainer_SpanningRowHonorsRowSpan(t *testing.T) {
	// 4-col, 3-row grid. Row 0 has both a ColSpan=2 horizontal-span and
	// a separate RowSpan=3 literal. Without the fix, the literal renders
	// in 1 row; rows 1-2 get a packed band with spacers in col 3.
	raw := [][]string{
		{`<highlight>title`, "--", "--", `"Lorem ipsum dolor"`},
		{"a", `"Caption:"`, "b", "^"},
		{"c", `"Caption2:"`, "d", "^"},
	}
	spec, err := gridlayout.ParseGrid(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Find the literal anchor; assert it's row=0 col=3 with RowSpan=3.
	var lit *gridlayout.Anchor
	for i := range spec.Anchors {
		a := &spec.Anchors[i]
		if a.Kind == gridlayout.AnchorLiteral && a.Row == 0 && a.Col == 3 {
			lit = a
			break
		}
	}
	if lit == nil {
		t.Fatalf("literal at row 0 col 3 not found in parsed anchors")
	}
	if lit.RowSpan != 3 {
		t.Fatalf("literal RowSpan = %d, want 3", lit.RowSpan)
	}

	prims := make([]tview.Primitive, len(spec.Anchors))
	for i := range prims {
		prims[i] = tview.NewTextView()
	}
	g := NewContainer(spec, prims, unitHeight)
	plan := SolveGridLayout(120, spec, unitHeight)

	// The placement-height for the literal must be its RowSpan, not 1.
	got := g.anchorPlacementHeight(*lit, plan)
	if got != 3 {
		t.Errorf("anchorPlacementHeight(literal) = %d, want 3", got)
	}

	// rebuild() must produce a Flex hierarchy where the spanning row's
	// outer Flex item gets enough rows of vertical space (>= 3) to host
	// the row-spanned literal. We probe by drawing into a simulation
	// screen at known dimensions and counting the rows that contain
	// content from the literal in col 3.
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 10)
	g.SetRect(0, 0, 80, 10)
	g.Draw(screen)

	// Confirm row-iteration in rebuild advanced past rows 1-2 instead of
	// double-rendering them. We inspect the inner Flex's item count: a
	// correct fix produces 2 child bands (spanning band + packed band
	// for any non-spanning rows), not 3 (per-row partitioning).
	// In this 3-row spec, ALL rows are inside the spanning band's
	// vertical span, so we expect exactly 1 child band.
	itemCount := g.GetItemCount()
	if itemCount != 1 {
		t.Errorf("Flex.GetItemCount() = %d, want 1 (single spanning band consuming all 3 rows)", itemCount)
	}
}

// TestContainer_AnchorPlacementHeight verifies that:
//   - Literal anchors always get height 1, regardless of heightOf.
//   - Field anchors get their natural heightOf result, NOT the solver's
//     grown row-band sum. This is the row-packing fix — short field
//     anchors must not get padded when a sibling column grows the row
//     band for a multi-line value.
func TestContainer_AnchorPlacementHeight(t *testing.T) {
	spec, err := gridlayout.ParseGrid([][]string{
		{"Status:", "status", "Tags:", "tags"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	heightOf := func(name string, w int) int {
		if name == "tags" {
			return 3
		}
		return 1
	}
	g := NewContainer(spec, make([]tview.Primitive, len(spec.Anchors)), heightOf)
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
