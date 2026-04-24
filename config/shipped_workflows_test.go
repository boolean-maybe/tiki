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

// TestEmbeddedWorkflows_MatchShipped ensures the embedded copies under
// config/workflows/ are identical to the shipped copies under workflows/.
func TestEmbeddedWorkflows_MatchShipped(t *testing.T) {
	mapping := map[string]string{
		"kanban":      "../workflows/kanban/workflow.yaml",
		"todo":        "../workflows/todo/workflow.yaml",
		"bug-tracker": "../workflows/bug-tracker/workflow.yaml",
	}

	for name, shippedPath := range mapping {
		t.Run(name, func(t *testing.T) {
			embedded, ok := LookupEmbeddedWorkflow(name)
			if !ok {
				t.Fatalf("embedded workflow %q not found", name)
			}

			shipped, err := os.ReadFile(shippedPath)
			if err != nil {
				t.Fatalf("read shipped %s: %v", shippedPath, err)
			}

			if embedded != string(shipped) {
				t.Errorf("embedded %q differs from shipped %s — run: cp %s config/workflows/%s.yaml",
					name, shippedPath, shippedPath, name)
			}
		})
	}
}

// TestEmbeddedWorkflows_HaveDescription validates each embedded workflow
// has a parseable description field.
func TestEmbeddedWorkflows_HaveDescription(t *testing.T) {
	for _, name := range EmbeddedWorkflowNames() {
		t.Run(name, func(t *testing.T) {
			content, _ := LookupEmbeddedWorkflow(name)
			desc, err := DescribeWorkflowContent(content)
			if err != nil {
				t.Fatalf("parse embedded %s: %v", name, err)
			}
			if desc == "" {
				t.Errorf("embedded %s has empty description", name)
			}
		})
	}
}
