package integration

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"
	"github.com/boolean-maybe/tiki/view/taskdetail"

	"github.com/gdamore/tcell/v2"
)

func TestActionPalette_OpenAndClose(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()
	ta.Draw()

	// * opens the palette
	ta.SendKey(tcell.KeyRune, '*', tcell.ModNone)
	if !ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should be visible after pressing '*'")
	}

	// Esc closes it
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
	if ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should be hidden after pressing Esc")
	}
}

func TestActionPalette_F10TogglesHeader(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()
	ta.Draw()

	hc := ta.GetHeaderConfig()
	initialVisible := hc.IsVisible()

	// F10 should toggle header via the router
	ta.SendKey(tcell.KeyF10, 0, tcell.ModNone)
	if hc.IsVisible() == initialVisible {
		t.Fatal("F10 should toggle header visibility")
	}

	// toggle back
	ta.SendKey(tcell.KeyF10, 0, tcell.ModNone)
	if hc.IsVisible() != initialVisible {
		t.Fatal("second F10 should restore header visibility")
	}
}

func TestActionPalette_ModalBlocksGlobals(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()
	ta.Draw()

	hc := ta.GetHeaderConfig()
	startVisible := hc.IsVisible()

	// open palette
	ta.SendKey(tcell.KeyRune, '*', tcell.ModNone)
	if !ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should be open")
	}

	// F10 while palette is open should NOT toggle header
	// (app capture returns event unchanged, palette input handler swallows F10)
	ta.SendKey(tcell.KeyF10, 0, tcell.ModNone)
	if hc.IsVisible() != startVisible {
		t.Fatal("F10 should be blocked while palette is modal")
	}

	// close palette
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func TestActionPalette_AsteriskFiltersInPalette(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()
	ta.Draw()

	// open palette
	ta.SendKey(tcell.KeyRune, '*', tcell.ModNone)
	if !ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should be open")
	}

	// typing '*' while palette is open should be treated as filter text, not open another palette
	ta.SendKeyToFocused(tcell.KeyRune, '*', tcell.ModNone)

	// palette should still be open
	if !ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should remain open when '*' is typed as filter")
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

// getEditView type-asserts the active view to *taskdetail.TaskEditView.
func getEditView(t *testing.T, ta *testutil.TestApp) *taskdetail.TaskEditView {
	t.Helper()
	v := ta.NavController.GetActiveView()
	ev, ok := v.(*taskdetail.TaskEditView)
	if !ok {
		t.Fatalf("expected *taskdetail.TaskEditView, got %T", v)
	}
	return ev
}

func TestActionPalette_BlockedInTaskEdit_DraftFirstKey(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// press 'n' to create new task (draft path, title focused)
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// very first key in edit view is '*'
	ta.SendKey(tcell.KeyRune, '*', tcell.ModNone)

	if ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should NOT open when '*' is pressed in task edit (draft, first key)")
	}
	ev := getEditView(t, ta)
	if got := ev.GetEditedTitle(); got != "*" {
		t.Fatalf("title = %q, want %q", got, "*")
	}
}

func TestActionPalette_BlockedInTaskEdit_ExistingFirstKey(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Test", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// push task edit directly (non-draft path)
	ta.NavController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
		TaskID: "TIKI-1",
		Focus:  model.EditFieldTitle,
	}))
	ta.Draw()

	// first key is '*'
	ta.SendKey(tcell.KeyRune, '*', tcell.ModNone)

	if ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should NOT open when '*' is pressed in task edit (existing, first key)")
	}
	ev := getEditView(t, ta)
	if got := ev.GetEditedTitle(); got != "Test*" {
		t.Fatalf("title = %q, want %q", got, "Test*")
	}
}

func TestActionPalette_BlockedInTaskEdit_TitleMidText(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)
	ta.SendText("ab*cd")

	if ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should NOT open when '*' is typed mid-title")
	}
	ev := getEditView(t, ta)
	if got := ev.GetEditedTitle(); got != "ab*cd" {
		t.Fatalf("title = %q, want %q", got, "ab*cd")
	}
}

func TestActionPalette_BlockedInTaskEdit_Description(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Test", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	ta.NavController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
		TaskID:   "TIKI-1",
		Focus:    model.EditFieldDescription,
		DescOnly: true,
	}))
	ta.Draw()

	ta.SendText("x*y")

	if ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should NOT open when '*' is typed in description")
	}
	ev := getEditView(t, ta)
	desc := ev.GetEditedDescription()
	if !strings.Contains(desc, "x*y") {
		t.Fatalf("description = %q, should contain %q", desc, "x*y")
	}
}

func TestActionPalette_BlockedInTaskEdit_Tags(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Test", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	ta.NavController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
		TaskID:   "TIKI-1",
		TagsOnly: true,
	}))
	ta.Draw()

	ta.SendText("tag*val")

	if ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should NOT open when '*' is typed in tags")
	}
	ev := getEditView(t, ta)
	tags := ev.GetEditedTags()
	if len(tags) != 1 || tags[0] != "tag*val" {
		t.Fatalf("tags = %v, want [tag*val]", tags)
	}
}

func TestActionPalette_BlockedInTaskEdit_Assignee(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Test", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// push task edit (default title focus)
	ta.NavController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
		TaskID: "TIKI-1",
		Focus:  model.EditFieldTitle,
	}))
	ta.Draw()

	// tab 5× to Assignee: Title→Status→Type→Priority→Points→Assignee
	for i := 0; i < 5; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	ta.SendKey(tcell.KeyRune, '*', tcell.ModNone)

	if ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should NOT open when '*' is pressed on Assignee field")
	}
	editTask := ta.EditingTask()
	if editTask == nil {
		t.Fatal("expected editing task to be non-nil")
	}
	if editTask.Assignee != "Unassigned*" {
		t.Fatalf("assignee = %q, want %q ('*' appended to default value)", editTask.Assignee, "Unassigned*")
	}
}

func TestActionPalette_BlockedInTaskEdit_NonTypeableField(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Test", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// push task edit (default title focus)
	ta.NavController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
		TaskID: "TIKI-1",
		Focus:  model.EditFieldTitle,
	}))
	ta.Draw()

	// tab 1× to Status (non-typeable field)
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	ta.SendKey(tcell.KeyRune, '*', tcell.ModNone)

	if ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should NOT open when '*' is pressed on Status field")
	}
	editTask := ta.EditingTask()
	if editTask == nil {
		t.Fatal("expected editing task to be non-nil")
	}
	if editTask.Status != taskpkg.StatusReady {
		t.Fatalf("status = %v, want %v (rune should be silently ignored)", editTask.Status, taskpkg.StatusReady)
	}
}
