package background

import (
	"context"
	"log/slog"

	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// StartBurndownHistoryBuilder starts a background job to build burndown history
// and publish results into HeaderConfig.
func StartBurndownHistoryBuilder(
	ctx context.Context,
	tikiStore *tikistore.TikiStore,
	headerConfig *model.HeaderConfig,
	app *tview.Application,
) {
	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		gitUtil := tikiStore.GetGitOps()
		if gitUtil == nil {
			slog.Warn("skipping burndown: git not available")
			return
		}

		// Phase 2: burndown walks the unified document root so it sees both
		// legacy `.doc/tiki/*.md` files and new `.doc/<ID>.md` files in one
		// pass.
		history := store.NewTaskHistory(config.GetDocDir(), gitUtil)
		if history == nil {
			return
		}

		slog.Info("building burndown history in background")
		if err := history.Build(); err != nil {
			slog.Warn("failed to build task history", "error", err)
			return
		}

		slog.Info("burndown history built successfully")
		tikiStore.SetTaskHistory(history)

		select {
		case <-ctx.Done():
			return
		default:
		}

		app.QueueUpdateDraw(func() {
			headerConfig.SetBurndown(history.Burndown())
		})
	}()
}
