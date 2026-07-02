package config

import (
	"os"
	"path/filepath"
	"testing"
)

// --- readTriggersFromFile unit tests ---

func TestReadTriggersFromFile_NonExistent(t *testing.T) {
	defs, found, err := readTriggersFromFile("/no/such/file.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected found=false for missing file")
	}
	if defs != nil {
		t.Fatalf("expected nil defs, got %v", defs)
	}
}

func TestReadTriggersFromFile_NoTriggersKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	if err := os.WriteFile(path, []byte("views:\n  - name: board\n"), 0644); err != nil {
		t.Fatal(err)
	}

	defs, found, err := readTriggersFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected found=false when triggers: key absent")
	}
	if defs != nil {
		t.Fatalf("expected nil defs, got %v", defs)
	}
}

func TestReadTriggersFromFile_EmptyTriggersList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	if err := os.WriteFile(path, []byte("triggers: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	defs, found, err := readTriggersFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true when triggers: key present (even if empty)")
	}
	if len(defs) != 0 {
		t.Fatalf("expected 0 defs, got %d", len(defs))
	}
}

func TestReadTriggersFromFile_WithTriggers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	content := `triggers:
  - description: "block done"
    ruki: 'before update where new.status = "done" deny "no"'
  - description: "auto-assign"
    ruki: 'after create where new.assignee is empty update where id = new.id set assignee="bot"'
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	defs, found, err := readTriggersFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if len(defs) != 2 {
		t.Fatalf("expected 2 defs, got %d", len(defs))
	}
	if defs[0].Description != "block done" {
		t.Errorf("defs[0].Description = %q, want %q", defs[0].Description, "block done")
	}
	if defs[1].Description != "auto-assign" {
		t.Errorf("defs[1].Description = %q, want %q", defs[1].Description, "auto-assign")
	}
}

func TestReadTriggersFromFile_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	if err := os.WriteFile(path, []byte(":\ninvalid: [yaml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := readTriggersFromFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

// --- LoadTriggerDefs precedence tests ---

// setupTriggerPrecedenceTest creates temp dirs for the two config tiers — user
// and cwd root (projectRoot == cwd) — resets the PathManager, and chdirs into
// the cwd dir. There is no separate project (".doc") tier.
func setupTriggerPrecedenceTest(t *testing.T) (userTikiDir, cwdDir string) {
	t.Helper()

	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	userTikiDir = filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(userTikiDir, 0750); err != nil {
		t.Fatal(err)
	}

	cwdDir = t.TempDir()

	originalDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	_ = os.Chdir(cwdDir)

	// projectRoot mirrors the runtime invariant projectRoot == cwd.
	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = cwdDir

	return userTikiDir, cwdDir
}

func writeTriggerFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTriggerDefs_CwdOverridesUser(t *testing.T) {
	userDir, cwdDir := setupTriggerPrecedenceTest(t)

	writeTriggerFile(t, userDir, `triggers:
  - description: "user trigger"
    ruki: 'before update deny "user"'
`)
	writeTriggerFile(t, cwdDir, `triggers:
  - description: "cwd trigger"
    ruki: 'before update deny "cwd"'
`)

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 def (cwd wins), got %d", len(defs))
	}
	if defs[0].Description != "cwd trigger" {
		t.Errorf("expected cwd trigger, got %q", defs[0].Description)
	}
}

func TestLoadTriggerDefs_UserFallback(t *testing.T) {
	userDir, _ := setupTriggerPrecedenceTest(t)

	writeTriggerFile(t, userDir, `triggers:
  - description: "user trigger"
    ruki: 'before update deny "user"'
`)
	// no cwd workflow.yaml

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 def (user fallback), got %d", len(defs))
	}
	if defs[0].Description != "user trigger" {
		t.Errorf("expected user trigger, got %q", defs[0].Description)
	}
}

func TestLoadTriggerDefs_EmptyListIsAuthoritative(t *testing.T) {
	_, cwdDir := setupTriggerPrecedenceTest(t)

	// winning file explicitly has empty triggers
	writeTriggerFile(t, cwdDir, "triggers: []\n")

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected 0 defs from explicit empty triggers, got %d", len(defs))
	}
}

func TestLoadTriggerDefs_MissingTriggersKeyMeansNone(t *testing.T) {
	_, cwdDir := setupTriggerPrecedenceTest(t)

	// winning file has no triggers: key at all
	writeTriggerFile(t, cwdDir, "views:\n  plugins:\n    - name: board\n")

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected 0 defs when triggers: key is absent, got %d", len(defs))
	}
}

func TestLoadTriggerDefs_NoWorkflowFiles(t *testing.T) {
	_, _ = setupTriggerPrecedenceTest(t)
	// no workflow.yaml files anywhere

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected 0 defs, got %d", len(defs))
	}
}

func TestReadTriggersFromFile_MalformedTriggersSection(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "workflow.yaml")
	// triggers key is present but with wrong type (string instead of list)
	content := "triggers: \"not a list\"\n"
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, err := readTriggersFromFile(f)
	if err == nil {
		t.Fatal("expected error for malformed triggers section")
	}
}

func TestReadTriggersFromFile_PermissionError(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(f, []byte("triggers: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// make file unreadable
	if err := os.Chmod(f, 0000); err != nil {
		t.Skip("cannot change file permissions on this platform")
	}
	t.Cleanup(func() { _ = os.Chmod(f, 0600) })
	// on Windows, chmod succeeds but doesn't restrict reads — verify it actually worked
	if r, openErr := os.Open(f); openErr == nil {
		_ = r.Close()
		t.Skip("chmod 0000 did not restrict read access on this platform")
	}

	_, _, err := readTriggersFromFile(f)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
}

func TestLoadTriggerDefs_FileReadError(t *testing.T) {
	userDir, cwdDir := setupTriggerPrecedenceTest(t)

	// user dir has a valid file
	writeTriggerFile(t, userDir, `triggers:
  - description: "user trigger"
    ruki: 'before update deny "user"'
`)

	// cwd-root has an unreadable file (not invalid YAML, but unreadable)
	f := filepath.Join(cwdDir, "workflow.yaml")
	if err := os.WriteFile(f, []byte("triggers: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f, 0000); err != nil {
		t.Skip("cannot change file permissions on this platform")
	}
	t.Cleanup(func() { _ = os.Chmod(f, 0600) })
	if r, openErr := os.Open(f); openErr == nil {
		_ = r.Close()
		t.Skip("chmod 0000 did not restrict read access on this platform")
	}

	_, err := LoadTriggerDefs()
	if err == nil {
		t.Fatal("expected error for unreadable workflow.yaml")
	}
}

func TestLoadTriggerDefs_CwdEqualsProjectConfigDir(t *testing.T) {
	// at runtime projectRoot == cwd, so the user-config candidate and the
	// cwd-root candidate (ProjectConfigDir == projectRoot) are the only two.
	// A workflow.yaml at the cwd root must be read exactly once.
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir()
	// resolve symlinks so projectRoot matches what filepath.Abs returns from cwd
	// (on macOS /var/folders -> /private/var/folders via symlink)
	projectDir, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		t.Fatal(err)
	}

	// set cwd to the project root so projectRoot == cwd, the runtime invariant.
	originalDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	_ = os.Chdir(projectDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = projectDir

	writeTriggerFile(t, projectDir, `triggers:
  - description: "doc trigger"
    ruki: 'before update deny "doc"'
`)

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// the cwd-root file should be read exactly once
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Description != "doc trigger" {
		t.Errorf("expected 'doc trigger', got %q", defs[0].Description)
	}
}
