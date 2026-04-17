package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupResetTest creates a temp config dir, sets XDG_CONFIG_HOME, and resets
// the path manager so GetConfigDir() points to the temp dir.
// Returns the tiki config dir (e.g. <tmp>/tiki).
func setupResetTest(t *testing.T) string {
	t.Helper()
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	ResetPathManager()
	t.Cleanup(ResetPathManager)

	tikiDir := filepath.Join(xdgDir, "tiki")
	if err := os.MkdirAll(tikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	return tikiDir
}

// writeTestFile is a test helper that writes content to path.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResetConfig_GlobalAll(t *testing.T) {
	tikiDir := setupResetTest(t)

	// seed all three files with custom content
	writeTestFile(t, filepath.Join(tikiDir, "config.yaml"), "logging:\n  level: debug\n")
	writeTestFile(t, filepath.Join(tikiDir, "workflow.yaml"), "custom: true\n")
	writeTestFile(t, filepath.Join(tikiDir, "new.md"), "custom template\n")

	affected, err := ResetConfig(ScopeGlobal, TargetAll)
	if err != nil {
		t.Fatalf("ResetConfig() error = %v", err)
	}
	if len(affected) != 3 {
		t.Fatalf("expected 3 affected files, got %d: %v", len(affected), affected)
	}

	// config.yaml should be deleted (no embedded default)
	if _, err := os.Stat(filepath.Join(tikiDir, "config.yaml")); !os.IsNotExist(err) {
		t.Error("config.yaml should be deleted after global reset")
	}

	// workflow.yaml should contain embedded default
	got, err := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if err != nil {
		t.Fatalf("read workflow.yaml: %v", err)
	}
	if string(got) != GetDefaultWorkflowYAML() {
		t.Error("workflow.yaml does not match embedded default after global reset")
	}

	// new.md should contain embedded default
	got, err = os.ReadFile(filepath.Join(tikiDir, "new.md"))
	if err != nil {
		t.Fatalf("read new.md: %v", err)
	}
	if string(got) != GetDefaultNewTaskTemplate() {
		t.Error("new.md does not match embedded default after global reset")
	}
}

func TestResetConfig_GlobalSingleTarget(t *testing.T) {
	tests := []struct {
		target   ResetTarget
		filename string
		deleted  bool // true = file deleted, false = file overwritten with default
	}{
		{TargetConfig, "config.yaml", true},
		{TargetWorkflow, "workflow.yaml", false},
		{TargetNew, "new.md", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.target), func(t *testing.T) {
			tikiDir := setupResetTest(t)

			writeTestFile(t, filepath.Join(tikiDir, tt.filename), "custom\n")

			affected, err := ResetConfig(ScopeGlobal, tt.target)
			if err != nil {
				t.Fatalf("ResetConfig() error = %v", err)
			}
			if len(affected) != 1 {
				t.Fatalf("expected 1 affected file, got %d", len(affected))
			}

			_, statErr := os.Stat(filepath.Join(tikiDir, tt.filename))
			if tt.deleted {
				if !os.IsNotExist(statErr) {
					t.Errorf("%s should be deleted", tt.filename)
				}
			} else {
				if statErr != nil {
					t.Errorf("%s should exist after reset: %v", tt.filename, statErr)
				}
			}
		})
	}
}

func TestResetConfig_LocalDeletesFiles(t *testing.T) {
	tikiDir := setupResetTest(t)

	// set up project dir with .doc/tiki so IsProjectInitialized() passes
	projectDir := t.TempDir()
	docDir := filepath.Join(projectDir, ".doc")
	if err := os.MkdirAll(filepath.Join(docDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	pm := mustGetPathManager()
	pm.projectRoot = projectDir

	// seed project config files
	writeTestFile(t, filepath.Join(docDir, "config.yaml"), "custom\n")
	writeTestFile(t, filepath.Join(docDir, "workflow.yaml"), "custom\n")
	writeTestFile(t, filepath.Join(docDir, "new.md"), "custom\n")

	// also write global defaults so we can verify local doesn't overwrite
	writeTestFile(t, filepath.Join(tikiDir, "workflow.yaml"), "global\n")

	affected, err := ResetConfig(ScopeLocal, TargetAll)
	if err != nil {
		t.Fatalf("ResetConfig() error = %v", err)
	}
	if len(affected) != 3 {
		t.Fatalf("expected 3 affected files, got %d: %v", len(affected), affected)
	}

	// all project files should be deleted
	for _, name := range []string{"config.yaml", "workflow.yaml", "new.md"} {
		if _, err := os.Stat(filepath.Join(docDir, name)); !os.IsNotExist(err) {
			t.Errorf("project %s should be deleted after local reset", name)
		}
	}

	// global workflow should be untouched
	got, err := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if err != nil {
		t.Fatalf("global workflow.yaml should still exist: %v", err)
	}
	if string(got) != "global\n" {
		t.Error("global workflow.yaml should be untouched after local reset")
	}
}

func TestResetConfig_CurrentDeletesFiles(t *testing.T) {
	_ = setupResetTest(t)

	cwdDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	writeTestFile(t, filepath.Join(cwdDir, "workflow.yaml"), "override\n")

	affected, err := ResetConfig(ScopeCurrent, TargetWorkflow)
	if err != nil {
		t.Fatalf("ResetConfig() error = %v", err)
	}
	if len(affected) != 1 {
		t.Fatalf("expected 1 affected file, got %d", len(affected))
	}
	if _, err := os.Stat(filepath.Join(cwdDir, "workflow.yaml")); !os.IsNotExist(err) {
		t.Error("cwd workflow.yaml should be deleted after current reset")
	}
}

func TestResetConfig_IdempotentOnMissingFiles(t *testing.T) {
	_ = setupResetTest(t)

	// reset when no files exist — should succeed with 0 affected
	affected, err := ResetConfig(ScopeGlobal, TargetConfig)
	if err != nil {
		t.Fatalf("ResetConfig() error = %v", err)
	}
	if len(affected) != 0 {
		t.Errorf("expected 0 affected files for missing config.yaml, got %d", len(affected))
	}
}

func TestResetConfig_GlobalWorkflowCreatesDir(t *testing.T) {
	// use a fresh temp dir where tiki subdir doesn't exist yet
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	ResetPathManager()
	t.Cleanup(ResetPathManager)

	affected, err := ResetConfig(ScopeGlobal, TargetWorkflow)
	if err != nil {
		t.Fatalf("ResetConfig() error = %v", err)
	}
	if len(affected) != 1 {
		t.Fatalf("expected 1 affected file, got %d", len(affected))
	}

	// should have created the directory and written the default
	tikiDir := filepath.Join(xdgDir, "tiki")
	got, err := os.ReadFile(filepath.Join(tikiDir, "workflow.yaml"))
	if err != nil {
		t.Fatalf("read workflow.yaml: %v", err)
	}
	if string(got) != GetDefaultWorkflowYAML() {
		t.Error("workflow.yaml should match embedded default")
	}
}

func TestResetConfig_GlobalSkipsWhenAlreadyDefault(t *testing.T) {
	tikiDir := setupResetTest(t)

	// write the embedded default content — reset should detect no change
	writeTestFile(t, filepath.Join(tikiDir, "workflow.yaml"), GetDefaultWorkflowYAML())

	affected, err := ResetConfig(ScopeGlobal, TargetWorkflow)
	if err != nil {
		t.Fatalf("ResetConfig() error = %v", err)
	}
	if len(affected) != 0 {
		t.Errorf("expected 0 affected files when already default, got %d", len(affected))
	}
}

func TestValidResetTarget(t *testing.T) {
	valid := []ResetTarget{TargetAll, TargetConfig, TargetWorkflow, TargetNew}
	for _, target := range valid {
		if !ValidResetTarget(target) {
			t.Errorf("ValidResetTarget(%q) = false, want true", target)
		}
	}
	invalid := []ResetTarget{"themes", "invalid", "reset"}
	for _, target := range invalid {
		if ValidResetTarget(target) {
			t.Errorf("ValidResetTarget(%q) = true, want false", target)
		}
	}
}

func TestResetConfig_LocalRejectsUninitializedProject(t *testing.T) {
	// point projectRoot at a temp dir that has no .doc/tiki
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	ResetPathManager()
	t.Cleanup(ResetPathManager)

	pm := mustGetPathManager()
	pm.projectRoot = t.TempDir() // empty dir — not initialized

	_, err := ResetConfig(ScopeLocal, TargetAll)
	if err == nil {
		t.Fatal("expected error for uninitialized project, got nil")
	}
	if msg := err.Error(); !strings.Contains(msg, "not in an initialized tiki project") {
		t.Errorf("unexpected error message: %s", msg)
	}
}
