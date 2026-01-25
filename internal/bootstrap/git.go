package bootstrap

import (
	"errors"

	"github.com/boolean-maybe/tiki/store/tikistore"
)

// ErrNotGitRepo indicates the current directory is not a git repository
var ErrNotGitRepo = errors.New("not a git repository")

// EnsureGitRepo validates that the current directory is a git repository.
// Returns ErrNotGitRepo if the current directory is not a git repository.
func EnsureGitRepo() error {
	if tikistore.IsGitRepo("") {
		return nil
	}
	return ErrNotGitRepo
}
