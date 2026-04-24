package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyWorkflowInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKind WorkflowSourceKind
	}{
		{"embedded bare name", "kanban", WorkflowSourceEmbedded},
		{"embedded bare name todo", "todo", WorkflowSourceEmbedded},
		{"embedded bare name bug-tracker", "bug-tracker", WorkflowSourceEmbedded},
		{"embedded unknown name", "sprint", WorkflowSourceEmbedded},
		{"url http", "http://example.com/w.yaml", WorkflowSourceURL},
		{"url https", "https://example.com/w.yaml", WorkflowSourceURL},
		{"file with slash", "./workflow.yaml", WorkflowSourceFile},
		{"file relative parent", "../custom.yaml", WorkflowSourceFile},
		{"file with yaml suffix", "my-workflow.yaml", WorkflowSourceFile},
		{"file with yml suffix", "my-workflow.yml", WorkflowSourceFile},
		{"file absolute path", "/tmp/workflow.yaml", WorkflowSourceFile},
		{"file path separator", "dir/workflow", WorkflowSourceFile},
		{"file ../../etc", "../../etc", WorkflowSourceFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := ClassifyWorkflowInput(tt.input)
			if err != nil {
				t.Fatalf("ClassifyWorkflowInput(%q) error = %v", tt.input, err)
			}
			if src.Kind != tt.wantKind {
				t.Errorf("kind = %d, want %d", src.Kind, tt.wantKind)
			}
		})
	}
}

func TestClassifyWorkflowInput_FilePathIsAbsolute(t *testing.T) {
	src, err := ClassifyWorkflowInput("./relative.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Kind != WorkflowSourceFile {
		t.Fatalf("kind = %d, want file", src.Kind)
	}
	if !filepath.IsAbs(src.Name) {
		t.Errorf("file path %q is not absolute", src.Name)
	}
}

func TestFetchWorkflowContent_Embedded(t *testing.T) {
	src := WorkflowSource{Kind: WorkflowSourceEmbedded, Name: "kanban"}
	content, err := FetchWorkflowContent(src)
	if err != nil {
		t.Fatalf("FetchWorkflowContent() error = %v", err)
	}
	if content == "" {
		t.Error("expected non-empty content for embedded kanban")
	}
}

func TestFetchWorkflowContent_UnknownEmbedded(t *testing.T) {
	src := WorkflowSource{Kind: WorkflowSourceEmbedded, Name: "nonexistent"}
	_, err := FetchWorkflowContent(src)
	if err == nil {
		t.Fatal("expected error for unknown embedded workflow")
	}
}

func TestFetchWorkflowContent_File(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test-workflow.yaml")
	if err := os.WriteFile(tmp, []byte("description: test file\n"), 0644); err != nil {
		t.Fatal(err)
	}

	src := WorkflowSource{Kind: WorkflowSourceFile, Name: tmp}
	content, err := FetchWorkflowContent(src)
	if err != nil {
		t.Fatalf("FetchWorkflowContent() error = %v", err)
	}
	if content != "description: test file\n" {
		t.Errorf("content = %q, want %q", content, "description: test file\n")
	}
}

func TestFetchWorkflowContent_URL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("description: remote workflow\n"))
	}))
	defer server.Close()

	src := WorkflowSource{Kind: WorkflowSourceURL, Name: server.URL + "/workflow.yaml"}
	content, err := FetchWorkflowContent(src)
	if err != nil {
		t.Fatalf("FetchWorkflowContent() error = %v", err)
	}
	if content != "description: remote workflow\n" {
		t.Errorf("content = %q", content)
	}
}

func TestDescribeWorkflowContent(t *testing.T) {
	desc, err := DescribeWorkflowContent("description: |\n  My workflow.\n  Two lanes.\nstatuses:\n  - key: todo\n")
	if err != nil {
		t.Fatalf("DescribeWorkflowContent() error = %v", err)
	}
	want := "My workflow.\nTwo lanes.\n"
	if desc != want {
		t.Errorf("description = %q, want %q", desc, want)
	}
}

func TestDescribeWorkflowContent_Empty(t *testing.T) {
	desc, err := DescribeWorkflowContent("statuses:\n  - key: todo\n")
	if err != nil {
		t.Fatalf("DescribeWorkflowContent() error = %v", err)
	}
	if desc != "" {
		t.Errorf("description = %q, want empty", desc)
	}
}

func TestValidateWorkflowContent_Valid(t *testing.T) {
	content := `version: 0.5.3
statuses:
  - key: todo
    label: Todo
    default: true
  - key: done
    label: Done
    done: true
types:
  - key: task
    label: Task
`
	vw, err := ValidateWorkflowContent(content)
	if err != nil {
		t.Fatalf("expected valid workflow, got: %v", err)
	}
	if vw.StatusReg == nil {
		t.Error("expected non-nil status registry")
	}
	if vw.TypeReg == nil {
		t.Error("expected non-nil type registry")
	}
}

func TestValidateWorkflowContent_MissingStatuses(t *testing.T) {
	content := "description: bad workflow\n"
	_, err := ValidateWorkflowContent(content)
	if err == nil {
		t.Fatal("expected error for workflow without statuses, got nil")
	}
}

func TestValidateWorkflowContent_MissingTypes(t *testing.T) {
	content := `statuses:
  - key: todo
    label: Todo
    default: true
`
	_, err := ValidateWorkflowContent(content)
	if err == nil {
		t.Fatal("expected error for workflow without types, got nil")
	}
}
