package shell

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// FileVersion represents the content of a file at a specific commit
type FileVersion struct {
	Hash    string
	Author  string
	Email   string
	When    time.Time
	Content string
}

// FileVersionsSince returns file contents for commits since the provided time.
// if includePrior is true, the most recent commit before the given time is included as the first version.
func (u *Util) FileVersionsSince(filePath string, since time.Time, includePrior bool) ([]FileVersion, error) {
	relPath, err := u.toRelative(filePath)
	if err != nil {
		return nil, err
	}

	sinceStr := since.Format(time.RFC3339)
	var versions []FileVersion

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

func (u *Util) buildFileVersion(logLine string, relPath string) (FileVersion, error) {
	parts := strings.SplitN(strings.TrimSpace(logLine), "|", 4)
	if len(parts) < 4 {
		return FileVersion{}, fmt.Errorf("unexpected git log format for %s", relPath)
	}

	when, err := parseGitTime(parts[3])
	if err != nil {
		return FileVersion{}, err
	}

	showTarget := fmt.Sprintf("%s:%s", parts[0], relPath)
	//nolint:gosec // G204: git command with controlled commit hash and file path
	showCmd := exec.Command("git", "show", showTarget)
	showCmd.Dir = u.repoPath
	content, err := showCmd.Output()
	if err != nil {
		return FileVersion{}, fmt.Errorf("failed to read %s at %s: %w", relPath, parts[0], err)
	}

	return FileVersion{
		Hash:    parts[0],
		Author:  parts[1],
		Email:   parts[2],
		When:    when,
		Content: string(content),
	}, nil
}

// AllFileVersionsSince returns file versions for all files matching dirPattern since the given time.
// Returns a map of relative file paths to their version history.
// If includePrior is true, includes the most recent commit before the time window for each file.
func (u *Util) AllFileVersionsSince(dirPattern string, since time.Time, includePrior bool) (map[string][]FileVersion, error) {
	sinceStr := since.Format(time.RFC3339)
	result := make(map[string][]FileVersion)

	// Step 1: Get all commits that changed the status field in files matching the pattern
	//nolint:gosec // G204: git command with controlled directory pattern and timestamp
	cmd := exec.Command("git", "log", "--all", "--full-history", "-G^status:",
		"--format=%H|%an|%ae|%aI", "--name-only", "--since", sinceStr, "--", dirPattern)
	cmd.Dir = u.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log for %s: %w", dirPattern, err)
	}

	// Step 2: Parse commits and group files by commit hash
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
			// This is a commit header line
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
			// This is a file name
			currentCommit.files = append(currentCommit.files, line)
			// Update the last commit in slice
			commits[len(commits)-1] = *currentCommit
		}
	}

	// Step 3: Build a map of file -> list of (commit, when)
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

	// Step 4: If includePrior, get the most recent commit before the time window for each file
	// Build a map of file -> most recent prior commit (only status changes)
	if includePrior {
		//nolint:gosec // G204: git command with controlled directory pattern and timestamp
		cmd := exec.Command("git", "log", "--all", "--full-history", "-G^status:",
			"--format=%H|%an|%ae|%aI", "--name-only", "--before", sinceStr, "--", dirPattern)
		cmd.Dir = u.repoPath
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			var currentCommit *commitInfo
			priorCommits := make(map[string]fileCommit) // file -> most recent commit before window

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
					// This is a file name
					file := line
					if _, exists := fileCommits[file]; exists {
						// Only track files we care about, and only keep the first (most recent) prior commit
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
			}

			// Prepend prior commits to their respective files
			for file, priorCommit := range priorCommits {
				fileCommits[file] = append([]fileCommit{priorCommit}, fileCommits[file]...)
			}
		}
	}

	// Step 5: Fetch file contents in parallel using worker pool
	type blobRequest struct {
		file   string
		commit fileCommit
	}

	type blobResult struct {
		file    string
		version FileVersion
		err     error
	}

	// Build list of all blobs to fetch
	var requests []blobRequest
	for file, commits := range fileCommits {
		for _, commit := range commits {
			requests = append(requests, blobRequest{file: file, commit: commit})
		}
	}

	// Parallel fetch with worker pool (limit concurrency to avoid overwhelming git)
	const workerCount = 10
	resultChan := make(chan blobResult, len(requests))
	requestChan := make(chan blobRequest, len(requests))

	// Send all requests
	for _, req := range requests {
		requestChan <- req
	}
	close(requestChan)

	// Start workers
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
					version: FileVersion{
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

	// Wait for all workers
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for res := range resultChan {
		if res.err != nil {
			continue
		}
		result[res.file] = append(result[res.file], res.version)
	}

	return result, nil
}
