package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/gdamore/tcell/v2"
)

func TestListFieldCountText(t *testing.T) {
	missing := tikipkg.New()

	withThree := tikipkg.New()
	withThree.Set("tags", []string{"a", "b", "c"})

	empty := tikipkg.New()
	empty.Set("tags", []string{})

	scalar := tikipkg.New()
	scalar.Set("tags", "solo") // a scalar coerces to a one-element list

	nonCoercible := tikipkg.New()
	nonCoercible.Set("tags", map[string]any{"k": "v"})

	cases := []struct {
		name string
		tk   *tikipkg.Tiki
		want string
	}{
		{"missing field", missing, "0"},
		{"three items", withThree, "3"},
		{"empty list", empty, "0"},
		{"scalar counts as one", scalar, "1"},
		{"non-coercible map", nonCoercible, "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := listFieldCountText("tags", c.tk); got != c.want {
				t.Errorf("listFieldCountText = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRenderViewModeAnchor_CountRendersItemCount(t *testing.T) {
	tk := tikipkg.New()
	tk.Set("tags", []string{"x", "y", "z"})
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	a := gridlayout.Anchor{Kind: gridlayout.AnchorField, Name: "tags", Display: gridlayout.DisplayCount}

	got := extractTextView(RenderViewModeAnchor(a, tk, ctx), true)
	if strings.TrimSpace(got) != "3" {
		t.Errorf("count anchor rendered %q, want %q", got, "3")
	}
}

func TestRenderViewModeAnchor_CountEmptyIsZero(t *testing.T) {
	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	a := gridlayout.Anchor{Kind: gridlayout.AnchorField, Name: "tags", Display: gridlayout.DisplayCount}

	got := extractTextView(RenderViewModeAnchor(a, tk, ctx), true)
	if strings.TrimSpace(got) != "0" {
		t.Errorf("empty count anchor rendered %q, want %q", got, "0")
	}
}

func TestRenderViewModeAnchor_CountScalarIsOne(t *testing.T) {
	tk := tikipkg.New()
	tk.Set("tags", "solo")
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	a := gridlayout.Anchor{Kind: gridlayout.AnchorField, Name: "tags", Display: gridlayout.DisplayCount}

	got := extractTextView(RenderViewModeAnchor(a, tk, ctx), true)
	if strings.TrimSpace(got) != "1" {
		t.Errorf("scalar count anchor rendered %q, want %q", got, "1")
	}
}

func TestRenderViewModeAnchor_CompositeCountMissingFieldIsZero(t *testing.T) {
	// a project with no dependsOn key at all must still render "0" in a
	// composite count segment — not the "—" absent-value dash. Regression for
	// the composite path short-circuiting on the missing key before the count
	// branch.
	tk := tikipkg.New() // dependsOn never set
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	a := gridlayout.Anchor{
		Kind: gridlayout.AnchorComposite,
		Name: "dependsOn",
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentField, Name: "dependsOn", Display: gridlayout.DisplayCount},
			{Kind: gridlayout.SegmentLiteral, Text: " tasks"},
		},
	}

	got := extractTextView(RenderViewModeAnchor(a, tk, ctx), true)
	if !strings.Contains(got, "0 tasks") {
		t.Errorf("missing-field composite count rendered %q, want it to contain %q", got, "0 tasks")
	}
	if strings.Contains(got, "—") {
		t.Errorf("missing-field composite count rendered the absent dash %q, want a zero count", got)
	}
}

func TestRenderViewModeAnchor_CompositeListSegmentEmptyIsBlank(t *testing.T) {
	// a `"tags: " + tags` composite must read "tags:" (label, nothing after)
	// when the list is empty/absent — not "tags: —". The absent dash belongs to
	// standalone value rendering; inside a composite a label beside an empty
	// list should contribute no value text.
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	a := gridlayout.Anchor{
		Kind: gridlayout.AnchorComposite,
		Name: "tags",
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentLiteral, Text: "tags: "},
			{Kind: gridlayout.SegmentField, Name: "tags"},
		},
	}

	empty := tikipkg.New() // tags never set
	got := extractTextView(RenderViewModeAnchor(a, empty, ctx), true)
	if strings.Contains(got, "—") {
		t.Errorf("empty tags composite rendered the absent dash %q, want %q", got, "tags:")
	}
	if strings.TrimSpace(got) != "tags:" {
		t.Errorf("empty tags composite rendered %q, want %q", got, "tags:")
	}

	full := tikipkg.New()
	full.Set("tags", []string{"xxx", "yyy", "zzz"})
	gotFull := extractTextView(RenderViewModeAnchor(a, full, ctx), true)
	if !strings.Contains(gotFull, "tags: xxx, yyy, zzz") {
		t.Errorf("populated tags composite rendered %q, want it to contain %q", gotFull, "tags: xxx, yyy, zzz")
	}
}

func TestRenderViewModeAnchor_CompositeWithCount(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := tikipkg.New()
	tk.Set("dependsOn", []string{"AAAAAA", "BBBBBB"})
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles(), Store: s}
	a := gridlayout.Anchor{
		Kind: gridlayout.AnchorComposite,
		Name: "dependsOn",
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentLiteral, Text: "Deps: "},
			{Kind: gridlayout.SegmentField, Name: "dependsOn", Display: gridlayout.DisplayCount},
		},
	}

	got := extractTextView(RenderViewModeAnchor(a, tk, ctx), true)
	if !strings.Contains(got, "Deps: 2") {
		t.Errorf("composite count rendered %q, want it to contain %q", got, "Deps: 2")
	}
}

func TestRenderViewModeAnchor_LongCompositeTruncatesWithEllipsis(t *testing.T) {
	// a composite that overflows its drawn width must show a trailing ellipsis
	// instead of hard-clipping mid-token. Drawn into a narrow rect directly.
	tk := tikipkg.New()
	tk.Set("tags", []string{"backend", "security", "urgent", "refactor"})
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	a := gridlayout.Anchor{
		Kind: gridlayout.AnchorComposite,
		Name: "tags",
		Segments: []gridlayout.Segment{
			{Kind: gridlayout.SegmentLiteral, Text: "tags: "},
			{Kind: gridlayout.SegmentField, Name: "tags"},
		},
	}
	prim := RenderViewModeAnchor(a, tk, ctx)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(16, 1)
	prim.SetRect(0, 0, 16, 1)
	prim.Draw(screen)
	var b strings.Builder
	for x := 0; x < 16; x++ {
		r, _, _, _ := screen.GetContent(x, 0)
		if r == 0 {
			r = ' '
		}
		b.WriteRune(r)
	}
	if !strings.Contains(b.String(), "…") {
		t.Errorf("long composite should truncate with an ellipsis, got %q", b.String())
	}
}

func TestMeasureAnchor_CountMeasuresDigitWidth(t *testing.T) {
	tk := tikipkg.New()
	// a tag value far wider than its count's digit width, so the two measures
	// are unambiguously distinguishable.
	tk.Set("tags", []string{"a-very-long-tag-token"})
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}

	countAnchor := gridlayout.Anchor{Kind: gridlayout.AnchorField, Name: "tags", Display: gridlayout.DisplayCount}
	valueAnchor := gridlayout.Anchor{Kind: gridlayout.AnchorField, Name: "tags", Display: gridlayout.DisplayLabel}

	count := MeasureAnchor(countAnchor, tk, ctx)
	value := MeasureAnchor(valueAnchor, tk, ctx)
	if count >= value {
		t.Errorf("count measure %d should be smaller than value measure %d (digit width, not token width)", count, value)
	}
	if count > 2 {
		t.Errorf("count measure %d; a one-digit count should measure ~1 cell", count)
	}
}
