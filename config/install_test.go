package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const validTestWorkflow = `version: 0.5.3
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

func TestInstallWorkflow_URL(t *testing.T) {
	tikiDir := setupResetTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/workflow.yaml" {
			_, _ = w.Write([]byte(validTestWorkflow))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	src := WorkflowSource{Kind: WorkflowSourceURL, Name: server.URL + "/workflow.yaml"}
	results, err := InstallWorkflow(src, ScopeGlobal)
	if err != nil {
		t.Fatalf("InstallWorkflow() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Changed {
		t.Errorf("expected %s to be changed on fresh install", results[0].Path)
	}

	got, err := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if err != nil {
		t.Fatalf("read workflow.yaml: %v", err)
	}
	if string(got) != validTestWorkflow {
		t.Errorf("workflow.yaml content mismatch")
	}
}

func TestInstallWorkflow_Embedded(t *testing.T) {
	tikiDir := setupResetTest(t)

	src := WorkflowSource{Kind: WorkflowSourceEmbedded, Name: "todo"}
	results, err := InstallWorkflow(src, ScopeGlobal)
	if err != nil {
		t.Fatalf("InstallWorkflow() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Changed {
		t.Errorf("expected file to be changed on fresh install")
	}

	got, err := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if err != nil {
		t.Fatalf("read workflow.yaml: %v", err)
	}
	embedded, _ := LookupEmbeddedWorkflow("todo")
	if string(got) != embedded {
		t.Errorf("installed content does not match embedded todo workflow")
	}
}

func TestInstallWorkflow_File(t *testing.T) {
	tikiDir := setupResetTest(t)

	srcFile := filepath.Join(t.TempDir(), "custom.yaml")
	if err := os.WriteFile(srcFile, []byte(validTestWorkflow), 0644); err != nil {
		t.Fatal(err)
	}

	src := WorkflowSource{Kind: WorkflowSourceFile, Name: srcFile}
	results, err := InstallWorkflow(src, ScopeGlobal)
	if err != nil {
		t.Fatalf("InstallWorkflow() error = %v", err)
	}
	if !results[0].Changed {
		t.Errorf("expected file to be changed")
	}

	got, _ := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if string(got) != validTestWorkflow {
		t.Errorf("workflow.yaml content mismatch")
	}
}

func TestInstallWorkflow_Overwrites(t *testing.T) {
	tikiDir := setupResetTest(t)

	if err := os.WriteFile(filepath.Join(tikiDir, "workflow.yaml"), []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(validTestWorkflow))
	}))
	defer server.Close()

	src := WorkflowSource{Kind: WorkflowSourceURL, Name: server.URL + "/workflow.yaml"}
	results, err := InstallWorkflow(src, ScopeGlobal)
	if err != nil {
		t.Fatalf("InstallWorkflow() error = %v", err)
	}
	for _, r := range results {
		if !r.Changed {
			t.Errorf("expected %s to be changed on overwrite", r.Path)
		}
	}

	got, _ := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if string(got) != validTestWorkflow {
		t.Errorf("workflow.yaml not overwritten")
	}
}

func TestInstallWorkflow_URLNotFound(t *testing.T) {
	_ = setupResetTest(t)

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	src := WorkflowSource{Kind: WorkflowSourceURL, Name: server.URL + "/workflow.yaml"}
	_, err := InstallWorkflow(src, ScopeGlobal)
	if err == nil {
		t.Fatal("expected error for URL not found, got nil")
	}
}

func TestInstallWorkflow_UnknownEmbedded(t *testing.T) {
	_ = setupResetTest(t)

	src := WorkflowSource{Kind: WorkflowSourceEmbedded, Name: "nonexistent"}
	_, err := InstallWorkflow(src, ScopeGlobal)
	if err == nil {
		t.Fatal("expected error for unknown embedded workflow, got nil")
	}
}

func TestInstallWorkflow_MissingFile(t *testing.T) {
	_ = setupResetTest(t)

	src := WorkflowSource{Kind: WorkflowSourceFile, Name: "/tmp/does-not-exist-workflow.yaml"}
	_, err := InstallWorkflow(src, ScopeGlobal)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestInstallWorkflowFromContent(t *testing.T) {
	tikiDir := setupResetTest(t)

	results, err := InstallWorkflowFromContent("test content", ScopeGlobal)
	if err != nil {
		t.Fatalf("InstallWorkflowFromContent() error = %v", err)
	}
	if !results[0].Changed {
		t.Error("expected file to be changed")
	}

	got, _ := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if string(got) != "test content" {
		t.Errorf("workflow.yaml = %q, want %q", got, "test content")
	}
}

func TestInstallWorkflow_AlreadyUpToDate(t *testing.T) {
	_ = setupResetTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(validTestWorkflow))
	}))
	defer server.Close()

	src := WorkflowSource{Kind: WorkflowSourceURL, Name: server.URL + "/workflow.yaml"}
	if _, err := InstallWorkflow(src, ScopeGlobal); err != nil {
		t.Fatalf("first install: %v", err)
	}

	results, err := InstallWorkflow(src, ScopeGlobal)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	for _, r := range results {
		if r.Changed {
			t.Errorf("expected %s to be unchanged on repeat install", r.Path)
		}
	}
}

func TestDescribeWorkflow_URL(t *testing.T) {
	_ = setupResetTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workflow.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("description: |\n  Sprint workflow.\n  Two-week cycles.\nstatuses:\n  - key: todo\n"))
	}))
	defer server.Close()

	src := WorkflowSource{Kind: WorkflowSourceURL, Name: server.URL + "/workflow.yaml"}
	desc, err := DescribeWorkflow(src)
	if err != nil {
		t.Fatalf("DescribeWorkflow() error = %v", err)
	}
	want := "Sprint workflow.\nTwo-week cycles.\n"
	if desc != want {
		t.Errorf("description = %q, want %q", desc, want)
	}
}

func TestDescribeWorkflow_Embedded(t *testing.T) {
	desc, err := DescribeWorkflow(WorkflowSource{Kind: WorkflowSourceEmbedded, Name: "kanban"})
	if err != nil {
		t.Fatalf("DescribeWorkflow() error = %v", err)
	}
	if desc == "" {
		t.Error("expected non-empty description for embedded kanban")
	}
}

func TestDescribeWorkflow_NoDescriptionField(t *testing.T) {
	_ = setupResetTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("statuses:\n  - key: todo\n"))
	}))
	defer server.Close()

	src := WorkflowSource{Kind: WorkflowSourceURL, Name: server.URL + "/workflow.yaml"}
	desc, err := DescribeWorkflow(src)
	if err != nil {
		t.Fatalf("DescribeWorkflow() error = %v", err)
	}
	if desc != "" {
		t.Errorf("description = %q, want empty", desc)
	}
}

func TestDescribeWorkflow_URLNotFound(t *testing.T) {
	_ = setupResetTest(t)

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	src := WorkflowSource{Kind: WorkflowSourceURL, Name: server.URL + "/workflow.yaml"}
	_, err := DescribeWorkflow(src)
	if err == nil {
		t.Fatal("expected error for URL not found, got nil")
	}
}
