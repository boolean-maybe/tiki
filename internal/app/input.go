package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/model"
)

// InstallGlobalInputCapture installs the global keyboard handler
// (header toggle, router dispatch).
func InstallGlobalInputCapture(
	app *tview.Application,
	headerConfig *model.HeaderConfig,
	inputRouter *controller.InputRouter,
	navController *controller.NavigationController,
) {
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyF10 {
			headerConfig.ToggleUserPreference()
			return nil
		}
		if inputRouter.HandleInput(event, navController.CurrentView()) {
			return nil
		}
		return event
	})
}
