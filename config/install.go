package config

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"time"
)

const (
	httpTimeout     = 15 * time.Second
	maxResponseSize = 1 << 20 // 1 MiB
)

var validWorkflowName = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)

// ValidWorkflowName reports whether name is a valid workflow name.
func ValidWorkflowName(name string) bool {
	return validWorkflowName.MatchString(name)
}

// InstallResult describes the outcome for a single installed file.
type InstallResult struct {
	Path    string
	Changed bool
}

// InstallWorkflow fetches workflow content from the given source and writes it
// to the workflow.yaml in the directory for the given scope.
// Callers are responsible for validating content before calling this function
// (use ValidateWorkflowContent + validateWorkflowTriggers for non-embedded sources).
func InstallWorkflow(src WorkflowSource, scope Scope) ([]InstallResult, error) {
	content, err := FetchWorkflowContent(src)
	if err != nil {
		return nil, err
	}
	return InstallWorkflowFromContent(content, scope)
}

// InstallWorkflowFromContent writes pre-fetched (and already validated) workflow
// content to the workflow.yaml in the directory for the given scope.
func InstallWorkflowFromContent(content string, scope Scope) ([]InstallResult, error) {
	dir, err := resolveDir(scope)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, defaultWorkflowFilename)
	changed, err := writeFileIfChanged(path, content)
	if err != nil {
		return nil, fmt.Errorf("write %s: %w", defaultWorkflowFilename, err)
	}

	return []InstallResult{{Path: path, Changed: changed}}, nil
}

// DescribeWorkflow fetches the workflow content from the given source and
// returns the value of its top-level description field.
func DescribeWorkflow(src WorkflowSource) (string, error) {
	content, err := FetchWorkflowContent(src)
	if err != nil {
		return "", err
	}
	return DescribeWorkflowContent(content)
}

var httpClient = &http.Client{Timeout: httpTimeout}
