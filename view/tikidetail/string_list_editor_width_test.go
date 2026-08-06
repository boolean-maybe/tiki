package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
	"github.com/gdamore/tcell/v2"
)

// TestStringListEditor_SeedRendersUnwrappedAtMeasuredWidth pins the measure↔render
// contract for a focused string-list editor. The grid solver sizes the value
// column to MeasureFieldValue (the longest token, no padding — it measures the
// read-only WordList footprint). The focused editor swaps in a *tview.TextArea
// for the same column, so the editor's usable inner width must equal the
// measured content width; otherwise the seed value wraps mid-word even though
// the solver reserved exactly enough room for it.
//
// regression: the TextArea carried SetBorderPadding(0,0,1,1), consuming two
// columns the measure never accounted for. A four-cell seed rendered as two
// stacked rows the moment focus landed on the field.
func TestStringListEditor_SeedRendersUnwrappedAtMeasuredWidth(t *testing.T) {
	const fieldName = "labels"
	const seed = "idea"
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: fieldName, Type: workflow.TypeListString},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("LIST01")
	tk.SetTitle("seeded project")
	tk.Set(fieldName, []string{seed})

	ctx := FieldRenderContext{Mode: RenderModeEdit, Roles: theme.Roles(), FieldName: fieldName}

	// width the solver grants the value column: its measured content.
	colWidth := MeasureFieldValue(fieldName, tk, ctx)
	if colWidth < len(seed) {
		t.Fatalf("measured list width %d < seed length %d; measure regressed", colWidth, len(seed))
	}

	cv := &ConfigurableDetailView{
		editMode:          true,
		editors:           map[string]FieldEditorWidget{},
		onEditFieldChange: map[string]func(string){},
	}
	editor := cv.ensureEditor(fieldName, tk, ctx)

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
		t.Errorf("list editor at measured width %d wrapped the seed: row0=%q, want it to contain %q whole", colWidth, row0, seed)
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
