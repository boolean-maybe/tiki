package shell

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/types"
)

// CurrentUser returns the current git user's name and email
func (u *Util) CurrentUser() (name string, email string, err error) {
	u.currentUserMu.Lock()
	if u.currentUserCached {
		name = u.currentUserName
		email = u.currentUserEmail
		err = u.currentUserErr
		u.currentUserMu.Unlock()
		return name, email, err
	}
	u.currentUserMu.Unlock()

	nameCmd := exec.Command("git", "config", "user.name")
	nameCmd.Dir = u.repoPath
	if nameBytes, err := nameCmd.Output(); err == nil {
		name = strings.TrimSpace(string(nameBytes))
	}

	emailCmd := exec.Command("git", "config", "user.email")
	emailCmd.Dir = u.repoPath
	if emailBytes, err := emailCmd.Output(); err == nil {
		email = strings.TrimSpace(string(emailBytes))
	}

	if name == "" {
		nameCmd := exec.Command("git", "config", "--global", "user.name")
		if nameBytes, err := nameCmd.Output(); err == nil {
			name = strings.TrimSpace(string(nameBytes))
		}
	}

	if email == "" {
		emailCmd := exec.Command("git", "config", "--global", "user.email")
		if emailBytes, err := emailCmd.Output(); err == nil {
			email = strings.TrimSpace(string(emailBytes))
		}
	}

	if name == "" && email == "" {
		err = errors.New("git user not configured (user.name and user.email are empty)")
		u.currentUserMu.Lock()
		u.currentUserName = name
		u.currentUserEmail = email
		u.currentUserErr = err
		u.currentUserCached = true
		u.currentUserMu.Unlock()
		return "", "", err
	}

	u.currentUserMu.Lock()
	u.currentUserName = name
	u.currentUserEmail = email
	u.currentUserErr = nil
	u.currentUserCached = true
	u.currentUserMu.Unlock()
	return name, email, nil
}

// Author returns information about who created a file
func (u *Util) Author(filePath string) (*types.AuthorInfo, error) {
	relPath := filePath
	if filepath.IsAbs(filePath) {
		var err error
		relPath, err = filepath.Rel(u.repoPath, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to convert path %s to relative: %w", filePath, err)
		}
	}

	//nolint:gosec // G204: git command with controlled file path
	cmd := exec.Command("git", "log", "--diff-filter=A", "--format=%H|%an|%ae|%ai|%s", "--reverse", "--", relPath)
	cmd.Dir = u.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", relPath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, fmt.Errorf("no commits found for file %s", relPath)
	}

	parts := strings.SplitN(lines[0], "|", 5)
	if len(parts) < 5 {
		return nil, fmt.Errorf("unexpected git log format for %s", relPath)
	}

	hash := parts[0]
	authorName := parts[1]
	authorEmail := parts[2]
	dateStr := parts[3]
	message := parts[4]

	date, err := parseGitTime(dateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse date %s: %w", dateStr, err)
	}

	return &types.AuthorInfo{
		Name:       authorName,
		Email:      authorEmail,
		Date:       date,
		CommitHash: hash,
		Message:    message,
	}, nil
}

// AllAuthors returns author information for all files matching dirPattern in a single git command.
func (u *Util) AllAuthors(dirPattern string) (map[string]*types.AuthorInfo, error) {
	cmd := exec.Command("git", "log", "--all", "--diff-filter=A", "--format=%H|%an|%ae|%ai|%s", "--name-only", "--reverse", "--", dirPattern) //nolint:gosec // G204: git command with controlled directory pattern
	cmd.Dir = u.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", dirPattern, err)
	}

	result := make(map[string]*types.AuthorInfo)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	var currentHash, currentAuthor, currentEmail, currentDate, currentMessage string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "|") {
			parts := strings.SplitN(line, "|", 5)
			if len(parts) < 5 {
				continue
			}
			currentHash = parts[0]
			currentAuthor = parts[1]
			currentEmail = parts[2]
			currentDate = parts[3]
			currentMessage = parts[4]
		} else {
			date, err := parseGitTime(currentDate)
			if err != nil {
				continue
			}

			if _, exists := result[line]; !exists {
				result[line] = &types.AuthorInfo{
					Name:       currentAuthor,
					Email:      currentEmail,
					Date:       date,
					CommitHash: currentHash,
					Message:    currentMessage,
				}
			}
		}
	}

	return result, nil
}

// AllUsers returns a deduplicated list of all users who have made commits in the repository.
func (u *Util) AllUsers() ([]string, error) {
	if u.cachedUsers != nil {
		return u.cachedUsers, nil
	}

	cmd := exec.Command("git", "log", "--all", "--format=%an")
	cmd.Dir = u.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for all users: %w", err)
	}

	seen := make(map[string]bool)
	var users []string

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name != "" && !seen[name] {
			seen[name] = true
			users = append(users, name)
		}
	}

	u.cachedUsers = users

	return users, nil
}
