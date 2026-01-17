package shell

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Add stages files to the git index (git add)
func (u *Util) Add(paths ...string) error {
	if len(paths) == 0 {
		return errors.New("no paths provided")
	}

	relPaths := make([]string, len(paths))
	for i, path := range paths {
		relPath := path
		if filepath.IsAbs(path) {
			var err error
			relPath, err = filepath.Rel(u.repoPath, path)
			if err != nil {
				return fmt.Errorf("failed to convert path %s to relative: %w", path, err)
			}
		}
		relPaths[i] = relPath
	}

	args := append([]string{"add"}, relPaths...)
	//nolint:gosec // G204: git command with controlled file paths
	cmd := exec.Command("git", args...)
	cmd.Dir = u.repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to git add: %w", err)
	}

	return nil
}

// Remove removes files from the git index and working directory (git rm)
func (u *Util) Remove(paths ...string) error {
	if len(paths) == 0 {
		return errors.New("no paths provided")
	}

	relPaths := make([]string, len(paths))
	for i, path := range paths {
		relPath := path
		if filepath.IsAbs(path) {
			var err error
			relPath, err = filepath.Rel(u.repoPath, path)
			if err != nil {
				return fmt.Errorf("failed to convert path %s to relative: %w", path, err)
			}
		}
		relPaths[i] = relPath
	}

	args := append([]string{"rm"}, relPaths...)
	//nolint:gosec // G204: git command with controlled file paths
	cmd := exec.Command("git", args...)
	cmd.Dir = u.repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to git rm: %w", err)
	}

	return nil
}
