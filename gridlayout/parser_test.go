package gridlayout

import (
	"strings"
	"testing"

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
		{in: "|", check: func(t *testing.T, c Cell) {
			if _, ok := c.(RowSpanCell); !ok {
				t.Errorf("want RowSpanCell, got %T", c)
			}
		}},
		{in: "_", check: func(t *testing.T, c Cell) {
			if _, ok := c.(EmptyCell); !ok {
				t.Errorf("want EmptyCell, got %T", c)
			}
		}},
		{in: "<->", check: func(t *testing.T, c Cell) {
			if _, ok := c.(StretcherCell); !ok {
				t.Errorf("want StretcherCell, got %T", c)
			}
		}},
		{in: "status", check: func(t *testing.T, c Cell) {
			fc, ok := c.(FieldCell)
			if !ok || fc.Name != "status" || fc.WantedWidth != 0 {
				t.Errorf("want FieldCell{status,0}, got %+v", c)
			}
		}},
		{in: "tags:30", check: func(t *testing.T, c Cell) {
			fc, ok := c.(FieldCell)
			if !ok || fc.Name != "tags" || fc.WantedWidth != 30 {
				t.Errorf("want FieldCell{tags,30}, got %+v", c)
			}
		}},
		{in: "createdAt", check: func(t *testing.T, c Cell) {
			fc, ok := c.(FieldCell)
			if !ok || fc.Name != "createdAt" {
				t.Errorf("want FieldCell{createdAt}, got %+v", c)
			}
		}},
		{in: "tags:0", wantErr: true},
		// `^` and `|` both tokenize to RowSpanCell (`^` is the bare-legal
		// preferred form; `|` is accepted for backward-compat but requires
		// YAML quoting).
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
			if !ok || fc.Name != "title" || fc.Role != "highlight" || fc.WantedWidth != 0 {
				t.Errorf("want FieldCell{title, highlight, 0}, got %+v", c)
			}
		}},
		{in: "<accent>status:20", check: func(t *testing.T, c Cell) {
			fc, ok := c.(FieldCell)
			if !ok || fc.Name != "status" || fc.Role != "accent" || fc.WantedWidth != 20 {
				t.Errorf("want FieldCell{status, accent, 20}, got %+v", c)
			}
		}},
		{in: "<highlight>Status:", check: func(t *testing.T, c Cell) {
			lc, ok := c.(LiteralCell)
			if !ok || lc.Text != "<highlight>Status:" {
				t.Errorf("want LiteralCell{<highlight>Status:}, got %+v", c)
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
	// `|` is a synonym for `^` and produces the same diagnostic.
	_, err = ParseGrid([][]string{{"|"}, {"status"}})
	if err == nil || !strings.Contains(err.Error(), "orphan row-span") {
		t.Errorf("want orphan row-span error for '|', got %v", err)
	}
}

func TestParseGrid_StretcherMix(t *testing.T) {
	_, err := ParseGrid([][]string{
		{"status", "<->"},
		{"type", "tags"},
	})
	if err == nil || !strings.Contains(err.Error(), "<->") {
		t.Errorf("want stretcher-mix error, got %v", err)
	}
}

func TestParseGrid_DuplicateField(t *testing.T) {
	_, err := ParseGrid([][]string{
		{"status"},
		{"status"},
	})
	if err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Errorf("want duplicate-field error, got %v", err)
	}
}

func TestParseGrid_CanonicalExample(t *testing.T) {
	spec, err := ParseGrid([][]string{
		{"title", "--", "--", "--", "--"},
		{"status", "assignee", "<->", "tags:30", "depends:25"},
		{"type", "createdBy", "<->", "|", "|"},
		{"priority", "createdAt", "<->", "_", "_"},
		{"points", "updatedAt", "<->", "_", "_"},
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if spec.Rows != 5 || spec.Cols != 5 {
		t.Errorf("dimensions: got rows=%d cols=%d, want 5x5", spec.Rows, spec.Cols)
	}
	want := []struct {
		Name       string
		Row, Col   int
		RS, CS, WW int
	}{
		{"title", 0, 0, 1, 5, 0},
		{"status", 1, 0, 1, 1, 0},
		{"assignee", 1, 1, 1, 1, 0},
		{"tags", 1, 3, 2, 1, 30},
		{"depends", 1, 4, 2, 1, 25},
		{"type", 2, 0, 1, 1, 0},
		{"createdBy", 2, 1, 1, 1, 0},
		{"priority", 3, 0, 1, 1, 0},
		{"createdAt", 3, 1, 1, 1, 0},
		{"points", 4, 0, 1, 1, 0},
		{"updatedAt", 4, 1, 1, 1, 0},
	}
	if len(spec.Anchors) != len(want) {
		t.Fatalf("anchor count: got %d, want %d (anchors=%+v)", len(spec.Anchors), len(want), spec.Anchors)
	}
	for i, w := range want {
		a := spec.Anchors[i]
		if a.Name != w.Name || a.Row != w.Row || a.Col != w.Col || a.RowSpan != w.RS || a.ColSpan != w.CS || a.WantedWidth != w.WW {
			t.Errorf("anchor[%d]: got %+v, want name=%s row=%d col=%d rs=%d cs=%d ww=%d", i, a, w.Name, w.Row, w.Col, w.RS, w.CS, w.WW)
		}
	}
	// Stretcher column is col 2.
	wantStretch := []bool{false, false, true, false, false}
	if len(spec.Stretcher) != 5 {
		t.Fatalf("stretcher len: got %d, want 5", len(spec.Stretcher))
	}
	for c, s := range wantStretch {
		if spec.Stretcher[c] != s {
			t.Errorf("stretcher[%d]: got %v, want %v", c, spec.Stretcher[c], s)
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

// TestYAML_BareMarkers verifies the unquoted-marker contract: --, _, ^, <->
// are bare-legal in YAML flow context, while bare `|` is rejected (it is
// YAML's block-scalar indicator). This test pins both halves of the
// contract end-to-end: YAML → tokenizer.
func TestYAML_BareMarkers(t *testing.T) {
	// Round-trip a grid with all four bare markers in valid positions:
	//   row 0: title spans columns 0-1 via `--`; status is a separate anchor; <-> marks a stretcher column
	//   row 1: title continues into col 0-1 via `^`; status takes col 2; col 3 is `_` (empty)
	src := `m:
  - [title, --, status, <->]
  - [^,     ^,  _,      <->]
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
		{FieldCell{Name: "title"}, ColSpanCell{}, FieldCell{Name: "status"}, StretcherCell{}},
		{RowSpanCell{}, RowSpanCell{}, EmptyCell{}, StretcherCell{}},
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
		wantLit  bool // expect LiteralCell fallback (all-literal composite)
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
			in:      `"A" + " " + "B"`,
			wantLit: true, // all literals → falls through to LiteralCell
		},
		{
			in:       `status:15 + " " + status.visual`,
			segments: 3,
			check: func(t *testing.T, c CompositeCell) {
				if c.Segments[0].WantedWidth != 15 {
					t.Errorf("seg[0] width: got %d, want 15", c.Segments[0].WantedWidth)
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
			if tc.wantLit {
				if _, ok := got.(LiteralCell); !ok {
					t.Fatalf("want LiteralCell for all-literal composite, got %T", got)
				}
				return
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

func TestParseGrid_CompositeDuplicateFieldRejected(t *testing.T) {
	// "status" appears both in the composite AND as a standalone cell
	_, err := ParseGrid([][]string{
		{`status.visual + " " + status.label`, "status"},
	})
	if err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Errorf("want duplicate-field error, got %v", err)
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
