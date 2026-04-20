package integration

import (
	"testing"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

func TestActionPalette_OpenAndClose(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()
	ta.Draw()

	// Ctrl+A opens the palette
	ta.SendKey(tcell.KeyCtrlA, 0, tcell.ModCtrl)
	if !ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should be visible after pressing Ctrl+A")
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
	ta.SendKey(tcell.KeyCtrlA, 0, tcell.ModCtrl)
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

func TestActionPalette_AsteriskIsFilterTextInPalette(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()
	ta.Draw()

	// open palette
	ta.SendKey(tcell.KeyCtrlA, 0, tcell.ModCtrl)
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

func TestActionPalette_AsteriskDoesNotOpenPalette(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()
	ta.Draw()

	// send '*' as a rune on the plugin view
	ta.SendKey(tcell.KeyRune, '*', tcell.ModNone)

	if ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should NOT open when '*' is pressed — only Ctrl+A should open it")
	}
}

func TestActionPalette_OpensInTaskEdit(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Test", taskpkg.StatusReady, taskpkg.TypeStory); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	ta.NavController.PushView(model.TaskEditViewID, model.EncodeTaskEditParams(model.TaskEditParams{
		TaskID: "TIKI-1",
		Focus:  model.EditFieldTitle,
	}))
	ta.Draw()

	// Ctrl+A should open the palette even in task edit
	ta.SendKey(tcell.KeyCtrlA, 0, tcell.ModCtrl)

	if !ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should open when Ctrl+A is pressed in task edit view")
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func TestActionPalette_OpensWithInputBoxFocused(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// open search to focus input box
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)

	v := ta.NavController.GetActiveView()
	iv, ok := v.(controller.InputableView)
	if !ok || !iv.IsInputBoxFocused() {
		t.Fatal("input box should be focused after '/'")
	}

	// Ctrl+A should open the palette even with input box focused
	ta.SendKey(tcell.KeyCtrlA, 0, tcell.ModCtrl)

	if !ta.GetPaletteConfig().IsVisible() {
		t.Fatal("palette should open when Ctrl+A is pressed with input box focused")
	}

	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
}
