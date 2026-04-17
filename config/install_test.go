package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallWorkflow_Success(t *testing.T) {
	tikiDir := setupResetTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workflows/sprint/workflow.yaml":
			_, _ = w.Write([]byte("statuses:\n  - key: todo\n"))
		case "/workflows/sprint/new.md":
			_, _ = w.Write([]byte("---\ntitle:\n---\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	results, err := InstallWorkflow("sprint", ScopeGlobal, server.URL)
	if err != nil {
		t.Fatalf("InstallWorkflow() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Changed {
			t.Errorf("expected %s to be changed on fresh install", r.Path)
		}
	}

	got, err := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if err != nil {
		t.Fatalf("read workflow.yaml: %v", err)
	}
	if string(got) != "statuses:\n  - key: todo\n" {
		t.Errorf("workflow.yaml content = %q", string(got))
	}

	got, err = os.ReadFile(filepath.Join(tikiDir, "new.md"))
	if err != nil {
		t.Fatalf("read new.md: %v", err)
	}
	if string(got) != "---\ntitle:\n---\n" {
		t.Errorf("new.md content = %q", string(got))
	}
}

func TestInstallWorkflow_Overwrites(t *testing.T) {
	tikiDir := setupResetTest(t)

	if err := os.WriteFile(filepath.Join(tikiDir, "workflow.yaml"), []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("new content"))
	}))
	defer server.Close()

	results, err := InstallWorkflow("sprint", ScopeGlobal, server.URL)
	if err != nil {
		t.Fatalf("InstallWorkflow() error = %v", err)
	}
	for _, r := range results {
		if !r.Changed {
			t.Errorf("expected %s to be changed on overwrite", r.Path)
		}
	}

	got, _ := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if string(got) != "new content" {
		t.Errorf("workflow.yaml not overwritten: %q", string(got))
	}
}

func TestInstallWorkflow_NotFound(t *testing.T) {
	_ = setupResetTest(t)

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	_, err := InstallWorkflow("nonexistent", ScopeGlobal, server.URL)
	if err == nil {
		t.Fatal("expected error for nonexistent workflow, got nil")
	}
}

func TestInstallWorkflow_AlreadyUpToDate(t *testing.T) {
	_ = setupResetTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("same content"))
	}))
	defer server.Close()

	if _, err := InstallWorkflow("sprint", ScopeGlobal, server.URL); err != nil {
		t.Fatalf("first install: %v", err)
	}

	results, err := InstallWorkflow("sprint", ScopeGlobal, server.URL)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	for _, r := range results {
		if r.Changed {
			t.Errorf("expected %s to be unchanged on repeat install", r.Path)
		}
	}
}

func TestInstallWorkflow_InvalidName(t *testing.T) {
	_ = setupResetTest(t)

	for _, name := range []string{"../../etc", "a b", "", "foo/bar", "-dash", "dot."} {
		_, err := InstallWorkflow(name, ScopeGlobal, "http://unused")
		if err == nil {
			t.Errorf("expected error for name %q, got nil", name)
		}
	}
}

func TestInstallWorkflow_AtomicFetch(t *testing.T) {
	tikiDir := setupResetTest(t)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			_, _ = w.Write([]byte("ok"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err := InstallWorkflow("partial", ScopeGlobal, server.URL)
	if err == nil {
		t.Fatal("expected error for partial failure, got nil")
	}

	for _, filename := range []string{"workflow.yaml", "new.md"} {
		if _, statErr := os.Stat(filepath.Join(tikiDir, filename)); !os.IsNotExist(statErr) {
			t.Errorf("%s should not exist after fetch failure", filename)
		}
	}
}
