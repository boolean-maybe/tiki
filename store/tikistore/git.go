package tikistore

import (
	"github.com/boolean-maybe/tiki/store/internal/git"
)

// GitInit initializes a new git repository at the given directory.
func GitInit(dir string) error {
	return git.Init(dir)
}

// NewGitAdder returns a function that stages files using the git abstraction.
// The returned function is suitable for passing to config.BootstrapSystem.
func NewGitAdder(repoPath string) func(...string) error {
	ops, err := git.NewGitOps(repoPath)
	if err != nil {
		return nil
	}
	return ops.Add
}
