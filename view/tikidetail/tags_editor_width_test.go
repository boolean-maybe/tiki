package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/gdamore/tcell/v2"
)

// TestTagsEditor_SeedRendersUnwrappedAtMeasuredWidth pins the measure↔render
// contract for the focused tags editor. The grid solver sizes the tags value
// column to MeasureFieldValue (the longest token, no padding — it measures the
// read-only WordList footprint). The focused editor swaps in a *tview.TextArea
// for the same column, so the editor's usable inner width must equal the
// measured content width; otherwise the seed value wraps mid-word even though
// the solver reserved exactly enough room for it.
//
// Regression: the tags TextArea carried SetBorderPadding(0,0,1,1), consuming
// two columns the measure never accounted for. A new project seeded with the
// template tag "idea" rendered "id"/"ea" stacked the moment focus landed on
// tags — the column was 4 cells, the padded editor's inner width was 2.
func TestTagsEditor_SeedRendersUnwrappedAtMeasuredWidth(t *testing.T) {
	const seed = "idea"

	tk := tikipkg.New()
	tk.SetID("TAGED1")
	tk.SetTitle("seeded project")
	tk.Set(tikipkg.FieldTags, []string{seed})

	ctx := FieldRenderContext{Mode: RenderModeEdit, Roles: theme.Roles(), FieldName: tikipkg.FieldTags}

	// width the solver grants the tags value column: its measured content.
	colWidth := MeasureFieldValue(tikipkg.FieldTags, tk, ctx)
	if colWidth < len(seed) {
		t.Fatalf("measured tags width %d < seed length %d; measure regressed", colWidth, len(seed))
	}

	cv := &ConfigurableDetailView{
		editMode:          true,
		editors:           map[string]FieldEditorWidget{},
		onEditFieldChange: map[string]func(string){},
	}
	editor := cv.ensureEditor(tikipkg.FieldTags, tk, ctx)

	const height = 4
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(colWidth, height)
	editor.SetRect(0, 0, colWidth, height)
	editor.Draw(screen)
	screen.Show()

	row0 := readSimRow(screen, 0)
	if !strings.Contains(row0, seed) {
		t.Errorf("tags editor at measured width %d wrapped the seed: row0=%q, want it to contain %q whole", colWidth, row0, seed)
	}
}

// readSimRow returns the printable runes of one row of a simulation screen.
func readSimRow(screen tcell.SimulationScreen, row int) string {
	cells, w, _ := screen.GetContents()
	out := make([]rune, 0, w)
	for col := 0; col < w; col++ {
		r := cells[row*w+col].Runes
		if len(r) > 0 && r[0] != 0 {
			out = append(out, r[0])
			continue
		}
		out = append(out, ' ')
	}
	return strings.TrimRight(string(out), " ")
}
