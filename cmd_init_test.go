package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// setupInitTest creates a temp git repo for init tests and sets XDG paths.
func setupInitTest(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()

	//nolint:gosec // G204: git init in test temp dir
	cmd := exec.Command("git", "init", repoDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	config.ResetPathManager()
	t.Cleanup(config.ResetPathManager)

	return repoDir
}

// --- parseInitArgs tests ---

func TestParseInitArgs_NoArgs(t *testing.T) {
	opts, err := parseInitArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cwd, _ := os.Getwd()
	absDir, _ := filepath.Abs(cwd)
	if opts.Directory != absDir {
		t.Errorf("directory = %q, want abs of cwd %q", opts.Directory, absDir)
	}
	if opts.WorkflowName != "" {
		t.Errorf("workflow = %q, want empty", opts.WorkflowName)
	}
}

func TestParseInitArgs_DirectoryOnly(t *testing.T) {
	dir := t.TempDir()
	opts, err := parseInitArgs([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, _ := filepath.Abs(dir)
	if opts.Directory != want {
		t.Errorf("directory = %q, want %q", opts.Directory, want)
	}
}

func TestParseInitArgs_BareDirectoryName(t *testing.T) {
	opts, err := parseInitArgs([]string{"my-repo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cwd, _ := os.Getwd()
	want := filepath.Join(cwd, "my-repo")
	if opts.Directory != want {
		t.Errorf("directory = %q, want %q", opts.Directory, want)
	}
}

func TestParseInitArgs_WorkflowShortFlag(t *testing.T) {
	opts, err := parseInitArgs([]string{"-w", "todo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.WorkflowName != "todo" {
		t.Errorf("workflow = %q, want %q", opts.WorkflowName, "todo")
	}
}

func TestParseInitArgs_WorkflowEqualsForm(t *testing.T) {
	opts, err := parseInitArgs([]string{"--workflow=kanban"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.WorkflowName != "kanban" {
		t.Errorf("workflow = %q, want %q", opts.WorkflowName, "kanban")
	}
}

func TestParseInitArgs_AISkillCommaSeparated(t *testing.T) {
	opts, err := parseInitArgs([]string{"--ai-skill", "claude,gemini"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.AISkills) != 2 || opts.AISkills[0] != "claude" || opts.AISkills[1] != "gemini" {
		t.Errorf("ai-skills = %v, want [claude gemini]", opts.AISkills)
	}
}

func TestParseInitArgs_AISkillEqualsForm(t *testing.T) {
	opts, err := parseInitArgs([]string{"--ai-skill=claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.AISkills) != 1 || opts.AISkills[0] != "claude" {
		t.Errorf("ai-skills = %v, want [claude]", opts.AISkills)
	}
}

func TestParseInitArgs_Samples(t *testing.T) {
	opts, err := parseInitArgs([]string{"--samples"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Samples {
		t.Error("samples should be true")
	}
}

func TestParseInitArgs_NonInteractiveShort(t *testing.T) {
	opts, err := parseInitArgs([]string{"-n"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.NonInteractive {
		t.Error("non-interactive should be true")
	}
}

func TestParseInitArgs_NonInteractiveLong(t *testing.T) {
	opts, err := parseInitArgs([]string{"--non-interactive"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.NonInteractive {
		t.Error("non-interactive should be true")
	}
}

func TestParseInitArgs_DirectoryAndWorkflowMixed(t *testing.T) {
	dir := t.TempDir()
	wantDir, _ := filepath.Abs(dir)

	// dir before -w
	opts, err := parseInitArgs([]string{dir, "-w", "todo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Directory != wantDir {
		t.Errorf("directory = %q, want %q", opts.Directory, wantDir)
	}
	if opts.WorkflowName != "todo" {
		t.Errorf("workflow = %q, want todo", opts.WorkflowName)
	}

	// -w before dir
	opts, err = parseInitArgs([]string{"-w", "kanban", dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Directory != wantDir {
		t.Errorf("directory = %q, want %q", opts.Directory, wantDir)
	}
	if opts.WorkflowName != "kanban" {
		t.Errorf("workflow = %q, want kanban", opts.WorkflowName)
	}
}

func TestParseInitArgs_Help(t *testing.T) {
	_, err := parseInitArgs([]string{"--help"})
	if !errors.Is(err, errHelpRequested) {
		t.Fatalf("expected errHelpRequested, got %v", err)
	}

	_, err = parseInitArgs([]string{"-h"})
	if !errors.Is(err, errHelpRequested) {
		t.Fatalf("expected errHelpRequested, got %v", err)
	}
}

func TestParseInitArgs_UnknownFlag(t *testing.T) {
	_, err := parseInitArgs([]string{"--verbose"})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestParseInitArgs_MultipleDirectories(t *testing.T) {
	_, err := parseInitArgs([]string{"dir1", "dir2"})
	if err == nil || !strings.Contains(err.Error(), "multiple directories") {
		t.Fatalf("expected multiple directories error, got %v", err)
	}
}

func TestParseInitArgs_WorkflowMissingValue(t *testing.T) {
	_, err := parseInitArgs([]string{"-w"})
	if err == nil || !strings.Contains(err.Error(), "requires a value") {
		t.Fatalf("expected requires value error, got %v", err)
	}
}

// --- validateInitOpts tests ---

func TestValidateInitOpts_CreatesNonExistentDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "newproject")

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	config.ResetPathManager()
	t.Cleanup(config.ResetPathManager)

	err := validateInitOpts(InitOpts{Directory: dir})
	if err != nil {
		t.Fatalf("expected directory to be created, got error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("created path is not a directory")
	}

	if !tikistore.IsGitRepo(dir) {
		t.Fatal("expected git repo to be initialized in new directory")
	}
}

func TestValidateInitOpts_UncreatableDirectory(t *testing.T) {
	// use a file as the parent — creating a subdirectory under a file always fails
	f := filepath.Join(t.TempDir(), "notadir.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := validateInitOpts(InitOpts{Directory: filepath.Join(f, "child")})
	if err == nil {
		t.Fatal("expected error for uncreatable directory")
	}
}

func TestValidateInitOpts_FileNotDirectory(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := validateInitOpts(InitOpts{Directory: f})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not-a-directory error, got %v", err)
	}
}

func TestValidateInitOpts_InitializesGitInExistingDir(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	config.ResetPathManager()
	t.Cleanup(config.ResetPathManager)

	err := validateInitOpts(InitOpts{Directory: dir})
	if err != nil {
		t.Fatalf("expected git init to succeed, got: %v", err)
	}

	if !tikistore.IsGitRepo(dir) {
		t.Fatal("expected directory to be a git repo after validation")
	}
}

func TestValidateInitOpts_UnknownAISkill(t *testing.T) {
	repoDir := setupInitTest(t)
	err := validateInitOpts(InitOpts{Directory: repoDir, AISkills: []string{"unknown-tool"}})
	if err == nil || !strings.Contains(err.Error(), "unknown AI skill") {
		t.Fatalf("expected unknown AI skill error, got %v", err)
	}
}

func TestValidateInitOpts_ValidAISkills(t *testing.T) {
	repoDir := setupInitTest(t)
	err := validateInitOpts(InitOpts{Directory: repoDir, AISkills: []string{"claude", "gemini"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateInitOpts_SamplesWithWorkflow(t *testing.T) {
	repoDir := setupInitTest(t)
	err := validateInitOpts(InitOpts{Directory: repoDir, Samples: true, WorkflowName: "todo"})
	if err == nil || !strings.Contains(err.Error(), "--samples cannot be used with --workflow") {
		t.Fatalf("expected samples+workflow error, got %v", err)
	}
}

func TestValidateInitOpts_ValidWorkflowName(t *testing.T) {
	repoDir := setupInitTest(t)
	err := validateInitOpts(InitOpts{Directory: repoDir, WorkflowName: "todo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateInitOpts_InvalidWorkflowName(t *testing.T) {
	repoDir := setupInitTest(t)
	err := validateInitOpts(InitOpts{Directory: repoDir, WorkflowName: "../escape"})
	if err == nil || !strings.Contains(err.Error(), "invalid workflow name") {
		t.Fatalf("expected invalid workflow name error, got %v", err)
	}
}

// --- runInit behavior tests ---

func TestRunInit_Help(t *testing.T) {
	if code := runInit([]string{"--help"}); code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
}

func TestRunInit_NonInteractiveDefaultNoSamples(t *testing.T) {
	repoDir := setupInitTest(t)

	code := runInit([]string{repoDir, "-n"})
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	// verify .doc/tiki exists (project initialized)
	tikiDir := filepath.Join(repoDir, ".doc", "tiki")
	if _, err := os.Stat(tikiDir); err != nil {
		t.Fatalf("expected .doc/tiki to exist: %v", err)
	}

	// with -n (no --samples), no sample tiki files should be created
	entries, _ := os.ReadDir(tikiDir)
	var tikiFiles int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "tiki-") && strings.HasSuffix(e.Name(), ".md") {
			tikiFiles++
		}
	}
	if tikiFiles != 0 {
		t.Errorf("expected 0 sample tikis, found %d", tikiFiles)
	}
}

func TestRunInit_NonInteractiveWithSamples(t *testing.T) {
	repoDir := setupInitTest(t)

	code := runInit([]string{repoDir, "-n", "--samples"})
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	// verify sample tikis were created
	tikiDir := filepath.Join(repoDir, ".doc", "tiki")
	entries, _ := os.ReadDir(tikiDir)
	var tikiFiles int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "tiki-") && strings.HasSuffix(e.Name(), ".md") {
			tikiFiles++
		}
	}
	if tikiFiles == 0 {
		t.Error("expected sample tikis to be created with --samples")
	}
}

func TestRunInit_NonInteractiveWithAISkills(t *testing.T) {
	repoDir := setupInitTest(t)

	code := runInit([]string{repoDir, "-n", "--ai-skill", "claude,gemini"})
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	// verify AI skill directories were created
	claudeSkill := filepath.Join(repoDir, ".claude", "skills", "tiki", "SKILL.md")
	if _, err := os.Stat(claudeSkill); err != nil {
		t.Errorf("expected claude tiki skill to exist: %v", err)
	}
	geminiSkill := filepath.Join(repoDir, ".gemini", "skills", "tiki", "SKILL.md")
	if _, err := os.Stat(geminiSkill); err != nil {
		t.Errorf("expected gemini tiki skill to exist: %v", err)
	}
}

func TestRunInit_AlreadyInitialized(t *testing.T) {
	repoDir := setupInitTest(t)

	// first init
	code := runInit([]string{repoDir, "-n"})
	if code != exitOK {
		t.Fatalf("first init: exit code = %d, want %d", code, exitOK)
	}

	// reset path manager so second runInit can re-init paths
	config.ResetPathManager()

	// second init on same repo
	code = runInit([]string{repoDir, "-n"})
	if code != exitOK {
		t.Fatalf("second init: exit code = %d, want %d", code, exitOK)
	}
}

func TestRunInit_InitializesGitRepo(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fresh")

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	config.ResetPathManager()
	t.Cleanup(config.ResetPathManager)

	code := runInit([]string{dir, "-n"})
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	tikiDir := filepath.Join(dir, ".doc", "tiki")
	if _, err := os.Stat(tikiDir); err != nil {
		t.Fatalf("expected .doc/tiki to exist: %v", err)
	}
}
