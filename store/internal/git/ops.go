package git

import "time"

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

// NewGitOps creates a new GitOps instance. Backend selection (shell vs go-git)
// is deferred until the first method call, based on whether shell git is available.
func NewGitOps(repoPath string) (GitOps, error) {
	return newSelector(repoPath), nil
}
