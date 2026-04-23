package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
)

// setupWorkflowTest creates a temp config dir for workflow commands.
func setupWorkflowTest(t *testing.T) string {
	t.Helper()
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	config.ResetPathManager()
	t.Cleanup(config.ResetPathManager)

	tikiDir := filepath.Join(xdgDir, "tiki")
	if err := os.MkdirAll(tikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	return tikiDir
}

func overrideBaseURL(t *testing.T, url string) {
	t.Helper()
	orig := config.DefaultWorkflowBaseURL
	config.DefaultWorkflowBaseURL = url
	t.Cleanup(func() { config.DefaultWorkflowBaseURL = orig })
}

func TestParseScopeArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		positional string
		scope      config.Scope
		wantErr    error
		errSubstr  string
	}{
		{
			name:       "global no positional",
			args:       []string{"--global"},
			positional: "",
			scope:      config.ScopeGlobal,
		},
		{
			name:       "local with positional",
			args:       []string{"workflow", "--local"},
			positional: "workflow",
			scope:      config.ScopeLocal,
		},
		{
			name:       "scope before positional",
			args:       []string{"--current", "config"},
			positional: "config",
			scope:      config.ScopeCurrent,
		},
		{
			name:    "help flag",
			args:    []string{"--help"},
			wantErr: errHelpRequested,
		},
		{
			name:    "short help flag",
			args:    []string{"-h"},
			wantErr: errHelpRequested,
		},
		{
			name:       "missing scope defaults to local",
			args:       []string{"config"},
			positional: "config",
			scope:      config.ScopeLocal,
		},
		{
			name:      "unknown flag",
			args:      []string{"--verbose"},
			errSubstr: "unknown flag",
		},
		{
			name:      "multiple positional",
			args:      []string{"config", "workflow", "--global"},
			errSubstr: "multiple positional arguments",
		},
		{
			name:      "duplicate scopes",
			args:      []string{"--global", "--local"},
			errSubstr: "only one scope allowed",
		},
		{
			name:  "no args defaults to local",
			args:  nil,
			scope: config.ScopeLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			positional, scope, err := parseScopeArgs(tt.args)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if tt.errSubstr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if msg := err.Error(); !strings.Contains(msg, tt.errSubstr) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, msg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if positional != tt.positional {
				t.Errorf("positional = %q, want %q", positional, tt.positional)
			}
			if scope != tt.scope {
				t.Errorf("scope = %q, want %q", scope, tt.scope)
			}
		})
	}
}

func TestParsePositionalOnly(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		positional string
		wantErr    error
		errSubstr  string
	}{
		{name: "no args", args: nil},
		{name: "single positional", args: []string{"sprint"}, positional: "sprint"},
		{name: "help flag", args: []string{"--help"}, wantErr: errHelpRequested},
		{name: "short help flag", args: []string{"-h"}, wantErr: errHelpRequested},
		{name: "rejects scope", args: []string{"sprint", "--global"}, errSubstr: "unknown flag"},
		{name: "rejects unknown flag", args: []string{"sprint", "--verbose"}, errSubstr: "unknown flag"},
		{name: "multiple positional", args: []string{"a", "b"}, errSubstr: "multiple positional arguments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			positional, err := parsePositionalOnly(tt.args)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if tt.errSubstr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if msg := err.Error(); !strings.Contains(msg, tt.errSubstr) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, msg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if positional != tt.positional {
				t.Errorf("positional = %q, want %q", positional, tt.positional)
			}
		})
	}
}

// --- runWorkflow dispatch tests ---

func TestRunWorkflow_NoArgs(t *testing.T) {
	if code := runWorkflow(nil); code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestRunWorkflow_UnknownSubcommand(t *testing.T) {
	if code := runWorkflow([]string{"bogus"}); code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestRunWorkflow_Help(t *testing.T) {
	if code := runWorkflow([]string{"--help"}); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
}

// --- runWorkflowReset integration tests ---

func TestRunWorkflowReset_GlobalAll(t *testing.T) {
	tikiDir := setupWorkflowTest(t)

	if err := os.WriteFile(filepath.Join(tikiDir, "workflow.yaml"), []byte("custom"), 0644); err != nil {
		t.Fatal(err)
	}

	if code := runWorkflowReset([]string{"--global"}); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}

	got, err := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == "custom" {
		t.Error("workflow.yaml was not reset")
	}
}

func TestRunWorkflowReset_NothingToReset(t *testing.T) {
	_ = setupWorkflowTest(t)

	if code := runWorkflowReset([]string{"config", "--global"}); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
}

func TestRunWorkflowReset_InvalidTarget(t *testing.T) {
	_ = setupWorkflowTest(t)

	if code := runWorkflowReset([]string{"themes", "--global"}); code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestRunWorkflowReset_DefaultsToLocal(t *testing.T) {
	_ = setupWorkflowTest(t)

	// without an initialized project, --local scope fails with exitInternal (not exitUsage)
	if code := runWorkflowReset([]string{"config"}); code == exitUsage {
		t.Error("missing scope should not produce usage error — it should default to --local")
	}
}

func TestRunWorkflowReset_Help(t *testing.T) {
	if code := runWorkflowReset([]string{"--help"}); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
}

// --- runWorkflowInstall integration tests ---

func TestRunWorkflowInstall_Success(t *testing.T) {
	tikiDir := setupWorkflowTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workflows/sprint/workflow.yaml":
			_, _ = w.Write([]byte("sprint workflow"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	overrideBaseURL(t, server.URL)

	if code := runWorkflowInstall([]string{"sprint", "--global"}); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	got, _ := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if string(got) != "sprint workflow" {
		t.Errorf("workflow.yaml = %q, want %q", got, "sprint workflow")
	}
}

func TestRunWorkflowInstall_MissingName(t *testing.T) {
	_ = setupWorkflowTest(t)

	if code := runWorkflowInstall([]string{"--global"}); code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestRunWorkflowInstall_InvalidName(t *testing.T) {
	_ = setupWorkflowTest(t)

	if code := runWorkflowInstall([]string{"../../etc", "--global"}); code != exitInternal {
		t.Errorf("exit code = %d, want %d", code, exitInternal)
	}
}

func TestRunWorkflowInstall_NotFound(t *testing.T) {
	_ = setupWorkflowTest(t)

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	overrideBaseURL(t, server.URL)

	if code := runWorkflowInstall([]string{"nonexistent", "--global"}); code != exitInternal {
		t.Errorf("exit code = %d, want %d", code, exitInternal)
	}
}

func TestRunWorkflowInstall_DefaultsToLocal(t *testing.T) {
	_ = setupWorkflowTest(t)

	// without an initialized project, --local scope fails with exitInternal (not exitUsage)
	if code := runWorkflowInstall([]string{"sprint"}); code == exitUsage {
		t.Error("missing scope should not produce usage error — it should default to --local")
	}
}

func TestRunWorkflowInstall_Help(t *testing.T) {
	if code := runWorkflowInstall([]string{"--help"}); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
}

// --- runWorkflowDescribe integration tests ---

func TestRunWorkflowDescribe_Success(t *testing.T) {
	_ = setupWorkflowTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workflows/sprint/workflow.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("description: |\n  sprint desc\n"))
	}))
	defer server.Close()
	overrideBaseURL(t, server.URL)

	if code := runWorkflowDescribe([]string{"sprint"}); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
}

func TestRunWorkflowDescribe_MissingName(t *testing.T) {
	_ = setupWorkflowTest(t)

	if code := runWorkflowDescribe(nil); code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestRunWorkflowDescribe_InvalidName(t *testing.T) {
	_ = setupWorkflowTest(t)

	if code := runWorkflowDescribe([]string{"../../etc"}); code != exitInternal {
		t.Errorf("exit code = %d, want %d", code, exitInternal)
	}
}

func TestRunWorkflowDescribe_NotFound(t *testing.T) {
	_ = setupWorkflowTest(t)

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	overrideBaseURL(t, server.URL)

	if code := runWorkflowDescribe([]string{"nonexistent"}); code != exitInternal {
		t.Errorf("exit code = %d, want %d", code, exitInternal)
	}
}

func TestRunWorkflowDescribe_UnknownFlag(t *testing.T) {
	_ = setupWorkflowTest(t)

	if code := runWorkflowDescribe([]string{"sprint", "--verbose"}); code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestRunWorkflowDescribe_RejectsScopeFlags(t *testing.T) {
	_ = setupWorkflowTest(t)

	for _, flag := range []string{"--global", "--local", "--current"} {
		if code := runWorkflowDescribe([]string{"sprint", flag}); code != exitUsage {
			t.Errorf("%s: exit code = %d, want %d", flag, code, exitUsage)
		}
	}
}

func TestRunWorkflowDescribe_Help(t *testing.T) {
	if code := runWorkflowDescribe([]string{"--help"}); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
}

func TestRunWorkflow_DescribeDispatch(t *testing.T) {
	_ = setupWorkflowTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("description: hi\n"))
	}))
	defer server.Close()
	overrideBaseURL(t, server.URL)

	if code := runWorkflow([]string{"describe", "sprint"}); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
}
