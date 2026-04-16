package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ResetScope identifies which config tier to reset.
type ResetScope string

// ResetTarget identifies which config file to reset.
type ResetTarget string

const (
	ScopeGlobal  ResetScope = "global"
	ScopeLocal   ResetScope = "local"
	ScopeCurrent ResetScope = "current"

	TargetAll      ResetTarget = ""
	TargetConfig   ResetTarget = "config"
	TargetWorkflow ResetTarget = "workflow"
	TargetNew      ResetTarget = "new"
)

// resetEntry pairs a filename with the default content to restore for global scope.
// If defaultContent is empty, the file is always deleted (no embedded default exists).
type resetEntry struct {
	filename       string
	defaultContent string
}

var resetEntries = []resetEntry{
	// TODO: embed a default config.yaml once one exists; until then, global reset deletes the file
	{filename: configFilename, defaultContent: ""},
	{filename: defaultWorkflowFilename, defaultContent: defaultWorkflowYAML},
	{filename: templateFilename, defaultContent: defaultNewTaskTemplate},
}

// ResetConfig resets configuration files for the given scope and target.
// Returns the list of file paths that were actually modified or deleted.
func ResetConfig(scope ResetScope, target ResetTarget) ([]string, error) {
	dir, err := resolveDir(scope)
	if err != nil {
		return nil, err
	}

	entries, err := filterEntries(target)
	if err != nil {
		return nil, err
	}

	var affected []string
	for _, e := range entries {
		path := filepath.Join(dir, e.filename)
		changed, err := resetFile(path, scope, e.defaultContent)
		if err != nil {
			return affected, fmt.Errorf("reset %s: %w", e.filename, err)
		}
		if changed {
			affected = append(affected, path)
		}
	}

	return affected, nil
}

// resolveDir returns the directory path for the given scope.
func resolveDir(scope ResetScope) (string, error) {
	switch scope {
	case ScopeGlobal:
		return GetConfigDir(), nil
	case ScopeLocal:
		if !IsProjectInitialized() {
			return "", fmt.Errorf("not in an initialized tiki project (run 'tiki init' first)")
		}
		return GetProjectConfigDir(), nil
	case ScopeCurrent:
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		return cwd, nil
	default:
		return "", fmt.Errorf("unknown scope: %s", scope)
	}
}

// filterEntries returns the reset entries matching the target.
func filterEntries(target ResetTarget) ([]resetEntry, error) {
	if target == TargetAll {
		return resetEntries, nil
	}
	var filename string
	switch target {
	case TargetConfig:
		filename = configFilename
	case TargetWorkflow:
		filename = defaultWorkflowFilename
	case TargetNew:
		filename = templateFilename
	default:
		return nil, fmt.Errorf("unknown reset target: %q", target)
	}
	for _, e := range resetEntries {
		if e.filename == filename {
			return []resetEntry{e}, nil
		}
	}
	return nil, fmt.Errorf("no reset entry for target %q", target)
}

// resetFile either overwrites or deletes a file depending on scope and available defaults.
// For global scope with non-empty default content, the file is overwritten.
// Otherwise the file is deleted. Returns true if the file was changed.
func resetFile(path string, scope ResetScope, defaultContent string) (bool, error) {
	if scope == ScopeGlobal && defaultContent != "" {
		return writeDefault(path, defaultContent)
	}
	return deleteIfExists(path)
}

// writeDefault writes defaultContent to path, creating parent dirs if needed.
// Returns true if the file was actually changed (skips write when content already matches).
func writeDefault(path string, content string) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		return false, nil
	}

	dir := filepath.Dir(path)
	//nolint:gosec // G301: 0755 is appropriate for config directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("create directory %s: %w", dir, err)
	}
	//nolint:gosec // G306: 0644 is appropriate for config file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return false, err
	}
	return true, nil
}

// deleteIfExists removes a file if it exists. Returns true if the file was deleted.
func deleteIfExists(path string) (bool, error) {
	err := os.Remove(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
