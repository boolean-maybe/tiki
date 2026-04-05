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

// setupTriggerPrecedenceTest creates temp dirs for user, project, and cwd,
// resets the PathManager, and returns a cleanup function.
func setupTriggerPrecedenceTest(t *testing.T) (userTikiDir, projectDocDir, cwdDir string) {
	t.Helper()

	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	userTikiDir = filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(userTikiDir, 0750); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir()
	projectDocDir = filepath.Join(projectDir, ".doc")
	if err := os.MkdirAll(projectDocDir, 0750); err != nil {
		t.Fatal(err)
	}

	cwdDir = t.TempDir()

	originalDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	_ = os.Chdir(cwdDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = projectDir

	return userTikiDir, projectDocDir, cwdDir
}

func writeTriggerFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTriggerDefs_CwdOverridesProjectAndUser(t *testing.T) {
	userDir, projectDir, cwdDir := setupTriggerPrecedenceTest(t)

	writeTriggerFile(t, userDir, `triggers:
  - description: "user trigger"
    ruki: 'before update deny "user"'
`)
	writeTriggerFile(t, projectDir, `triggers:
  - description: "project trigger"
    ruki: 'before update deny "project"'
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

func TestLoadTriggerDefs_ProjectOverridesUser(t *testing.T) {
	userDir, projectDir, _ := setupTriggerPrecedenceTest(t)

	writeTriggerFile(t, userDir, `triggers:
  - description: "user trigger"
    ruki: 'before update deny "user"'
`)
	writeTriggerFile(t, projectDir, `triggers:
  - description: "project trigger"
    ruki: 'before update deny "project"'
`)
	// no cwd workflow.yaml

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 def (project wins), got %d", len(defs))
	}
	if defs[0].Description != "project trigger" {
		t.Errorf("expected project trigger, got %q", defs[0].Description)
	}
}

func TestLoadTriggerDefs_UserFallback(t *testing.T) {
	userDir, _, _ := setupTriggerPrecedenceTest(t)

	writeTriggerFile(t, userDir, `triggers:
  - description: "user trigger"
    ruki: 'before update deny "user"'
`)
	// no project or cwd workflow.yaml

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

func TestLoadTriggerDefs_EmptyListOverridesInherited(t *testing.T) {
	userDir, projectDir, _ := setupTriggerPrecedenceTest(t)

	writeTriggerFile(t, userDir, `triggers:
  - description: "user trigger"
    ruki: 'before update deny "user"'
`)
	// project explicitly disables triggers with empty list
	writeTriggerFile(t, projectDir, "triggers: []\n")

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected 0 defs (empty list overrides user), got %d", len(defs))
	}
}

func TestLoadTriggerDefs_NoTriggersKeyDoesNotOverride(t *testing.T) {
	userDir, projectDir, _ := setupTriggerPrecedenceTest(t)

	writeTriggerFile(t, userDir, `triggers:
  - description: "user trigger"
    ruki: 'before update deny "user"'
`)
	// project has workflow.yaml but no triggers: key — should not override
	writeTriggerFile(t, projectDir, "views:\n  - name: board\n")

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 def (user preserved), got %d", len(defs))
	}
	if defs[0].Description != "user trigger" {
		t.Errorf("expected user trigger, got %q", defs[0].Description)
	}
}

func TestLoadTriggerDefs_NoWorkflowFiles(t *testing.T) {
	_, _, _ = setupTriggerPrecedenceTest(t)
	// no workflow.yaml files anywhere

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected 0 defs, got %d", len(defs))
	}
}

func TestLoadTriggerDefs_DeduplicatesAbsPath(t *testing.T) {
	// when project root == cwd, the project and cwd candidates resolve to the same file
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	sharedDir := t.TempDir()
	docDir := filepath.Join(sharedDir, ".doc")
	if err := os.MkdirAll(docDir, 0750); err != nil {
		t.Fatal(err)
	}

	originalDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	_ = os.Chdir(sharedDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = sharedDir

	// write workflow.yaml in cwd (== project root's parent of .doc — but workflow.yaml is at cwd level)
	writeTriggerFile(t, sharedDir, `triggers:
  - description: "shared trigger"
    ruki: 'before update deny "shared"'
`)

	defs, err := LoadTriggerDefs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// should find it once, not duplicated
	if len(defs) != 1 {
		t.Fatalf("expected 1 def (deduped), got %d", len(defs))
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
