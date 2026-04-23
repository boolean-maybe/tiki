package shell

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/types"
)

// FileVersionsSince returns file contents for commits since the provided time.
// if includePrior is true, the most recent commit before the given time is included as the first version.
func (u *Util) FileVersionsSince(filePath string, since time.Time, includePrior bool) ([]types.FileVersion, error) {
	relPath, err := u.toRelative(filePath)
	if err != nil {
		return nil, err
	}

	sinceStr := since.Format(time.RFC3339)
	var versions []types.FileVersion

	if includePrior {
		//nolint:gosec // G204: git command with controlled file path and timestamp
		cmd := exec.Command("git", "log", "-1", "--format=%H|%an|%ae|%aI", "--before", sinceStr, "--", relPath)
		cmd.Dir = u.repoPath
		if output, err := cmd.Output(); err == nil {
			line := strings.TrimSpace(string(output))
			if line != "" {
				version, err := u.buildFileVersion(line, relPath)
				if err != nil {
					return nil, err
				}
				versions = append(versions, version)
			}
		}
	}

	//nolint:gosec // G204: git command with controlled file path and timestamp
	cmd := exec.Command("git", "log", "--format=%H|%an|%ae|%aI", "--since", sinceStr, "--reverse", "--", relPath)
	cmd.Dir = u.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", relPath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		version, err := u.buildFileVersion(line, relPath)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, nil
}

func (u *Util) buildFileVersion(logLine string, relPath string) (types.FileVersion, error) {
	parts := strings.SplitN(strings.TrimSpace(logLine), "|", 4)
	if len(parts) < 4 {
		return types.FileVersion{}, fmt.Errorf("unexpected git log format for %s", relPath)
	}

	when, err := parseGitTime(parts[3])
	if err != nil {
		return types.FileVersion{}, err
	}

	showTarget := fmt.Sprintf("%s:%s", parts[0], relPath)
	//nolint:gosec // G204: git command with controlled commit hash and file path
	showCmd := exec.Command("git", "show", showTarget)
	showCmd.Dir = u.repoPath
	content, err := showCmd.Output()
	if err != nil {
		return types.FileVersion{}, fmt.Errorf("failed to read %s at %s: %w", relPath, parts[0], err)
	}

	return types.FileVersion{
		Hash:    parts[0],
		Author:  parts[1],
		Email:   parts[2],
		When:    when,
		Content: string(content),
	}, nil
}

// AllFileVersionsSince returns file versions for all files matching dirPattern since the given time.
func (u *Util) AllFileVersionsSince(dirPattern string, since time.Time, includePrior bool) (map[string][]types.FileVersion, error) {
	sinceStr := since.Format(time.RFC3339)
	result := make(map[string][]types.FileVersion)

	//nolint:gosec // G204: git command with controlled directory pattern and timestamp
	cmd := exec.Command("git", "log", "--all", "--full-history", "-G^status:",
		"--format=%H|%an|%ae|%aI", "--name-only", "--since", sinceStr, "--", dirPattern)
	cmd.Dir = u.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", dirPattern, err)
	}

	type commitInfo struct {
		hash   string
		author string
		email  string
		when   time.Time
		files  []string
	}

	var commits []commitInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var currentCommit *commitInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "|") {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) < 4 {
				continue
			}
			when, err := parseGitTime(parts[3])
			if err != nil {
				continue
			}
			currentCommit = &commitInfo{
				hash:   parts[0],
				author: parts[1],
				email:  parts[2],
				when:   when,
				files:  []string{},
			}
			commits = append(commits, *currentCommit)
		} else if currentCommit != nil {
			currentCommit.files = append(currentCommit.files, line)
			commits[len(commits)-1] = *currentCommit
		}
	}

	type fileCommit struct {
		hash   string
		author string
		email  string
		when   time.Time
	}
	fileCommits := make(map[string][]fileCommit)

	for _, commit := range commits {
		for _, file := range commit.files {
			fileCommits[file] = append(fileCommits[file], fileCommit{
				hash:   commit.hash,
				author: commit.author,
				email:  commit.email,
				when:   commit.when,
			})
		}
	}

	if includePrior {
		//nolint:gosec // G204: git command with controlled directory pattern and timestamp
		cmd := exec.Command("git", "log", "--all", "--full-history", "-G^status:",
			"--format=%H|%an|%ae|%aI", "--name-only", "--before", sinceStr, "--", dirPattern)
		cmd.Dir = u.repoPath
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			var currentCommit *commitInfo
			priorCommits := make(map[string]fileCommit)

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				if strings.Contains(line, "|") {
					parts := strings.SplitN(line, "|", 4)
					if len(parts) < 4 {
						continue
					}
					when, err := parseGitTime(parts[3])
					if err != nil {
						continue
					}
					currentCommit = &commitInfo{
						hash:   parts[0],
						author: parts[1],
						email:  parts[2],
						when:   when,
						files:  []string{},
					}
				} else if currentCommit != nil {
					file := line
					if _, alreadyHave := priorCommits[file]; !alreadyHave {
						priorCommits[file] = fileCommit{
							hash:   currentCommit.hash,
							author: currentCommit.author,
							email:  currentCommit.email,
							when:   currentCommit.when,
						}
					}
				}
			}

			for file, priorCommit := range priorCommits {
				fileCommits[file] = append([]fileCommit{priorCommit}, fileCommits[file]...)
			}
		}
	}

	type blobRequest struct {
		file   string
		commit fileCommit
	}

	type blobResult struct {
		file    string
		version types.FileVersion
		err     error
	}

	var requests []blobRequest
	for file, commits := range fileCommits {
		for _, commit := range commits {
			requests = append(requests, blobRequest{file: file, commit: commit})
		}
	}

	const workerCount = 10
	resultChan := make(chan blobResult, len(requests))
	requestChan := make(chan blobRequest, len(requests))

	for _, req := range requests {
		requestChan <- req
	}
	close(requestChan)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range requestChan {
				showTarget := fmt.Sprintf("%s:%s", req.commit.hash, req.file)
				//nolint:gosec // G204: git command with controlled commit hash and file path
				showCmd := exec.Command("git", "show", showTarget)
				showCmd.Dir = u.repoPath
				content, err := showCmd.Output()

				if err != nil {
					resultChan <- blobResult{file: req.file, err: err}
					continue
				}

				resultChan <- blobResult{
					file: req.file,
					version: types.FileVersion{
						Hash:    req.commit.hash,
						Author:  req.commit.author,
						Email:   req.commit.email,
						When:    req.commit.when,
						Content: string(content),
					},
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for res := range resultChan {
		if res.err != nil {
			continue
		}
		result[res.file] = append(result[res.file], res.version)
	}

	return result, nil
}
