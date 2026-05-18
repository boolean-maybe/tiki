package bootstrap

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
)

// EnsureProjectInitialized ensures the project is properly initialized.
// It takes the embedded tiki skill content.
// gitAdd, when non-nil, is called to stage created files.
// Returns (proceed, error) where proceed indicates if the user wants to continue.
func EnsureProjectInitialized(tikiSkillContent string, gitAdd func(...string) error) (bool, error) {
	proceed, err := config.EnsureProjectInitialized(tikiSkillContent, gitAdd)
	if err != nil {
		return false, fmt.Errorf("initialize project: %w", err)
	}
	return proceed, nil
}
