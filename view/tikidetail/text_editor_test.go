package tikidetail

import (
	"errors"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"

	"github.com/gdamore/tcell/v2"
)

type userListStore struct {
	store.Store
	users []string
	err   error
}

func (s userListStore) GetAllUsers() ([]string, error) {
	return s.users, s.err
}

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

func TestEditTextValue_AssigneeTextUsesPlainInput(t *testing.T) {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{Name: tikipkg.FieldAssignee, Type: workflow.TypeString},
	})
	t.Cleanup(teststatuses.Init)

	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TXT002")
	ctx := FieldRenderContext{FieldName: tikipkg.FieldAssignee, Roles: theme.Roles(), Store: s}
	w := buildFieldEditor(tikipkg.FieldAssignee, tk, ctx, func(string) {})
	if w == nil {
		t.Fatal("assignee editor is nil")
	}
	if _, ok := w.(*textInputEditAdapter); !ok {
		t.Fatalf("assignee text editor = %T, want *textInputEditAdapter", w)
	}
}

func TestEditUserValue_CustomUserUsesPicker(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "reviewer", Type: workflow.TypeUser},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("USR001")
	ctx := FieldRenderContext{
		FieldName: "reviewer",
		Roles:     theme.Roles(),
		Store:     userListStore{users: []string{"alice"}},
	}
	w := buildFieldEditor("reviewer", tk, ctx, func(string) {})
	picker, ok := w.(*selectListAdapter)
	if !ok {
		t.Fatalf("user editor = %T, want *selectListAdapter", w)
	}
	if !picker.AcceptsTextInput() {
		t.Fatal("user picker must allow free-form typing")
	}
}

func TestEditUserValue_CyclesKnownUsers(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "reviewer", Type: workflow.TypeUser},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("USR002")
	var changed string
	ctx := FieldRenderContext{
		FieldName: "reviewer",
		Roles:     theme.Roles(),
		Store:     userListStore{users: []string{"alice", "bob"}},
	}
	w := buildFieldEditor("reviewer", tk, ctx, func(v string) { changed = v })
	if !w.CycleValue(1) {
		t.Fatal("CycleValue returned false")
	}
	if got := w.GetText(); got != "alice" {
		t.Fatalf("cycled user = %q, want alice", got)
	}
	if changed != "alice" {
		t.Fatalf("onChange = %q, want alice", changed)
	}
}

func TestEditUserValue_AcceptsFreeTyping(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "reviewer", Type: workflow.TypeUser},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("USR003")
	var changed string
	ctx := FieldRenderContext{
		FieldName: "reviewer",
		Roles:     theme.Roles(),
		Store:     userListStore{users: []string{"alice"}},
	}
	w := buildFieldEditor("reviewer", tk, ctx, func(v string) { changed = v })
	handler := w.InputHandler()
	if handler == nil {
		t.Fatal("user editor has no input handler")
	}
	handler(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone), nil)
	if got := w.GetText(); got != "z" {
		t.Fatalf("typed user = %q, want z", got)
	}
	if changed != "z" {
		t.Fatalf("onChange = %q, want z", changed)
	}
}

func TestEditUserValue_ExistingUnknownSeedsValue(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "reviewer", Type: workflow.TypeUser},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("USR004")
	tk.Set("reviewer", "carol")
	ctx := FieldRenderContext{
		FieldName: "reviewer",
		Roles:     theme.Roles(),
		Store:     userListStore{users: nil},
	}
	w := buildFieldEditor("reviewer", tk, ctx, func(string) {})
	if got := w.GetText(); got != "carol" {
		t.Fatalf("existing unknown user seeded %q, want carol", got)
	}
}

func TestEditUserValue_StoreErrorStillAllowsTyping(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "reviewer", Type: workflow.TypeUser},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("USR005")
	ctx := FieldRenderContext{
		FieldName: "reviewer",
		Roles:     theme.Roles(),
		Store:     userListStore{err: errors.New("users unavailable")},
	}
	w := buildFieldEditor("reviewer", tk, ctx, func(string) {})
	picker, ok := w.(*selectListAdapter)
	if !ok {
		t.Fatalf("user editor = %T, want *selectListAdapter", w)
	}
	if !picker.AcceptsTextInput() {
		t.Fatal("user picker should still allow typing after user lookup failure")
	}
	if got := w.GetText(); got != "" {
		t.Fatalf("empty user editor seeded %q, want empty", got)
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
