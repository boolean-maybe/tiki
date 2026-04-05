package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// TriggerDef represents a single trigger entry in workflow.yaml.
type TriggerDef struct {
	Description string `yaml:"description"`
	Ruki        string `yaml:"ruki"`
}

// triggerFileData is the minimal YAML structure for reading triggers from workflow.yaml.
type triggerFileData struct {
	Triggers []TriggerDef `yaml:"triggers"`
}

// LoadTriggerDefs discovers and returns raw trigger definitions from workflow.yaml files.
// Uses its own discovery path (not FindWorkflowFiles) to avoid the empty-views filter.
// Override semantics: last file with a triggers: section wins (cwd > project > user).
// An explicit empty list (triggers: []) overrides inherited triggers.
// The caller is responsible for parsing each TriggerDef.Ruki with ruki.ParseTrigger.
func LoadTriggerDefs() ([]TriggerDef, error) {
	pm := mustGetPathManager()

	// candidate paths in discovery order: user config → project config → cwd
	candidates := []string{
		pm.UserConfigWorkflowFile(),
		filepath.Join(pm.ProjectConfigDir(), defaultWorkflowFilename),
		defaultWorkflowFilename, // relative to cwd
	}

	var winningDefs []TriggerDef
	seen := make(map[string]bool)

	for _, path := range candidates {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true

		defs, found, err := readTriggersFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading triggers from %s: %w", path, err)
		}
		if found {
			// last file with a triggers: section wins
			winningDefs = defs
		}
	}

	return winningDefs, nil
}

// readTriggersFromFile reads a workflow.yaml and returns its triggers section.
// Returns (defs, true, nil) if the file exists and has a triggers: key.
// Returns (nil, false, nil) if the file doesn't exist or has no triggers: key.
func readTriggersFromFile(path string) ([]TriggerDef, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	// check whether the YAML contains a triggers: key at all
	// (absent section = no opinion, does not override)
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, false, fmt.Errorf("parsing YAML: %w", err)
	}
	if _, hasTriggers := raw["triggers"]; !hasTriggers {
		return nil, false, nil
	}

	var tf triggerFileData
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, false, fmt.Errorf("parsing triggers: %w", err)
	}

	return tf.Triggers, true, nil
}
