package bootstrap

import (
	"log/slog"
	"os"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// InitStores initializes the task stores or terminates the process on failure.
// Returns the tikiStore and a generic store interface reference to it.
func InitStores() (*tikistore.TikiStore, store.Store) {
	tikiStore, err := tikistore.NewTikiStore(config.TaskDir)
	if err != nil {
		slog.Error("failed to initialize task store", "error", err)
		os.Exit(1)
	}
	return tikiStore, tikiStore
}
