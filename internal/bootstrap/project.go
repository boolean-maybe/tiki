package bootstrap

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
)

// EnsureProjectInitialized ensures the project is properly initialized.
// It takes the embedded skill content for tiki and doki.
// Returns (proceed, error) where proceed indicates if the user wants to continue.
func EnsureProjectInitialized(tikiSkillContent, dokiSkillContent string) (bool, error) {
	proceed, err := config.EnsureProjectInitialized(tikiSkillContent, dokiSkillContent)
	if err != nil {
		return false, fmt.Errorf("initialize project: %w", err)
	}
	return proceed, nil
}
