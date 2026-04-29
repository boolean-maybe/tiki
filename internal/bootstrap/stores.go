package bootstrap

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// InitStores initializes the task stores.
// Returns the tikiStore, a generic store interface, and any error.
// Validates store.name here (in addition to Bootstrap) because runExec and
// pipe paths call InitStores directly without going through Bootstrap.
func InitStores() (*tikistore.TikiStore, store.Store, error) {
	if name := config.GetStoreName(); name != "tiki" {
		return nil, nil, fmt.Errorf("unknown store backend: %q (supported: tiki)", name)
	}
	// Phase 2: the store scans the unified document root recursively, so we
	// pass .doc/ here instead of .doc/tiki/. Any existing `.doc/tiki/*.md`
	// files continue to load (they are picked up by the recursive walk);
	// new documents are written at `.doc/<ID>.md` directly under the root.
	tikiStore, err := tikistore.NewTikiStore(config.GetDocDir())
	if err != nil {
		return nil, nil, fmt.Errorf("initialize task store: %w", err)
	}
	return tikiStore, tikiStore, nil
}
