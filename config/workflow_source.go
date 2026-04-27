package config

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/workflow"
	"gopkg.in/yaml.v3"
)

// WorkflowSourceKind identifies how a workflow source should be resolved.
type WorkflowSourceKind int

const (
	WorkflowSourceEmbedded WorkflowSourceKind = iota
	WorkflowSourceFile
	WorkflowSourceURL
)

// WorkflowSource pairs a kind with the resolved name/path/URL.
type WorkflowSource struct {
	Kind WorkflowSourceKind
	Name string
}

// ClassifyWorkflowInput determines whether value is an embedded name, file path, or URL.
// File paths are resolved to absolute via filepath.Abs so they survive later os.Chdir calls.
func ClassifyWorkflowInput(value string) (WorkflowSource, error) {
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return WorkflowSource{Kind: WorkflowSourceURL, Name: value}, nil
	}

	if strings.Contains(value, "/") || strings.Contains(value, string(filepath.Separator)) ||
		filepath.IsAbs(value) ||
		strings.HasSuffix(value, ".yaml") || strings.HasSuffix(value, ".yml") {
		abs, err := filepath.Abs(value)
		if err != nil {
			return WorkflowSource{}, fmt.Errorf("resolve workflow path %q: %w", value, err)
		}
		return WorkflowSource{Kind: WorkflowSourceFile, Name: abs}, nil
	}

	return WorkflowSource{Kind: WorkflowSourceEmbedded, Name: value}, nil
}

// FetchWorkflowContent returns the YAML content for the given source.
func FetchWorkflowContent(src WorkflowSource) (string, error) {
	switch src.Kind {
	case WorkflowSourceEmbedded:
		content, ok := LookupEmbeddedWorkflow(src.Name)
		if !ok {
			return "", fmt.Errorf("unknown embedded workflow %q (available: %s)",
				src.Name, strings.Join(EmbeddedWorkflowNames(), ", "))
		}
		return content, nil

	case WorkflowSourceFile:
		data, err := os.ReadFile(src.Name)
		if err != nil {
			return "", fmt.Errorf("read workflow file %q: %w", src.Name, err)
		}
		return string(data), nil

	case WorkflowSourceURL:
		return fetchWorkflowURL(src.Name)

	default:
		return "", fmt.Errorf("unknown workflow source kind: %d", src.Kind)
	}
}

// DescribeWorkflowContent extracts the top-level description field from workflow YAML.
func DescribeWorkflowContent(yamlContent string) (string, error) {
	var wf struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(yamlContent), &wf); err != nil {
		return "", fmt.Errorf("parse workflow YAML: %w", err)
	}
	return wf.Description, nil
}

// ValidateWorkflowContent checks that the YAML content defines a usable workflow
// (version compatibility, statuses, types, custom fields, triggers).
// Returns registries and trigger defs for callers that need to do further
// validation (e.g. trigger rule parsing which requires a ruki.Schema that
// config cannot construct without a circular import).
func ValidateWorkflowContent(content string) (*ValidatedWorkflow, error) {
	tmp, err := os.CreateTemp("", "tiki-workflow-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write([]byte(content)); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	statusReg, typeReg, fieldDefs, err := LoadRegistriesFromFile(tmp.Name())
	if err != nil {
		return nil, err
	}

	triggerDefs, err := LoadTriggerDefsFromFile(tmp.Name())
	if err != nil {
		return nil, err
	}

	return &ValidatedWorkflow{
		StatusReg:   statusReg,
		TypeReg:     typeReg,
		FieldDefs:   fieldDefs,
		TriggerDefs: triggerDefs,
	}, nil
}

// ValidatedWorkflow holds the parsed components of a validated workflow file.
type ValidatedWorkflow struct {
	StatusReg   *workflow.StatusRegistry
	TypeReg     *workflow.TypeRegistry
	FieldDefs   []workflow.FieldDef
	TriggerDefs []TriggerDef
}

func fetchWorkflowURL(url string) (string, error) {
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
		return "", fmt.Errorf("workflow not found at %s", url)
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
