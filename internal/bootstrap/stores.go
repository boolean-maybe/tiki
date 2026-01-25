package bootstrap

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// InitStores initializes the task stores.
// Returns the tikiStore, a generic store interface, and any error.
func InitStores() (*tikistore.TikiStore, store.Store, error) {
	tikiStore, err := tikistore.NewTikiStore(config.GetTaskDir())
	if err != nil {
		return nil, nil, fmt.Errorf("initialize task store: %w", err)
	}
	return tikiStore, tikiStore, nil
}
