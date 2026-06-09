package gridlayout

import (
	"slices"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/theme"
	"gopkg.in/yaml.v3"
)

func TestTokenizeCell(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
		check   func(t *testing.T, c Cell)
	}{
		{in: "", wantErr: true},
		{in: "  ", wantErr: true},
		{in: "--", check: func(t *testing.T, c Cell) {
			if _, ok := c.(ColSpanCell); !ok {
				t.Errorf("want ColSpanCell, got %T", c)
			}
		}},
		{in: "_", check: func(t *testing.T, c Cell) {
			if _, ok := c.(EmptyCell); !ok {
				t.Errorf("want EmptyCell, got %T", c)
			}
		}},
		{in: "<->", check: func(t *testing.T, c Cell) {
			// retired marker: falls through to literal text now.
			if _, ok := c.(LiteralCell); !ok {
				t.Errorf("want LiteralCell (<-> retired), got %T", c)
			}
		}},
		{in: "status", check: func(t *testing.T, c Cell) {
			fc, ok := c.(FieldCell)
			if !ok || fc.Name != "status" || fc.Sizing != (Sizing{Mode: SizeAuto}) {
				t.Errorf("want FieldCell{status,auto}, got %+v", c)
			}
		}},
		{in: "tags:30", check: func(t *testing.T, c Cell) {
			fc, ok := c.(FieldCell)
			if !ok || fc.Name != "tags" || fc.Sizing != (Sizing{Mode: SizeFixed, Min: 30, Max: 30}) {
				t.Errorf("want FieldCell{tags,fixed30}, got %+v", c)
			}
		}},
		{in: "createdAt", check: func(t *testing.T, c Cell) {
			fc, ok := c.(FieldCell)
			if !ok || fc.Name != "createdAt" {
				t.Errorf("want FieldCell{createdAt}, got %+v", c)
			}
		}},
		{in: "tags:0", wantErr: true},
		{in: "^", check: func(t *testing.T, c Cell) {
			if _, ok := c.(RowSpanCell); !ok {
				t.Errorf("want RowSpanCell, got %T", c)
			}
		}},
		// Strings that are not bare markers and not valid field identifiers
		// become literal-text cells. This intentionally permits typos like
		// `tags:-5` to fall through as literal text rather than erroring —
		// authors will see the literal on screen and fix it; the alternative
		// would conflict with accepting `"Status:"` and similar captions.
		{in: "Status:", check: func(t *testing.T, c Cell) {
			lc, ok := c.(LiteralCell)
			if !ok || lc.Text != "Status:" {
				t.Errorf("want LiteralCell{Status:}, got %+v", c)
			}
		}},
		{in: `"Status:"`, check: func(t *testing.T, c Cell) {
			lc, ok := c.(LiteralCell)
			if !ok || lc.Text != "Status:" {
				t.Errorf("want LiteralCell{Status:} (quotes stripped), got %+v", c)
			}
		}},
		{in: `"Tags:"`, check: func(t *testing.T, c Cell) {
			lc, ok := c.(LiteralCell)
			if !ok || lc.Text != "Tags:" {
				t.Errorf("want LiteralCell{Tags:} (quotes stripped), got %+v", c)
			}
		}},
		{in: "Done!", check: func(t *testing.T, c Cell) {
			lc, ok := c.(LiteralCell)
			if !ok || lc.Text != "Done!" {
				t.Errorf("want LiteralCell{Done!}, got %+v", c)
			}
		}},
		{in: "foo bar", check: func(t *testing.T, c Cell) {
			lc, ok := c.(LiteralCell)
			if !ok || lc.Text != "foo bar" {
				t.Errorf("want LiteralCell{foo bar}, got %+v", c)
			}
		}},
		{in: "  Tag List  ", check: func(t *testing.T, c Cell) {
			lc, ok := c.(LiteralCell)
			if !ok || lc.Text != "Tag List" {
				t.Errorf("want LiteralCell{Tag List} (outer whitespace trimmed, inner preserved), got %+v", c)
			}
		}},
		{in: "<highlight>title", check: func(t *testing.T, c Cell) {
			fc, ok := c.(FieldCell)
			if !ok || fc.Name != "title" || fc.Role != "highlight" || fc.Sizing != (Sizing{Mode: SizeAuto}) {
				t.Errorf("want FieldCell{title, highlight, auto}, got %+v", c)
			}
		}},
		{in: "<accent>status:20", check: func(t *testing.T, c Cell) {
			fc, ok := c.(FieldCell)
			if !ok || fc.Name != "status" || fc.Role != "accent" || fc.Sizing != (Sizing{Mode: SizeFixed, Min: 20, Max: 20}) {
				t.Errorf("want FieldCell{status, accent, fixed20}, got %+v", c)
			}
		}},
		{in: "<highlight>Status:", check: func(t *testing.T, c Cell) {
			lc, ok := c.(LiteralCell)
			if !ok || lc.Text != "Status:" || lc.Role != "highlight" {
				t.Errorf("want LiteralCell{Text:Status: Role:highlight}, got %+v", c)
			}
		}},
		{in: "<highlight>", check: func(t *testing.T, c Cell) {
			lc, ok := c.(LiteralCell)
			if !ok || lc.Text != "<highlight>" {
				t.Errorf("want LiteralCell{<highlight>}, got %+v", c)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := TokenizeCell(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error for %q, got %+v", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

func TestParseGrid_Empty(t *testing.T) {
	if _, err := ParseGrid(nil); err == nil {
		t.Error("want error for nil grid")
	}
	if _, err := ParseGrid([][]string{}); err == nil {
		t.Error("want error for empty grid")
	}
	if _, err := ParseGrid([][]string{{}}); err == nil {
		t.Error("want error for empty row")
	}
}

func TestParseGrid_RaggedRows(t *testing.T) {
	_, err := ParseGrid([][]string{
		{"status", "type"},
		{"priority"},
	})
	if err == nil || !strings.Contains(err.Error(), "row 1") {
		t.Errorf("want row-shape error, got %v", err)
	}
}

func TestParseGrid_OrphanColSpan(t *testing.T) {
	_, err := ParseGrid([][]string{{"--", "status"}})
	if err == nil || !strings.Contains(err.Error(), "orphan '--'") {
		t.Errorf("want orphan col-span error, got %v", err)
	}
}

func TestParseGrid_OrphanRowSpan(t *testing.T) {
	_, err := ParseGrid([][]string{{"^"}, {"status"}})
	if err == nil || !strings.Contains(err.Error(), "orphan row-span") {
		t.Errorf("want orphan row-span error for '^', got %v", err)
	}
}

func TestParseGrid_DuplicateFieldAllowed(t *testing.T) {
	// A field may appear more than once (e.g. a caption cell + a value cell,
	// or the same value rendered twice). The DSL trusts the layout author.
	spec, err := ParseGrid([][]string{
		{"status.caption", "status"},
	})
	if err != nil {
		t.Fatalf("duplicate field should be allowed, got error: %v", err)
	}
	if len(spec.Anchors) != 2 {
		t.Errorf("want 2 anchors (caption + value), got %d", len(spec.Anchors))
	}
}

func TestParseGrid_CanonicalExample(t *testing.T) {
	spec, err := ParseGrid([][]string{
		{"title", "--", "--", "--", "--"},
		{"status", "assignee", "_", "tags:30", "depends:25"},
		{"type", "createdBy", "_", "^", "^"},
		{"priority", "createdAt", "_", "_", "_"},
		{"points", "updatedAt", "_", "_", "_"},
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if spec.Rows != 5 || spec.Cols != 5 {
		t.Errorf("dimensions: got rows=%d cols=%d, want 5x5", spec.Rows, spec.Cols)
	}
	auto := Sizing{Mode: SizeAuto}
	want := []struct {
		Name     string
		Row, Col int
		RS, CS   int
		SZ       Sizing
	}{
		{"title", 0, 0, 1, 5, auto},
		{"status", 1, 0, 1, 1, auto},
		{"assignee", 1, 1, 1, 1, auto},
		{"tags", 1, 3, 2, 1, Sizing{Mode: SizeFixed, Min: 30, Max: 30}},
		{"depends", 1, 4, 2, 1, Sizing{Mode: SizeFixed, Min: 25, Max: 25}},
		{"type", 2, 0, 1, 1, auto},
		{"createdBy", 2, 1, 1, 1, auto},
		{"priority", 3, 0, 1, 1, auto},
		{"createdAt", 3, 1, 1, 1, auto},
		{"points", 4, 0, 1, 1, auto},
		{"updatedAt", 4, 1, 1, 1, auto},
	}
	if len(spec.Anchors) != len(want) {
		t.Fatalf("anchor count: got %d, want %d (anchors=%+v)", len(spec.Anchors), len(want), spec.Anchors)
	}
	for i, w := range want {
		a := spec.Anchors[i]
		if a.Name != w.Name || a.Row != w.Row || a.Col != w.Col || a.RowSpan != w.RS || a.ColSpan != w.CS || a.Sizing != w.SZ {
			t.Errorf("anchor[%d]: got %+v, want name=%s row=%d col=%d rs=%d cs=%d sz=%+v", i, a, w.Name, w.Row, w.Col, w.RS, w.CS, w.SZ)
		}
	}
}

func TestParseGrid_LiteralAnchor(t *testing.T) {
	spec, err := ParseGrid([][]string{
		{"Status:", "status", "Tags:", "tags"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, want := len(spec.Anchors), 4; got != want {
		t.Fatalf("anchor count: got %d, want %d", got, want)
	}
	cases := []struct {
		kind AnchorKind
		name string
		text string
	}{
		{AnchorLiteral, "", "Status:"},
		{AnchorField, "status", ""},
		{AnchorLiteral, "", "Tags:"},
		{AnchorField, "tags", ""},
	}
	for i, want := range cases {
		a := spec.Anchors[i]
		if a.Kind != want.kind || a.Name != want.name || a.Text != want.text {
			t.Errorf("anchor[%d]: got kind=%v name=%q text=%q, want kind=%v name=%q text=%q",
				i, a.Kind, a.Name, a.Text, want.kind, want.name, want.text)
		}
	}
}

func TestParseGrid_LiteralAnchorNamesExcludesLiterals(t *testing.T) {
	spec, err := ParseGrid([][]string{
		{"Status:", "status", "Tags:", "tags"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := spec.AnchorNames()
	want := []string{"status", "tags"}
	if len(got) != len(want) {
		t.Fatalf("AnchorNames: got %v, want %v", got, want)
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("[%d]: got %q, want %q", i, got[i], n)
		}
	}
}

func TestParseGrid_CaretRowSpan(t *testing.T) {
	spec, err := ParseGrid([][]string{
		{"tags:30"},
		{"^"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(spec.Anchors) != 1 {
		t.Fatalf("anchor count: got %d, want 1", len(spec.Anchors))
	}
	a := spec.Anchors[0]
	if a.RowSpan != 2 {
		t.Errorf("rowspan: got %d, want 2", a.RowSpan)
	}
}

// TestYAML_BareMarkers verifies the unquoted-marker contract: --, _, ^ are
// bare-legal in YAML flow context, while bare `|` is rejected (it is YAML's
// block-scalar indicator). This test pins both halves of the contract
// end-to-end: YAML → tokenizer.
func TestYAML_BareMarkers(t *testing.T) {
	// Round-trip a grid with the bare markers in valid positions:
	//   row 0: title spans columns 0-1 via `--`; status is a separate anchor; col 3 is `_` (empty)
	//   row 1: title continues into col 0-1 via `^`; col 2 is `_` (empty); col 3 is `_` (empty)
	src := `m:
  - [title, --, status, _]
  - [^,     ^,  _,      _]
`
	var out struct {
		M [][]string `yaml:"m"`
	}
	if err := yaml.Unmarshal([]byte(src), &out); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	spec, err := ParseGrid(out.M)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Per-cell expected types — the bare YAML tokens have to round-trip
	// to these struct types, otherwise the contract is broken.
	wantCells := [][]Cell{
		{FieldCell{Name: "title"}, ColSpanCell{}, FieldCell{Name: "status"}, EmptyCell{}},
		{RowSpanCell{}, RowSpanCell{}, EmptyCell{}, EmptyCell{}},
	}
	for r, row := range wantCells {
		for c, want := range row {
			got := spec.Cells[r][c]
			if got != want {
				t.Errorf("cell[%d][%d]: got %T %+v, want %T %+v", r, c, got, got, want, want)
			}
		}
	}

	// Confirm bare `|` rejected by YAML in flow context — pin the contract
	// that motivated adding `^` as the preferred row-span marker.
	badSrc := `m: [|]`
	var bad struct {
		M []string `yaml:"m"`
	}
	if err := yaml.Unmarshal([]byte(badSrc), &bad); err == nil {
		t.Errorf("expected YAML error for bare '|' in flow context, got nil (parsed as %v)", bad.M)
	}
}

func TestTokenizeCell_Composite(t *testing.T) {
	cases := []struct {
		in       string
		wantErr  bool
		segments int
		check    func(t *testing.T, c CompositeCell)
	}{
		{
			in:       `status.visual + " " + status.label`,
			segments: 3,
			check: func(t *testing.T, c CompositeCell) {
				if c.Segments[0].Kind != SegmentField || c.Segments[0].Name != "status" || c.Segments[0].Display != DisplayVisual {
					t.Errorf("seg[0]: got %+v", c.Segments[0])
				}
				if c.Segments[1].Kind != SegmentLiteral || c.Segments[1].Text != " " {
					t.Errorf("seg[1]: got %+v", c.Segments[1])
				}
				if c.Segments[2].Kind != SegmentField || c.Segments[2].Name != "status" || c.Segments[2].Display != DisplayLabel {
					t.Errorf("seg[2]: got %+v", c.Segments[2])
				}
			},
		},
		{
			in:       `<dim>priority.visual + " - " + priority.label`,
			segments: 3,
			check: func(t *testing.T, c CompositeCell) {
				if c.Segments[0].Role != "dim" || c.Segments[0].Name != "priority" || c.Segments[0].Display != DisplayVisual {
					t.Errorf("seg[0]: got %+v", c.Segments[0])
				}
				if c.Segments[1].Text != " - " {
					t.Errorf("seg[1]: got %+v", c.Segments[1])
				}
				if c.Segments[2].Name != "priority" || c.Segments[2].Display != DisplayLabel {
					t.Errorf("seg[2]: got %+v", c.Segments[2])
				}
			},
		},
		{
			in:       `createdBy + " on " + createdAt`,
			segments: 3,
			check: func(t *testing.T, c CompositeCell) {
				if c.Segments[0].Name != "createdBy" {
					t.Errorf("seg[0]: got %+v", c.Segments[0])
				}
				if c.Segments[1].Text != " on " {
					t.Errorf("seg[1]: got %+v", c.Segments[1])
				}
				if c.Segments[2].Name != "createdAt" {
					t.Errorf("seg[2]: got %+v", c.Segments[2])
				}
			},
		},
		{
			in:       `"A" + " " + "B"`,
			segments: 3, // all-literal composites are valid; per-segment roles enable multi-color prose
			check: func(t *testing.T, c CompositeCell) {
				for i, seg := range c.Segments {
					if seg.Kind != SegmentLiteral {
						t.Errorf("seg[%d] Kind = %v, want SegmentLiteral", i, seg.Kind)
					}
				}
				if c.Segments[0].Text != "A" || c.Segments[1].Text != " " || c.Segments[2].Text != "B" {
					t.Errorf("texts: got %q/%q/%q", c.Segments[0].Text, c.Segments[1].Text, c.Segments[2].Text)
				}
			},
		},
		{
			// segment-level sizing is rejected; sizing belongs on the cell.
			in:      `status:15 + " " + status.visual`,
			wantErr: true,
		},
		{
			// bare trailing sizing (no parens) is also rejected.
			in:      `status.label + " " + status.visual:16..`,
			wantErr: true,
		},
		{
			in:       `(status.label + " " + status.visual):16..`,
			segments: 3,
			check: func(t *testing.T, c CompositeCell) {
				want := Sizing{Mode: SizeAuto, Min: 16, MinSet: true}
				if c.Sizing != want {
					t.Errorf("cell sizing: got %+v, want %+v", c.Sizing, want)
				}
				if c.Segments[0].Name != "status" || c.Segments[0].Display != DisplayLabel {
					t.Errorf("seg[0]: got %+v", c.Segments[0])
				}
			},
		},
		{
			in:       `(createdBy + " on " + createdAt):fr`,
			segments: 3,
			check: func(t *testing.T, c CompositeCell) {
				want := Sizing{Mode: SizeGrow, Weight: 1}
				if c.Sizing != want {
					t.Errorf("cell sizing: got %+v, want %+v", c.Sizing, want)
				}
			},
		},
		{
			in:       `(status.label + " " + status.visual):24`,
			segments: 3,
			check: func(t *testing.T, c CompositeCell) {
				want := Sizing{Mode: SizeFixed, Min: 24, Max: 24}
				if c.Sizing != want {
					t.Errorf("cell sizing: got %+v, want %+v", c.Sizing, want)
				}
			},
		},
		{
			// bare parens with no suffix → auto (inert), still a composite.
			in:       `(status.label + " " + status.visual)`,
			segments: 3,
			check: func(t *testing.T, c CompositeCell) {
				if c.Sizing != (Sizing{Mode: SizeAuto}) {
					t.Errorf("cell sizing: got %+v, want auto", c.Sizing)
				}
			},
		},
		{
			// unbalanced parens → error.
			in:      `(status.label + " " + status.visual`,
			wantErr: true,
		},
		{
			// a quoted literal containing parens/colon must not trip the peel.
			in:       `status.label + " (x:y) " + status.visual`,
			segments: 3,
			check: func(t *testing.T, c CompositeCell) {
				if c.Segments[1].Kind != SegmentLiteral || c.Segments[1].Text != " (x:y) " {
					t.Errorf("seg[1]: got %+v", c.Segments[1])
				}
				if c.Sizing != (Sizing{Mode: SizeAuto}) {
					t.Errorf("cell sizing: got %+v, want auto", c.Sizing)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := TokenizeCell(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			cc, ok := got.(CompositeCell)
			if !ok {
				t.Fatalf("want CompositeCell, got %T %+v", got, got)
			}
			if len(cc.Segments) != tc.segments {
				t.Fatalf("segment count: got %d, want %d", len(cc.Segments), tc.segments)
			}
			if tc.check != nil {
				tc.check(t, cc)
			}
		})
	}
}

func TestParseGrid_CompositeAnchor(t *testing.T) {
	spec, err := ParseGrid([][]string{
		{`status.visual + " " + status.label`, "--", "tags"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(spec.Anchors) != 2 {
		t.Fatalf("anchor count: got %d, want 2", len(spec.Anchors))
	}
	a := spec.Anchors[0]
	if a.Kind != AnchorComposite {
		t.Errorf("want AnchorComposite, got %v", a.Kind)
	}
	if a.Name != "status" {
		t.Errorf("want Name=status (single-field), got %q", a.Name)
	}
	if a.ColSpan != 2 {
		t.Errorf("want ColSpan=2, got %d", a.ColSpan)
	}
	if len(a.Segments) != 3 {
		t.Errorf("want 3 segments, got %d", len(a.Segments))
	}
}

func TestParseGrid_CompositeCellSizing(t *testing.T) {
	spec, err := ParseGrid([][]string{
		{`(status.label + " " + status.visual):16..`},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := spec.Anchors[0]
	if a.Kind != AnchorComposite {
		t.Fatalf("want AnchorComposite, got %v", a.Kind)
	}
	want := Sizing{Mode: SizeAuto, Min: 16, MinSet: true}
	if a.Sizing != want {
		t.Errorf("anchor sizing: got %+v, want %+v", a.Sizing, want)
	}
}

func TestParseGrid_CompositeMultiField(t *testing.T) {
	spec, err := ParseGrid([][]string{
		{`createdBy + " on " + createdAt`},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := spec.Anchors[0]
	if a.Kind != AnchorComposite {
		t.Errorf("want AnchorComposite, got %v", a.Kind)
	}
	if a.Name != "" {
		t.Errorf("want empty Name for multi-field composite, got %q", a.Name)
	}
}

func TestParseGrid_CompositeDuplicateFieldAllowed(t *testing.T) {
	// "status" appears both in the composite AND as a standalone cell — allowed:
	// the DSL no longer rejects a field appearing more than once.
	_, err := ParseGrid([][]string{
		{`status.visual + " " + status.label`, "status"},
	})
	if err != nil {
		t.Fatalf("duplicate field in composite + standalone should be allowed, got: %v", err)
	}
}

func TestParseGrid_AnchorOrderTopLeft(t *testing.T) {
	spec, err := ParseGrid([][]string{
		{"a", "b", "c"},
		{"d", "e", "f"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := spec.AnchorNames()
	want := []string{"a", "b", "c", "d", "e", "f"}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("[%d]: got %q, want %q", i, got[i], n)
		}
	}
}

func TestAnchorNamesColumnMajor(t *testing.T) {
	// Grid with a col-spanned title on row 0, a name.caption cell, and a `^`
	// row-span. Declaration (row-major) order interleaves columns; column-major
	// order must walk each column top-to-bottom before the next column.
	spec, err := ParseGrid([][]string{
		{"title", "--", "--"},
		{"status.caption", "status", "tags"},
		{"type", "priority", "^"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// AnchorNames stays declaration-order (row-major).
	gotDecl := spec.AnchorNames()
	wantDecl := []string{"title", "status", "status", "tags", "type", "priority"}
	if !slices.Equal(gotDecl, wantDecl) {
		t.Errorf("AnchorNames() = %v, want %v (declaration order must not change)", gotDecl, wantDecl)
	}

	// AnchorNamesColumnMajor sorts by (Col, Row): col 0 → title(0,0),
	// status.caption(1,0), type(2,0); col 1 → status(1,1), priority(2,1);
	// col 2 → tags(1,2).
	gotCM := spec.AnchorNamesColumnMajor()
	wantCM := []string{"title", "status", "type", "status", "priority", "tags"}
	if !slices.Equal(gotCM, wantCM) {
		t.Errorf("AnchorNamesColumnMajor() = %v, want %v", gotCM, wantCM)
	}
}

func TestAnchorDisplaysColumnMajor_AlignedWithNames(t *testing.T) {
	spec, err := ParseGrid([][]string{
		{"status.caption", "status"},
		{"type.caption", "type"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	names := spec.AnchorNamesColumnMajor()
	displays := spec.AnchorDisplaysColumnMajor()
	if len(names) != len(displays) {
		t.Fatalf("len mismatch: names=%d displays=%d", len(names), len(displays))
	}

	// Column-major: col 0 is the two .caption cells, col 1 is the two value
	// cells. The DisplayCaption mode must travel with its name's new position.
	wantNames := []string{"status", "type", "status", "type"}
	wantDisplays := []DisplayMode{DisplayCaption, DisplayCaption, DisplayLabel, DisplayLabel}
	if !slices.Equal(names, wantNames) {
		t.Fatalf("names = %v, want %v", names, wantNames)
	}
	for i := range wantDisplays {
		if displays[i] != wantDisplays[i] {
			t.Errorf("displays[%d] = %v, want %v (for name %q)", i, displays[i], wantDisplays[i], names[i])
		}
	}
}

func TestSplitRoleModifier_SplitsModifierSuffix(t *testing.T) {
	role, mod := theme.SplitRoleModifier("text.muted.accent")
	if role != "text.muted" || mod != "accent" {
		t.Errorf("got (%q, %q), want (%q, %q)", role, mod, "text.muted", "accent")
	}
}

func TestSplitRoleModifier_BareTokenHasNoModifier(t *testing.T) {
	role, mod := theme.SplitRoleModifier("text.muted")
	if role != "text.muted" || mod != "" {
		t.Errorf("got (%q, %q), want (%q, %q)", role, mod, "text.muted", "")
	}
}

func TestSplitRoleModifier_NoDotHasNoModifier(t *testing.T) {
	role, mod := theme.SplitRoleModifier("danger")
	if role != "danger" || mod != "" {
		t.Errorf("got (%q, %q), want (%q, %q)", role, mod, "danger", "")
	}
}

func TestTokenizeCell_RoleWithModifier(t *testing.T) {
	cell, err := TokenizeCell("<text.muted.accent>title")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	fc, ok := cell.(FieldCell)
	if !ok {
		t.Fatalf("want FieldCell, got %T", cell)
	}
	if fc.Role != "text.muted" || fc.Modifier != "accent" {
		t.Errorf("got role=%q modifier=%q, want role=%q modifier=%q", fc.Role, fc.Modifier, "text.muted", "accent")
	}
}

func TestParseSegment_RoleWithModifier(t *testing.T) {
	seg, err := parseSegment("<text.muted.accent>status")
	if err != nil {
		t.Fatalf("parseSegment: %v", err)
	}
	if seg.Role != "text.muted" || seg.Modifier != "accent" {
		t.Errorf("got role=%q modifier=%q, want role=%q modifier=%q", seg.Role, seg.Modifier, "text.muted", "accent")
	}
}

func TestTokenizeCell_RoleOnLiteral(t *testing.T) {
	cell, err := TokenizeCell(`<text.label>"Status:"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit, ok := cell.(LiteralCell)
	if !ok {
		t.Fatalf("expected LiteralCell, got %T", cell)
	}
	if lit.Text != "Status:" {
		t.Errorf("Text = %q, want %q", lit.Text, "Status:")
	}
	if lit.Role != "text.label" {
		t.Errorf("Role = %q, want %q", lit.Role, "text.label")
	}
	if lit.Modifier != "" {
		t.Errorf("Modifier = %q, want empty", lit.Modifier)
	}
}

func TestTokenizeCell_RoleWithModifierOnLiteral(t *testing.T) {
	cell, err := TokenizeCell(`<accent.lift>"Tags:"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit, ok := cell.(LiteralCell)
	if !ok {
		t.Fatalf("expected LiteralCell, got %T", cell)
	}
	if lit.Role != "accent" || lit.Modifier != "lift" {
		t.Errorf("Role/Modifier = %q/%q, want accent/lift", lit.Role, lit.Modifier)
	}
}

func TestTokenizeCell_PlainLiteralHasEmptyRole(t *testing.T) {
	cell, err := TokenizeCell(`"Status:"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit, ok := cell.(LiteralCell)
	if !ok {
		t.Fatalf("expected LiteralCell, got %T", cell)
	}
	if lit.Role != "" || lit.Modifier != "" {
		t.Errorf("expected empty Role/Modifier, got %q/%q", lit.Role, lit.Modifier)
	}
}

func TestTokenizeCell_RoleOnBareMarkerErrors(t *testing.T) {
	_, err := TokenizeCell(`<accent>--`)
	if err == nil {
		t.Fatalf("expected error for role on bare marker, got none")
	}
}

func TestTokenizeCell_AngleBracketInsideQuotedLiteralIsText(t *testing.T) {
	cell, err := TokenizeCell(`"<not a role>"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit, ok := cell.(LiteralCell)
	if !ok {
		t.Fatalf("expected LiteralCell, got %T", cell)
	}
	if lit.Text != "<not a role>" {
		t.Errorf("Text = %q, want %q", lit.Text, "<not a role>")
	}
	if lit.Role != "" {
		t.Errorf("Role = %q, want empty", lit.Role)
	}
}

func TestParseSegment_RoleOnLiteralSegment(t *testing.T) {
	seg, err := parseSegment(`<text.label>"Status: "`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seg.Kind != SegmentLiteral {
		t.Fatalf("Kind = %v, want SegmentLiteral", seg.Kind)
	}
	if seg.Text != "Status: " {
		t.Errorf("Text = %q, want %q", seg.Text, "Status: ")
	}
	if seg.Role != "text.label" {
		t.Errorf("Role = %q, want %q", seg.Role, "text.label")
	}
}

func TestParseSegment_RoleWithModifierOnLiteralSegment(t *testing.T) {
	seg, err := parseSegment(`<accent.lift>"!!"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seg.Kind != SegmentLiteral || seg.Role != "accent" || seg.Modifier != "lift" {
		t.Errorf("got Kind=%v Role=%q Modifier=%q, want SegmentLiteral/accent/lift",
			seg.Kind, seg.Role, seg.Modifier)
	}
}

func TestTokenizeCell_CompositeWithRoledLiterals(t *testing.T) {
	cell, err := TokenizeCell(`<text.label>"Status: " + status`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cc, ok := cell.(CompositeCell)
	if !ok {
		t.Fatalf("expected CompositeCell, got %T", cell)
	}
	if len(cc.Segments) != 2 {
		t.Fatalf("Segments len = %d, want 2", len(cc.Segments))
	}
	if cc.Segments[0].Kind != SegmentLiteral || cc.Segments[0].Role != "text.label" {
		t.Errorf("seg[0] = %+v, want literal with role text.label", cc.Segments[0])
	}
	if cc.Segments[1].Kind != SegmentField || cc.Segments[1].Name != "status" {
		t.Errorf("seg[1] = %+v, want field status", cc.Segments[1])
	}
}

func TestTokenizeCell_CaptionDisplayMode(t *testing.T) {
	cell, err := TokenizeCell("status.caption")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	fc, ok := cell.(FieldCell)
	if !ok {
		t.Fatalf("got %T, want FieldCell", cell)
	}
	if fc.Name != "status" {
		t.Errorf("Name = %q, want %q", fc.Name, "status")
	}
	if fc.Display != DisplayCaption {
		t.Errorf("Display = %v, want DisplayCaption", fc.Display)
	}
}

func TestTokenizeCell_CaptionWithRole(t *testing.T) {
	cell, err := TokenizeCell(`<text.label>status.caption`)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	fc, ok := cell.(FieldCell)
	if !ok || fc.Display != DisplayCaption || fc.Role != "text.label" {
		t.Fatalf("got %+v, want FieldCell{status, DisplayCaption, role=text.label}", cell)
	}
}

func TestTokenizeCell_Sizing(t *testing.T) {
	cases := []struct {
		in   string
		name string
		want Sizing
		hide bool
	}{
		{in: "status", name: "status", want: Sizing{Mode: SizeAuto}},
		{in: "tags:fr", name: "tags", want: Sizing{Mode: SizeGrow, Weight: 1}},
		{in: "title:2fr..40", name: "title", want: Sizing{Mode: SizeGrow, Weight: 2, Max: 40}},
		{in: "assignee:auto..20", name: "assignee", want: Sizing{Mode: SizeAuto, Max: 20}},
		{in: "id:6", name: "id", want: Sizing{Mode: SizeFixed, Min: 6, Max: 6}},
		{in: "dependsOn?", name: "dependsOn", want: Sizing{Mode: SizeAuto}, hide: true},
		{in: "tags?:fr", name: "tags", want: Sizing{Mode: SizeGrow, Weight: 1}, hide: true},
	}
	for _, c := range cases {
		cell, err := TokenizeCell(c.in)
		if err != nil {
			t.Errorf("TokenizeCell(%q): %v", c.in, err)
			continue
		}
		fc, ok := cell.(FieldCell)
		if !ok {
			t.Errorf("TokenizeCell(%q): not a FieldCell: %T", c.in, cell)
			continue
		}
		if fc.Name != c.name || fc.Sizing != c.want || fc.HideWhenEmpty != c.hide {
			t.Errorf("TokenizeCell(%q) = name %q sizing %+v hide %v; want %q %+v %v",
				c.in, fc.Name, fc.Sizing, fc.HideWhenEmpty, c.name, c.want, c.hide)
		}
	}
}

func TestTokenizeCell_StretcherTokenRetired(t *testing.T) {
	// "<->" is no longer a special marker; grow behavior comes from :fr.
	cell, err := TokenizeCell("col:fr")
	if err != nil {
		t.Fatalf("col:fr: %v", err)
	}
	fc, ok := cell.(FieldCell)
	if !ok {
		t.Fatalf("col:fr: not a FieldCell: %T", cell)
	}
	if fc.Sizing.Mode != SizeGrow {
		t.Errorf("col:fr mode = %v, want SizeGrow", fc.Sizing.Mode)
	}
}
