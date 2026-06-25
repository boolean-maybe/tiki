package bootstrap

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// InitStores initializes the tiki stores.
// Returns the tikiStore, a generic store interface, and any error.
// Validates store.name here (in addition to Bootstrap) because runExec and
// pipe paths call InitStores directly without going through Bootstrap.
func InitStores() (*tikistore.TikiStore, store.Store, error) {
	if name := config.GetStoreName(); name != "tiki" {
		return nil, nil, fmt.Errorf("unknown store backend: %q (supported: tiki)", name)
	}
	// The store scans the document root (cwd) recursively. Hidden subdirs are
	// pruned except `.doc`, which the walker traverses by exception — so legacy
	// projects whose tikis still live under `.doc/` (e.g. `.doc/tiki/*.md`) load,
	// while new documents are written as `<slug>.md` directly under the root.
	tikiStore, err := tikistore.NewTikiStore(config.GetDocDir())
	if err != nil {
		return nil, nil, fmt.Errorf("initialize tiki store: %w", err)
	}
	return tikiStore, tikiStore, nil
}
