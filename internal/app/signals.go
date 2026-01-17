package app

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rivo/tview"
)

// SetupSignalHandler registers a signal handler that stops the application
// on SIGINT or SIGTERM.
func SetupSignalHandler(app *tview.Application) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		slog.Info("signal received, stopping app")
		app.Stop()
	}()
}
