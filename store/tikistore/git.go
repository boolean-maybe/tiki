package tikistore

import (
	"io"

	"github.com/boolean-maybe/tiki/store/internal/git"
)

// GitInit initializes a new git repository at the given directory.
func GitInit(dir string) error {
	return git.Init(dir)
}

// GitClone clones a git repository from url into dir.
func GitClone(url, dir string, stdout, stderr io.Writer) error {
	return git.Clone(url, dir, stdout, stderr)
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
