package git

import (
	"io"
	"os"
	"os/exec"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/gogit"
	"github.com/boolean-maybe/tiki/store/internal/git/internal/shell"
	gogitlib "github.com/go-git/go-git/v5"
)

// IsRepo checks if the given path is a git repository.
// If path is empty, the current working directory is used.
// Uses the process-wide backend selection rule: if shell git is available,
// probes with `git rev-parse`; otherwise uses go-git's PlainOpen.
func IsRepo(path string) bool {
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return false
		}
	}

	kind := probeBackendKind()
	switch kind {
	case backendShell:
		return isRepoShell(path)
	default:
		return isRepoGoGit(path)
	}
}

// Init initializes a new git repository at the given directory.
func Init(dir string) error {
	switch probeBackendKind() {
	case backendShell:
		return shell.Init(dir)
	default:
		return gogit.Init(dir)
	}
}

// Clone clones a git repository from url into dir.
func Clone(url, dir string, stdout, stderr io.Writer) error {
	switch probeBackendKind() {
	case backendShell:
		return shell.Clone(url, dir, stdout, stderr)
	default:
		return gogit.Clone(url, dir, stdout, stderr)
	}
}

func isRepoShell(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	return cmd.Run() == nil
}

func isRepoGoGit(path string) bool {
	_, err := gogitlib.PlainOpenWithOptions(path, &gogitlib.PlainOpenOptions{DetectDotGit: true})
	return err == nil
}
