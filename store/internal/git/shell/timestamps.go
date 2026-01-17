package shell

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// LastCommitTime returns the timestamp of the most recent commit that modified the file
func (u *Util) LastCommitTime(filePath string) (time.Time, error) {
	relPath, err := u.toRelative(filePath)
	if err != nil {
		return time.Time{}, err
	}

	// Get most recent commit that modified this file
	//nolint:gosec // G204: git command with controlled file path
	cmd := exec.Command("git", "log", "-1", "--format=%aI", "--", relPath)
	cmd.Dir = u.repoPath
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last commit for %s: %w", relPath, err)
	}

	dateStr := strings.TrimSpace(string(output))
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("no commits found for file %s", relPath)
	}

	return parseGitTime(dateStr)
}

// AllLastCommitTimes returns last commit timestamp for all files matching dirPattern in a single git command.
// Returns a map of file paths to their last commit time.
func (u *Util) AllLastCommitTimes(dirPattern string) (map[string]time.Time, error) {
	// Get all commits (most recent first due to default reverse chronological order)
	cmd := exec.Command("git", "log", "--all", "--format=%aI", "--name-only", "--", dirPattern)
	cmd.Dir = u.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", dirPattern, err)
	}

	result := make(map[string]time.Time)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	var currentDate time.Time

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to parse as a date (commit timestamp line)
		if date, err := parseGitTime(line); err == nil {
			currentDate = date
		} else {
			// This is a file name
			// Only store the first occurrence (most recent commit due to reverse chronological order)
			if _, exists := result[line]; !exists {
				result[line] = currentDate
			}
		}
	}

	return result, nil
}
