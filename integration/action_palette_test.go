package integration

import (
	"testing"

	"github.com/boolean-maybe/tiki/testutil"

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
