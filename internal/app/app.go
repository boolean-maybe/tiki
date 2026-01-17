package app

import (
	"log/slog"
	"os"

	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/view"
)

// NewApp creates a tview application.
func NewApp() *tview.Application {
	return tview.NewApplication()
}

// Run runs the tview application or terminates the process if it errors.
func Run(app *tview.Application, rootLayout *view.RootLayout) {
	app.SetRoot(rootLayout.GetPrimitive(), true).EnableMouse(false)
	if err := app.Run(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}
