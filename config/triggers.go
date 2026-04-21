package config

import (
	"fmt"
	"os"

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

// LoadTriggerDefs reads trigger definitions from the single highest-priority
// workflow.yaml. Missing triggers: section means no triggers.
func LoadTriggerDefs() ([]TriggerDef, error) {
	path := FindWorkflowFile()
	if path == "" {
		return nil, nil
	}

	defs, _, err := readTriggersFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading triggers from %s: %w", path, err)
	}

	return defs, nil
}

// LoadTriggerDefsFromFile reads trigger definitions from an explicit workflow
// file path. Used by init to validate triggers without global path discovery.
func LoadTriggerDefsFromFile(path string) ([]TriggerDef, error) {
	defs, _, err := readTriggersFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading triggers from %s: %w", path, err)
	}
	return defs, nil
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
