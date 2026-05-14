package gridlayout

import (
	"strings"
	"testing"
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
		{in: "tags:-5", wantErr: true},
		{in: "tags:", wantErr: true},
		{in: "1tag", wantErr: true},
		{in: "foo bar", wantErr: true},
		{in: "**", wantErr: true},
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
	_, err := ParseGrid([][]string{{"|"}, {"status"}})
	if err == nil || !strings.Contains(err.Error(), "orphan '|'") {
		t.Errorf("want orphan row-span error, got %v", err)
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
