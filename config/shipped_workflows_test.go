package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestShippedWorkflows_HaveDescription ensures every workflow.yaml shipped in
// the repo's top-level workflows/ directory has a non-empty top-level
// description field and parses cleanly as a full workflowFileData.
// Guards against a maintainer dropping the description when adding a new
// shipped workflow.
func TestShippedWorkflows_HaveDescription(t *testing.T) {
	matches, err := filepath.Glob("../workflows/*/workflow.yaml")
	if err != nil {
		t.Fatalf("glob shipped workflows: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no shipped workflows found at ../workflows/*/workflow.yaml")
	}

	for _, path := range matches {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			var desc struct {
				Description string `yaml:"description"`
			}
			if err := yaml.Unmarshal(data, &desc); err != nil {
				t.Fatalf("unmarshal description from %s: %v", path, err)
			}
			if desc.Description == "" {
				t.Errorf("%s: missing or empty top-level description", path)
			}

			if _, err := readWorkflowFile(path); err != nil {
				t.Errorf("readWorkflowFile(%s) failed: %v", path, err)
			}
		})
	}
}
