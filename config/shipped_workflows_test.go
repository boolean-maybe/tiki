package config

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestEmbeddedWorkflowNames_MatchSourceFiles ensures the registered embedded
// workflow names stay in sync with the YAML sources under config/workflows/.
func TestEmbeddedWorkflowNames_MatchSourceFiles(t *testing.T) {
	matches, err := filepath.Glob("workflows/*.yaml")
	if err != nil {
		t.Fatalf("glob embedded workflow files: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no embedded workflow files found at workflows/*.yaml")
	}

	names := make([]string, 0, len(matches))
	for _, path := range matches {
		base := filepath.Base(path)
		names = append(names, strings.TrimSuffix(base, filepath.Ext(base)))
	}
	sort.Strings(names)

	if got := EmbeddedWorkflowNames(); !reflect.DeepEqual(got, names) {
		t.Fatalf("EmbeddedWorkflowNames() = %v, want %v", got, names)
	}
}

// TestEmbeddedWorkflows_MatchSourceFiles ensures each registered embedded
// workflow matches its source file under config/workflows/.
func TestEmbeddedWorkflows_MatchSourceFiles(t *testing.T) {
	for _, name := range EmbeddedWorkflowNames() {
		t.Run(name, func(t *testing.T) {
			embedded, ok := LookupEmbeddedWorkflow(name)
			if !ok {
				t.Fatalf("embedded workflow %q not found", name)
			}

			sourcePath := filepath.Join("workflows", name+".yaml")
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatalf("read source %s: %v", sourcePath, err)
			}

			if embedded != string(source) {
				t.Errorf("embedded %q differs from source %s", name, sourcePath)
			}
		})
	}
}

// TestEmbeddedWorkflows_HaveDescription validates each embedded workflow has a
// parseable description field and source file that parses as a full workflow.
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

			sourcePath := filepath.Join("workflows", name+".yaml")
			data, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatalf("read %s: %v", sourcePath, err)
			}

			var sourceDesc struct {
				Description string `yaml:"description"`
			}
			if err := yaml.Unmarshal(data, &sourceDesc); err != nil {
				t.Fatalf("unmarshal description from %s: %v", sourcePath, err)
			}
			if sourceDesc.Description == "" {
				t.Errorf("%s: missing or empty top-level description", sourcePath)
			}

			if _, err := readWorkflowFile(sourcePath); err != nil {
				t.Errorf("readWorkflowFile(%s) failed: %v", sourcePath, err)
			}
		})
	}
}
