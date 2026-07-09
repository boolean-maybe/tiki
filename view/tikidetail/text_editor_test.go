package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"

	"github.com/gdamore/tcell/v2"
)

func TestEditTextValue_PlainInputForCatalogText(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "note", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("TXT001")
	tk.Set("note", "hello")
	ctx := FieldRenderContext{FieldName: "note", Roles: theme.Roles()}
	w := editTextValue(tk, ctx, func(string) {})
	if w == nil {
		t.Fatal("editTextValue returned nil for catalog text field")
	}
	if _, ok := w.(*textInputEditAdapter); !ok {
		t.Fatalf("catalog text editor = %T, want *textInputEditAdapter", w)
	}
	if w.GetText() != "hello" {
		t.Fatalf("initial text = %q want %q", w.GetText(), "hello")
	}
}

func TestEditTextValue_AssigneeUsesSuggestions(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TXT002")
	ctx := FieldRenderContext{FieldName: tikipkg.FieldAssignee, Roles: theme.Roles(), Store: s}
	w := buildFieldEditor(tikipkg.FieldAssignee, tk, ctx, func(string) {})
	if w == nil {
		t.Fatal("assignee editor is nil")
	}
	// assignee's descriptor.Suggestions != nil drives a select-list; assert the
	// widget is the picker adapter (selectListAdapter) not the plain input.
	if _, ok := w.(*selectListAdapter); !ok {
		t.Fatalf("assignee editor = %T, want *selectListAdapter", w)
	}
}

// TestEditTextValue_ExtremesNoClip pins the measure↔render contract for a plain
// text field at its narrowest (1 char) and widest (60 char) stored values, in
// edit mode: the focused input, drawn at the width the solver reserves for the
// field anchor (MeasureAnchor — which includes the focus-marker reserve), must
// render the whole seed unclipped. Mirrors the tags-editor extremes assertion,
// but through MeasureAnchor since that is the width the solver actually grants.
func TestEditTextValue_ExtremesNoClip(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "note", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	cases := []string{"x", strings.Repeat("a", 60)}
	for _, seed := range cases {
		tk := tikipkg.New()
		tk.SetID("TXTEX1")
		tk.Set("note", seed)
		ctx := FieldRenderContext{Mode: RenderModeEdit, Roles: theme.Roles(), FieldName: "note"}

		anchor := gridlayout.Anchor{Kind: gridlayout.AnchorField, Name: "note", RowSpan: 1, ColSpan: 1}
		colWidth := MeasureAnchor(anchor, tk, ctx)
		if colWidth < len(seed) {
			t.Fatalf("seed %q: measured width %d < seed length %d", seed, colWidth, len(seed))
		}

		cv := &ConfigurableDetailView{
			editMode:          true,
			editors:           map[string]FieldEditorWidget{},
			onEditFieldChange: map[string]func(string){},
		}
		editor := cv.ensureEditor("note", tk, ctx)

		screen := tcell.NewSimulationScreen("")
		if err := screen.Init(); err != nil {
			t.Fatal(err)
		}
		screen.SetSize(colWidth+2, 3)
		editor.SetRect(0, 0, colWidth, 3)
		editor.Draw(screen)
		screen.Show()
		row0 := readSimRow(screen, 0)
		if !strings.Contains(row0, seed) {
			t.Errorf("text editor at anchor width %d clipped seed %q: row0=%q", colWidth, seed, row0)
		}
	}
}
