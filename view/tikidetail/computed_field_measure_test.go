package tikidetail

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/rivo/tview"
)

// TestMeasureFieldValue_ComputedTimestampMatchesRender reproduces smoke-test
// defect #4: createdAt/updatedAt are computed system fields backed by struct
// accessors (tk.CreatedAt()/UpdatedAt()), not frontmatter keys. The renderer
// reads them via the descriptor's Get func and draws "2026-06-11 15:04" (16
// cells), but the width measure read tk.Get(name) — the frontmatter map, which
// has no such key — and reported the empty "Unknown" placeholder instead. The
// solver then under-reserved the column and the rendered datetime clipped to
// "2026-06-11…". Measure must equal what the renderer draws.
func TestMeasureFieldValue_ComputedTimestampMatchesRender(t *testing.T) {
	roles := theme.Roles()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: roles}

	tk := tikipkg.New()
	created := time.Date(2026, 6, 11, 15, 4, 0, 0, time.UTC)
	tk.SetCreatedAt(created)
	tk.SetUpdatedAt(created)

	for _, name := range []string{"createdAt", "updatedAt"} {
		rendered := renderedValueWidth(t, name, tk, ctx)
		measured := MeasureFieldValue(name, tk, ctx)
		// the measured footprint reserves one breathing cell beyond the rendered
		// content so the truncating view (which draws to width-1) never clips it.
		if want := rendered + scalarBreathingCell; measured != want {
			t.Errorf("%s: MeasureFieldValue = %d, want %d (rendered %d + breathing cell)",
				name, measured, want, rendered)
		}
	}
}

// TestMeasureFieldValue_EmptyComputedTimestampMatchesRender pins the empty side:
// a tiki with no createdAt must measure exactly the "Unknown" placeholder the
// renderer draws, so the column is neither starved nor over-reserved.
func TestMeasureFieldValue_EmptyComputedTimestampMatchesRender(t *testing.T) {
	roles := theme.Roles()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: roles}
	tk := tikipkg.New() // createdAt/updatedAt left zero

	for _, name := range []string{"createdAt", "updatedAt"} {
		rendered := renderedValueWidth(t, name, tk, ctx)
		measured := MeasureFieldValue(name, tk, ctx)
		if want := rendered + scalarBreathingCell; measured != want {
			t.Errorf("%s (empty): MeasureFieldValue = %d, want %d (rendered %d + breathing cell)",
				name, measured, want, rendered)
		}
	}
}

// renderedValueWidth renders a field through the registry and returns the
// visible width of the drawn text — the ground truth the measure must match.
func renderedValueWidth(t *testing.T, name string, tk *tikipkg.Tiki, ctx FieldRenderContext) int {
	t.Helper()
	fd, ok := LookupField(name)
	if !ok {
		t.Fatalf("LookupField(%q) failed", name)
	}
	ui, ok := LookupType(fd.Semantic)
	if !ok {
		t.Fatalf("LookupType(%q) failed", fd.Semantic)
	}
	rctx := withFieldDescriptor(ctx, fd)
	prim := ui.Render(tk, rctx)
	tv, ok := prim.(*tview.TextView)
	if !ok {
		// truncating text view embeds *tview.TextView; fall back to its text.
		if gt, ok := prim.(interface{ GetText(bool) string }); ok {
			return tview.TaggedStringWidth(gt.GetText(false))
		}
		t.Fatalf("%s: unexpected render primitive %T", name, prim)
	}
	return tview.TaggedStringWidth(tv.GetText(false))
}
