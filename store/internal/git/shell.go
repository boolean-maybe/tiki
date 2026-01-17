package git

import "github.com/boolean-maybe/tiki/store/internal/git/shell"

// GitShellUtil is an alias to shell.Util for backward compatibility
type GitShellUtil = shell.Util

// NewGitShellUtil creates a new shell-based Git utility
func NewGitShellUtil(repoPath string) (*GitShellUtil, error) {
	return shell.NewUtil(repoPath)
}
