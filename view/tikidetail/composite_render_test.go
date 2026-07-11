package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
	gradcore "github.com/boolean-maybe/tiki/internal/gradient"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestBuildCompositeText_TagEmissionShape pins the exact byte shape that
// the composite renderer produces for a representative anchor that exercises
// both the bare-role branch and the modifier branch. The two branches use
// subtly different tag-emission patterns (the modifier branch emits a
// trailing [-] reset and re-establishes the composite's default value tag,
// the bare-role branch relies on tview's auto-reset at the next tag). A
// regression that quietly drops the re-emission, or swaps the order, would
// re-color following segments incorrectly — this snapshot guards against
// that drift.
func TestBuildCompositeText_TagEmissionShape(t *testing.T) {
	// Enable gradients so the modifier branch emits its per-rune color tag
	// sweep (the more fragile, harder-to-reason-about path). Restore on exit.
	prev := gradcore.UseGradients.Load()
	gradcore.UseGradients.Store(true)
	t.Cleanup(func() { gradcore.UseGradients.Store(prev) })

	roles := theme.Roles()
	valueTag := roles.TextValue().Tag()
	mutedRole, ok := roles.ResolveByName("text.muted")
	if !ok {
		t.Fatalf("ResolveByName(text.muted) failed; theme bootstrap broken")
	}
	mutedTag := mutedRole.Tag()

	// Composite with three segments:
	//   1. literal "Status: " with bare role text.muted        → bare-role branch
	//   2. literal "[done]"   with role status.ok + modifier accent → modifier branch
	//   3. literal " (final)" with NO role                     → no-tag branch
	// Segment 3 verifies that, after the modifier branch's re-emission of
	// the composite's value tag, a following no-role segment still renders
	// in value color rather than stuck in the gradient.
	a := gridlayout.Anchor{
		Kind: gridlayout.AnchorComposite,
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentLiteral, Role: "text.muted", Text: "Status: "},
			{Kind: gridlayout.SegmentLiteral, Role: "status.ok", Modifier: "accent", Text: "[done]"},
			{Kind: gridlayout.SegmentLiteral, Text: " (final)"},
		},
	}
	tk := tikipkg.New() // no field segments referenced; tk is unused by literal segs
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: roles}

	got := buildCompositeText(a, tk, ctx)

	// 1. Output starts with the composite's default value tag.
	if !strings.HasPrefix(got, valueTag) {
		t.Errorf("expected leading TextValue tag %q; got %q", valueTag, got)
	}

	// 2. Bare-role segment writes the muted tag, then the literal verbatim.
	if !strings.Contains(got, mutedTag+"Status: ") {
		t.Errorf("expected %q substring; got %q", mutedTag+"Status: ", got)
	}

	// 3. Modifier branch produces a per-rune `[#rrggbb]` color tag sweep
	//    for "[done]" (6 runes → >=6 hex tags), then a `[-]` reset, then
	//    re-emits the value tag.
	hexTagCount := strings.Count(got, "[#")
	if hexTagCount < 6 {
		t.Errorf("expected >= 6 per-rune hex tags from modifier branch; got %d in %q", hexTagCount, got)
	}
	if !strings.Contains(got, "[-]"+valueTag+" (final)") {
		t.Errorf("expected modifier-reset + value-tag re-emission before final segment; got %q", got)
	}

	// 4. No-role segment writes its literal verbatim with no preceding tag
	//    of its own (relies on the value-tag re-emission upstream).
	if !strings.HasSuffix(got, " (final)") {
		t.Errorf("expected trailing literal %q at end; got %q", " (final)", got)
	}
}

// TestBuildCompositeText_GradientsOffDegradesToSolid pins that when the
// gradcore.UseGradients flag is false (8/16-color terminals), the modifier
// branch degrades to a single solid tag from the base role rather than a
// per-rune sweep. Same anchor as the gradient-on test for direct comparison.
func TestBuildCompositeText_GradientsOffDegradesToSolid(t *testing.T) {
	prev := gradcore.UseGradients.Load()
	gradcore.UseGradients.Store(false)
	t.Cleanup(func() { gradcore.UseGradients.Store(prev) })

	roles := theme.Roles()
	valueTag := roles.TextValue().Tag()
	okRole, ok := roles.ResolveByName("status.ok")
	if !ok {
		t.Fatalf("ResolveByName(status.ok) failed; theme bootstrap broken")
	}
	okTag := okRole.Tag()

	a := gridlayout.Anchor{
		Kind: gridlayout.AnchorComposite,
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentLiteral, Role: "status.ok", Modifier: "accent", Text: "[done]"},
		},
	}
	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: roles}

	got := buildCompositeText(a, tk, ctx)

	// Output: valueTag (composite default) + okTag + "[done]" + "[-]" + valueTag (re-emission).
	want := valueTag + okTag + "[done]" + "[-]" + valueTag
	if got != want {
		t.Errorf("gradients-off composite output mismatch\n got: %q\nwant: %q", got, want)
	}
	// The expected output has exactly 3 hex tags: the composite value tag,
	// the solid base, and the re-emitted value tag. The gradient sweep
	// (which would be one tag per rune of "[done]" → 6+) must not appear.
	// With "[done]" being 6 runes, gradients-on would push the count to >= 8.
	if n := strings.Count(got, "[#"); n != 3 {
		t.Errorf("expected exactly 3 hex tags (value + solid base + value re-emit) when gradients off; got %d in %q",
			n, got)
	}
}

// TestBuildCompositeText_RoleOnLiteralSegment verifies a literal segment with
// a bare role prefix paints its text with the role's tag. Together with
// existing modifier-branch tests above, this covers the contract that a
// `<role>"text"` segment in a composite renders with the requested color.
func TestBuildCompositeText_RoleOnLiteralSegment(t *testing.T) {
	roles := theme.Roles()
	labelTag := roles.TextLabel().Tag()

	a := gridlayout.Anchor{
		Kind: gridlayout.AnchorComposite,
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentLiteral, Text: "Status: ", Role: "text.label"},
			{Kind: gridlayout.SegmentField, Name: "status"},
		},
	}
	tk := tikipkg.New()
	tk.Set("status", "ready")
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: roles}

	got := buildCompositeText(a, tk, ctx)
	if !strings.Contains(got, labelTag+"Status: ") {
		t.Errorf("composite text missing %q before 'Status: '\n got: %q", labelTag, got)
	}
}

// TestRenderCompositePrimitive_RowSpanWraps pins that a composite anchor with
// RowSpan > 1 returns a TextView whose word-wrap is enabled, so a multi-segment
// prose paragraph (e.g. an all-literal composite with mid-text colour change)
// flows across the rows it occupies. This mirrors the contract of the literal
// prose-block path (renderLiteralAnchor RowSpan>1) for composites.
func TestRenderCompositePrimitive_RowSpanWraps(t *testing.T) {
	roles := theme.Roles()
	long := "Lorem ipsum dolor sit amet"
	tail := " consectetur adipiscing elit sed do eiusmod"
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorComposite,
		RowSpan: 3,
		ColSpan: 2,
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentLiteral, Text: long, Role: "text.muted"},
			{Kind: gridlayout.SegmentLiteral, Text: tail, Role: "text.secondary"},
		},
	}
	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: roles}

	prim := renderCompositePrimitive(a, tk, ctx)
	tv, ok := prim.(*tview.TextView)
	if !ok {
		t.Fatalf("composite prose: got %T, want *tview.TextView", prim)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(15, 10)
	tv.SetRect(0, 0, 15, 10)
	tv.Draw(screen)
	rowsWithContent := 0
	for y := 0; y < 10; y++ {
		for x := 0; x < 15; x++ {
			r, _, _, _ := screen.GetContent(x, y)
			if r != ' ' && r != 0 {
				rowsWithContent++
				break
			}
		}
	}
	if rowsWithContent < 2 {
		t.Errorf("expected wrapped composite prose to span >=2 rows at width 15, got %d", rowsWithContent)
	}
}

// TestRenderCompositePrimitive_SingleRowIsSingleLine pins that a single-row
// composite renders as a single-line cell (the truncating text view), NOT the
// word-wrapping prose-block primitive used for row-spanned composites. The
// truncating view embeds *tview.TextView; the prose block is a plain
// *tview.TextView with word-wrap on. We distinguish by asserting it is not the
// plain wrapping type and that it carries the rendered text.
func TestRenderCompositePrimitive_SingleRowIsSingleLine(t *testing.T) {
	roles := theme.Roles()
	a := gridlayout.Anchor{
		Kind:    gridlayout.AnchorComposite,
		RowSpan: 1,
		ColSpan: 1,
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentLiteral, Text: "Status: "},
			{Kind: gridlayout.SegmentField, Name: "status"},
		},
	}
	tk := tikipkg.New()
	tk.Set("status", "ready")
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: roles}

	prim := renderCompositePrimitive(a, tk, ctx)
	if _, isPlainWrap := prim.(*tview.TextView); isPlainWrap {
		t.Fatalf("single-row composite: got plain *tview.TextView (the prose-block wrapping shape), want single-line truncating view")
	}
	if got := extractTextView(prim, true); !strings.Contains(got, "Status:") {
		t.Errorf("single-row composite missing rendered text, got %q", got)
	}
}
