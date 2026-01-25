package app

import (
	"fmt"

	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/view"
)

// NewApp creates a tview application.
func NewApp() *tview.Application {
	return tview.NewApplication()
}

// Run runs the tview application.
// Returns an error if the application fails to run.
func Run(app *tview.Application, rootLayout *view.RootLayout) error {
	app.SetRoot(rootLayout.GetPrimitive(), true).EnableMouse(false)
	if err := app.Run(); err != nil {
		return fmt.Errorf("run application: %w", err)
	}
	return nil
}
