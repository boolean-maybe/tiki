package tikidetail

import (
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
)

// TestTikiEditView_SpecRoundTripsThroughAccessor pins that NewTikiEditView
// stores the supplied spec verbatim and exposes it through Spec() so the
// input router can re-encode it on view transitions without re-parsing the
// workflow. Layout() degrades to the spec's anchor-name slice (used by
// callers that still want a flat field list).
func TestTikiEditView_SpecRoundTripsThroughAccessor(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI200")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	spec := singleColumnSpec([]string{"title", "status", "priority"})
	ev := NewTikiEditView(s, tk.ID, nil, spec)

	got := ev.Spec()
	if got.Rows != spec.Rows || got.Cols != spec.Cols || len(got.Anchors) != len(spec.Anchors) {
		t.Fatalf("Spec() did not round-trip: got rows=%d cols=%d anchors=%d, want rows=%d cols=%d anchors=%d",
			got.Rows, got.Cols, len(got.Anchors), spec.Rows, spec.Cols, len(spec.Anchors))
	}
	if names := ev.Layout(); len(names) != 3 || names[0] != "title" || names[2] != "priority" {
		t.Errorf("Layout(): got %v, want [title status priority]", names)
	}
}

// TestTikiEditView_LiteralAnchorsRenderAsCaptions pins that the edit view
// honors the parsed grid's literal anchors (caption text) by emitting one
// primitive per anchor — a literal anchor produces a non-nil read-only
// primitive distinct from the field-anchor branch. This is the core
// unification contract: edit view and detail view both render the same
// 2D grid, including author-declared captions.
func TestTikiEditView_LiteralAnchorsRenderAsCaptions(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI201")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	// 1x2 grid: literal "Status:" caption then status field
	spec := gridlayout.GridSpec{
		Rows: 1, Cols: 2,
		Anchors: []gridlayout.Anchor{
			{Kind: gridlayout.AnchorLiteral, Text: "Status:", Row: 0, Col: 0, RowSpan: 1, ColSpan: 1},
			{Kind: gridlayout.AnchorField, Name: "status", Row: 0, Col: 1, RowSpan: 1, ColSpan: 1},
		},
		Stretcher: []bool{false, false},
		Cells: [][]gridlayout.Cell{{
			gridlayout.LiteralCell{Text: "Status:"},
			gridlayout.FieldCell{Name: "status"},
		}},
	}
	ev := NewTikiEditView(s, tk.ID, nil, spec)

	ctx := FieldRenderContext{Mode: RenderModeEdit, Roles: theme.Roles()}
	primitives := ev.buildAnchorPrimitives(tk, ctx)
	if len(primitives) != 2 {
		t.Fatalf("got %d primitives, want 2", len(primitives))
	}
	if primitives[0] == nil || primitives[1] == nil {
		t.Fatalf("nil primitive(s): caption=%v field=%v", primitives[0], primitives[1])
	}
}
