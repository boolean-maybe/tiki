package git

import (
	"time"

	"github.com/boolean-maybe/tiki/store/internal/git/shell"
)

// AuthorInfo contains information about who created a file
type AuthorInfo = shell.AuthorInfo

// FileVersion represents the content of a file at a specific commit
type FileVersion = shell.FileVersion

// GitOps defines the interface for git operations
type GitOps interface {
	Add(paths ...string) error
	Remove(paths ...string) error
	CurrentUser() (name string, email string, err error)
	Author(filePath string) (*AuthorInfo, error)
	AllAuthors(dirPattern string) (map[string]*AuthorInfo, error)
	LastCommitTime(filePath string) (time.Time, error)
	AllLastCommitTimes(dirPattern string) (map[string]time.Time, error)
	CurrentBranch() (string, error)
	FileVersionsSince(filePath string, since time.Time, includePrior bool) ([]FileVersion, error)
	AllFileVersionsSince(dirPattern string, since time.Time, includePrior bool) (map[string][]FileVersion, error)
	AllUsers() ([]string, error)
}

// NewGitOps creates a new GitOps instance using the shell-out implementation by default
func NewGitOps(repoPath string) (GitOps, error) {
	return NewGitShellUtil(repoPath)
}
