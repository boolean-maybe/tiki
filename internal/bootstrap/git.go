package bootstrap

import (
	"fmt"
	"os"

	"github.com/boolean-maybe/tiki/store/tikistore"
)

// EnsureGitRepoOrExit validates that the current directory is a git repository.
// If not, it prints an error message and exits the program.
func EnsureGitRepoOrExit() {
	if tikistore.IsGitRepo("") {
		return
	}
	_, err := fmt.Fprintln(os.Stderr, "Not a git repository")
	if err != nil {
		return
	}
	os.Exit(1)
}
