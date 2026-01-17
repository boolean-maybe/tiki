package bootstrap

import (
	"log/slog"
	"os"

	"github.com/boolean-maybe/tiki/config"
)

// EnsureProjectInitialized ensures the project is properly initialized.
// It takes the embedded skill content for tiki and doki and returns whether to proceed.
// If initialization fails, it logs an error and exits.
func EnsureProjectInitialized(tikiSkillContent, dokiSkillContent string) (proceed bool) {
	proceed, err := config.EnsureProjectInitialized(tikiSkillContent, dokiSkillContent)
	if err != nil {
		slog.Error("failed to initialize project", "error", err)
		os.Exit(1)
	}
	return proceed
}
