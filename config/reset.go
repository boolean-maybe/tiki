package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Scope identifies which config tier to operate on.
type Scope string

// ResetTarget identifies which config file to reset.
type ResetTarget string

const (
	ScopeGlobal  Scope = "global"
	ScopeLocal   Scope = "local"
	ScopeCurrent Scope = "current"

	TargetAll      ResetTarget = ""
	TargetConfig   ResetTarget = "config"
	TargetWorkflow ResetTarget = "workflow"
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
	{filename: defaultWorkflowFilename, defaultContent: embeddedKanbanYAML},
}

// ResetConfig resets configuration files for the given scope and target.
// Returns the list of file paths that were actually modified or deleted.
func ResetConfig(scope Scope, target ResetTarget) ([]string, error) {
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

// ResolveDir returns the directory path for the given scope.
func ResolveDir(scope Scope) (string, error) {
	return resolveDir(scope)
}

// resolveDir returns the directory path for the given scope.
func resolveDir(scope Scope) (string, error) {
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

// ValidResetTarget reports whether target is a recognized reset target.
func ValidResetTarget(target ResetTarget) bool {
	switch target {
	case TargetAll, TargetConfig, TargetWorkflow:
		return true
	default:
		return false
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
	default:
		return nil, fmt.Errorf("unknown target: %q (use config or workflow)", target)
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
func resetFile(path string, scope Scope, defaultContent string) (bool, error) {
	if scope == ScopeGlobal && defaultContent != "" {
		return writeFileIfChanged(path, defaultContent)
	}
	return deleteIfExists(path)
}

// writeFileIfChanged writes content to path, skipping if the file already has identical content.
// Returns true if the file was actually changed.
func writeFileIfChanged(path string, content string) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		return false, nil
	}
	return writeFile(path, content)
}

// writeFile writes content to path unconditionally, creating parent dirs if needed.
func writeFile(path string, content string) (bool, error) {
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
