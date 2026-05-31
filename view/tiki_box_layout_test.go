package view

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// makeTiki creates a tiki with the given ID, type string, and priority key.
// priority is an enum key; pass "" to omit the field.
func makeTiki(id string, tikiType string, priority string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.SetID(id)
	tk.Set(tikipkg.FieldType, tikiType)
	if priority != "" {
		tk.Set(tikipkg.FieldPriority, priority)
	}
	return tk
}

// testPluginLayout returns a minimal 3-row layout spec usable by tests
// that need a WorkflowPlugin.Layout to be populated.
func testPluginLayout(t *testing.T) gridlayout.GridSpec {
	t.Helper()
	spec, err := gridlayout.ParseGrid([][]string{
		{"id"},
		{"title"},
		{"priority"},
	})
	if err != nil {
		t.Fatalf("test layout parse: %v", err)
	}
	return spec
}

// TestCreateTikiBox_ReturnsFrame asserts the new layout-driven tiki box
// constructor produces a non-nil tview.Frame with the right number of
// content rows. The Frame wraps a gridbox.Container whose internal Flex
// holds one entry per layout row.
func TestCreateTikiBox_ReturnsFrame(t *testing.T) {
	tk := makeTiki("K3X9M2", "story", "medium")
	tk.SetTitle("Fix login retry logic")
	spec, err := gridlayout.ParseGrid([][]string{
		{"id"},
		{"<highlight>title"},
		{"priority"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	frame := CreateTikiBox(tk, spec, false, theme.Roles())
	if frame == nil {
		t.Fatal("CreateTikiBox returned nil")
	}
}

// TestTikiBoxItemHeight_DerivedFromLayout pins the height-derivation
// helper used by the plugin view. With TikiBoxOverhead = 2 (borders
// only — tiki cards have no inner padding rows), a 3-row layout
// becomes a 5-cell-tall card matching the pre-DSL compact rendering.
func TestTikiBoxItemHeight_DerivedFromLayout(t *testing.T) {
	spec := gridlayout.GridSpec{Rows: 3}
	got := tikiBoxItemHeight(spec)
	want := 3 + 2
	if got != want {
		t.Fatalf("tikiBoxItemHeight = %d, want %d", got, want)
	}
}

// TestTikiBoxItemHeight_SingleRowIsBorderless pins the special-case where
// a layout with exactly one row renders flat — no border, no overhead —
// so a list of single-row tikis stacks tightly with selection conveyed by
// background color rather than a frame.
func TestTikiBoxItemHeight_SingleRowIsBorderless(t *testing.T) {
	spec := gridlayout.GridSpec{Rows: 1}
	got := tikiBoxItemHeight(spec)
	if got != 1 {
		t.Fatalf("tikiBoxItemHeight(1-row) = %d, want 1", got)
	}
}

// TestCreateTikiBox_SingleRowRendersWithoutBorder pins that a one-row
// layout renders without box-drawing characters on either the top or
// bottom of the rendered output. The id must still appear on the only
// content row.
func TestCreateTikiBox_SingleRowRendersWithoutBorder(t *testing.T) {
	tk := makeTiki("K3X9M2", "story", "medium")
	tk.SetTitle("Fix login retry logic")
	spec, err := gridlayout.ParseGrid([][]string{
		{"id + \" \" + title"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rows := renderTikiBoxToString(t, tk, spec, 40)
	if len(rows) != 1 {
		t.Fatalf("rendered %d rows, want 1: %q", len(rows), rows)
	}
	if !strings.Contains(rows[0], "K3X9M2") {
		t.Errorf("row missing id: %q", rows[0])
	}
	for _, ch := range []string{"─", "│", "┌", "┐", "└", "┘"} {
		if strings.Contains(rows[0], ch) {
			t.Errorf("border character %q leaked into single-row tiki: %q", ch, rows[0])
		}
	}
}

// renderTikiBoxToString draws a CreateTikiBox into an offscreen tcell
// simulation screen and returns the visible characters, one row per
// line. The role-tag bytes are resolved by the simulation screen so the
// returned strings are user-visible content only.
func renderTikiBoxToString(t *testing.T, tk *tikipkg.Tiki, spec gridlayout.GridSpec, width int) []string {
	t.Helper()
	frame := CreateTikiBox(tk, spec, false, theme.Roles())
	height := tikiBoxItemHeight(spec)

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)
	frame.SetRect(0, 0, width, height)
	frame.Draw(screen)
	screen.Show()

	cells, w, _ := screen.GetContents()
	rows := make([]string, height)
	for y := 0; y < height; y++ {
		var row []rune
		for x := 0; x < w; x++ {
			r := cells[y*w+x].Runes
			if len(r) > 0 && r[0] != 0 {
				row = append(row, r[0])
			} else {
				row = append(row, ' ')
			}
		}
		rows[y] = strings.TrimRight(string(row), " ")
	}
	return rows
}

// TestCreateTikiBox_NarrowLaneStillRenders pins that the single-column
// stretcher safety net in gridbox.SolveGridLayout keeps the only column
// alive when the lane is narrower than the default field-width sum.
// Pre-fix, the column would be shed and the card would render as an
// empty bordered box.
func TestCreateTikiBox_NarrowLaneStillRenders(t *testing.T) {
	tk := makeTiki("K3X9M2", "story", "medium")
	tk.SetTitle("Fix login retry logic")
	spec, err := gridlayout.ParseGrid([][]string{
		{`type.visual + " " + id`},
		{"<highlight>title"},
		{`"priority " + priority.visual`},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	rows := renderTikiBoxToString(t, tk, spec, 22)
	// Top + 3 content rows + bottom = 5 rows. The id row should contain "K3X9M2".
	if len(rows) != 5 {
		t.Fatalf("rendered %d rows, want 5: %q", len(rows), rows)
	}
	if !strings.Contains(rows[1], "K3X9M2") {
		t.Errorf("row 1 missing id: %q (full output: %q)", rows[1], rows)
	}
	if !strings.Contains(rows[2], "Fix") {
		t.Errorf("row 2 missing title prefix 'Fix': %q (full output: %q)", rows[2], rows)
	}
}

// TestBuildTikiBoxPrimitives_TitleAnchorEscapesTviewTags pins that a
// title primitive's rendered text body contains the literal title
// characters and not any unresolved role-token bytes. tview.TextView
// holds the resolved text after SetText; we read it back with
// GetText(true) which strips tview style tags, leaving user-visible
// content only. This guards the property that role markup is composed
// before tview renders so the cell-width clip happens on visible
// characters, never mid-token.
func TestBuildTikiBoxPrimitives_TitleAnchorEscapesTviewTags(t *testing.T) {
	tk := makeTiki("K3X9M2", "story", "medium")
	tk.SetTitle(strings.Repeat("X", 80))
	spec, err := gridlayout.ParseGrid([][]string{
		{"<highlight>title"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	primitives := buildTikiBoxPrimitives(spec, tk, theme.Roles())
	if len(primitives) != 1 {
		t.Fatalf("expected 1 primitive, got %d", len(primitives))
	}
	tv, ok := primitives[0].(*tview.TextView)
	if !ok {
		t.Fatalf("title primitive type = %T, want *tview.TextView", primitives[0])
	}
	body := tv.GetText(true) // strip style tags
	if !strings.Contains(body, "X") {
		t.Errorf("title content missing: %q", body)
	}
	for _, badSubstr := range []string{"[-]", "[yellow]", "[red]", "[white]"} {
		if strings.Contains(body, badSubstr) {
			t.Errorf("raw role tag leaked into stripped output: %q in %q", badSubstr, body)
		}
	}
}

// TestBuildTikiBoxPrimitives_MissingPriorityRendersDash pins the
// empty-field fallback for composites referencing enum visuals: when
// the priority field is absent, the priority.visual segment must render
// as the dash placeholder rather than as an empty string or the literal
// token "priority.visual".
func TestBuildTikiBoxPrimitives_MissingPriorityRendersDash(t *testing.T) {
	tk := makeTiki("ZZZZZZ", "bug", "") // no priority
	tk.SetTitle("no priority")
	spec, err := gridlayout.ParseGrid([][]string{
		{`"priority " + priority.visual`},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	primitives := buildTikiBoxPrimitives(spec, tk, theme.Roles())
	if len(primitives) != 1 {
		t.Fatalf("expected 1 primitive, got %d", len(primitives))
	}
	tv, ok := primitives[0].(*tview.TextView)
	if !ok {
		t.Fatalf("composite primitive type = %T, want *tview.TextView", primitives[0])
	}
	body := tv.GetText(true) // strip style tags
	if !strings.Contains(body, "priority") {
		t.Errorf("priority label missing from primitive: %q", body)
	}
	if !strings.Contains(body, "—") {
		t.Errorf("expected dash placeholder for missing priority, got: %q", body)
	}
}
