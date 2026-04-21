package git

import (
	"os"
	"os/exec"

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

func isRepoShell(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	return cmd.Run() == nil
}

func isRepoGoGit(path string) bool {
	_, err := gogitlib.PlainOpenWithOptions(path, &gogitlib.PlainOpenOptions{DetectDotGit: true})
	return err == nil
}
