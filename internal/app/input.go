package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
)

// InstallGlobalInputCapture installs the global keyboard handler
// (palette modal short-circuit, statusline auto-hide dismiss, router dispatch).
// F10 (toggle header) and ? (open palette) are both routed through InputRouter
// rather than handled here, so keyboard and palette-entered globals behave identically.
func InstallGlobalInputCapture(
	app *tview.Application,
	paletteConfig *model.ActionPaletteConfig,
	statuslineConfig *model.StatuslineConfig,
	inputRouter *controller.InputRouter,
	navController *controller.NavigationController,
) {
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// while the palette is visible, pass the event through unchanged so the
		// focused palette input field receives it. Do not dismiss statusline or
		// dispatch through InputRouter — the palette is modal.
		if paletteConfig != nil && paletteConfig.IsVisible() {
			return event
		}

		// dismiss auto-hide statusline messages on any keypress
		statuslineConfig.DismissAutoHide()

		if inputRouter.HandleInput(event, navController.CurrentView()) {
			return nil
		}
		return event
	})
}
