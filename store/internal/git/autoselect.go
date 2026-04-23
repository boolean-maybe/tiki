package git

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/gogit"
	"github.com/boolean-maybe/tiki/store/internal/git/internal/shell"
)

type backendKind int

const (
	backendShell backendKind = iota
	backendGoGit
)

var (
	backendOnce   sync.Once
	chosenBackend backendKind
)

func probeBackendKind() backendKind {
	backendOnce.Do(func() {
		if _, err := exec.LookPath("git"); err == nil {
			chosenBackend = backendShell
		} else {
			chosenBackend = backendGoGit
		}
	})
	return chosenBackend
}

// selector is a lazy GitOps implementation that defers backend
// construction until the first git-related method call.
type selector struct {
	repoPath string
	once     sync.Once
	backend  GitOps
	initErr  error
}

func newSelector(repoPath string) *selector {
	return &selector{repoPath: repoPath}
}

func (s *selector) ensureBackend() error {
	s.once.Do(func() {
		kind := probeBackendKind()
		switch kind {
		case backendShell:
			s.backend, s.initErr = shell.NewUtil(s.repoPath)
		case backendGoGit:
			s.backend, s.initErr = gogit.NewUtil(s.repoPath)
		default:
			s.initErr = fmt.Errorf("unknown backend kind: %d", kind)
		}
	})
	return s.initErr
}

func (s *selector) Add(paths ...string) error {
	if err := s.ensureBackend(); err != nil {
		return err
	}
	return s.backend.Add(paths...)
}

func (s *selector) Remove(paths ...string) error {
	if err := s.ensureBackend(); err != nil {
		return err
	}
	return s.backend.Remove(paths...)
}

func (s *selector) CurrentUser() (string, string, error) {
	if err := s.ensureBackend(); err != nil {
		return "", "", err
	}
	return s.backend.CurrentUser()
}

func (s *selector) Author(filePath string) (*AuthorInfo, error) {
	if err := s.ensureBackend(); err != nil {
		return nil, err
	}
	return s.backend.Author(filePath)
}

func (s *selector) AllAuthors(dirPattern string) (map[string]*AuthorInfo, error) {
	if err := s.ensureBackend(); err != nil {
		return nil, err
	}
	return s.backend.AllAuthors(dirPattern)
}

func (s *selector) LastCommitTime(filePath string) (time.Time, error) {
	if err := s.ensureBackend(); err != nil {
		return time.Time{}, err
	}
	return s.backend.LastCommitTime(filePath)
}

func (s *selector) AllLastCommitTimes(dirPattern string) (map[string]time.Time, error) {
	if err := s.ensureBackend(); err != nil {
		return nil, err
	}
	return s.backend.AllLastCommitTimes(dirPattern)
}

func (s *selector) CurrentBranch() (string, error) {
	if err := s.ensureBackend(); err != nil {
		return "", err
	}
	return s.backend.CurrentBranch()
}

func (s *selector) FileVersionsSince(filePath string, since time.Time, includePrior bool) ([]FileVersion, error) {
	if err := s.ensureBackend(); err != nil {
		return nil, err
	}
	return s.backend.FileVersionsSince(filePath, since, includePrior)
}

func (s *selector) AllFileVersionsSince(dirPattern string, since time.Time, includePrior bool) (map[string][]FileVersion, error) {
	if err := s.ensureBackend(); err != nil {
		return nil, err
	}
	return s.backend.AllFileVersionsSince(dirPattern, since, includePrior)
}

func (s *selector) AllUsers() ([]string, error) {
	if err := s.ensureBackend(); err != nil {
		return nil, err
	}
	return s.backend.AllUsers()
}
