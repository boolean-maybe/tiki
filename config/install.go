package config

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"time"
)

const (
	httpTimeout     = 15 * time.Second
	maxResponseSize = 1 << 20 // 1 MiB
)

var DefaultWorkflowBaseURL = "https://raw.githubusercontent.com/boolean-maybe/tiki/main"

var validWorkflowName = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)

// InstallResult describes the outcome for a single installed file.
type InstallResult struct {
	Path    string
	Changed bool
}

var installFiles = []string{
	defaultWorkflowFilename,
	templateFilename,
}

// InstallWorkflow fetches a named workflow from baseURL and writes its files
// to the directory for the given scope, overwriting existing files.
// baseURL is the root URL before "/workflows" (e.g. "https://raw.githubusercontent.com/boolean-maybe/tiki/main").
func InstallWorkflow(name string, scope Scope, baseURL string) ([]InstallResult, error) {
	if !validWorkflowName.MatchString(name) {
		return nil, fmt.Errorf("invalid workflow name %q: use letters, digits, hyphens, dots, or underscores", name)
	}

	dir, err := resolveDir(scope)
	if err != nil {
		return nil, err
	}

	fetched := make(map[string]string, len(installFiles))
	for _, filename := range installFiles {
		content, err := fetchWorkflowFile(baseURL, name, filename)
		if err != nil {
			return nil, fmt.Errorf("fetch %s/%s: %w", name, filename, err)
		}
		fetched[filename] = content
	}

	var results []InstallResult
	for _, filename := range installFiles {
		path := filepath.Join(dir, filename)
		changed, err := writeFileIfChanged(path, fetched[filename])
		if err != nil {
			return results, fmt.Errorf("write %s: %w", filename, err)
		}
		results = append(results, InstallResult{Path: path, Changed: changed})
	}

	return results, nil
}

var httpClient = &http.Client{Timeout: httpTimeout}

func fetchWorkflowFile(baseURL, name, filename string) (string, error) {
	url := fmt.Sprintf("%s/workflows/%s/%s", baseURL, name, filename)

	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("workflow %q not found (%s)", name, filename)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	return string(body), nil
}
