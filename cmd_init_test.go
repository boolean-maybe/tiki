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
	"github.com/boolean-maybe/tiki/internal/bootstrap"
	"github.com/boolean-maybe/tiki/store/tikistore"
)

// setupInitTest creates a temp git repo for init tests and sets XDG paths.
func setupInitTest(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()

	if err := tikistore.GitInit(repoDir); err != nil {
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

func TestEnsureDirectory_CreatesNonExistent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "newproject")

	if err := ensureDirectory(dir); err != nil {
		t.Fatalf("expected directory to be created, got error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("created path is not a directory")
	}
}

func TestEnsureDirectory_UncreatablePath(t *testing.T) {
	// file as parent: creating a subdirectory under a regular file always fails.
	f := filepath.Join(t.TempDir(), "notadir.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureDirectory(filepath.Join(f, "child")); err == nil {
		t.Fatal("expected error for uncreatable directory")
	}
}

func TestEnsureDirectory_FileNotDirectory(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	err := ensureDirectory(f)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not-a-directory error, got %v", err)
	}
}

func TestReconcileInitGit_InitializesGitInExistingDir(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	config.ResetPathManager()
	t.Cleanup(config.ResetPathManager)

	t.Chdir(dir)
	if _, err := bootstrap.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	if err := reconcileInitGit(); err != nil {
		t.Fatalf("expected git init to succeed, got: %v", err)
	}

	if !tikistore.IsGitRepo(dir) {
		t.Fatal("expected directory to be a git repo after reconcileInitGit")
	}
}

func TestValidateInitOpts_UnknownAISkill(t *testing.T) {
	repoDir := setupInitTest(t)
	err := validateInitOpts(&InitOpts{Directory: repoDir, AISkills: []string{"unknown-tool"}})
	if err == nil || !strings.Contains(err.Error(), "unknown AI skill") {
		t.Fatalf("expected unknown AI skill error, got %v", err)
	}
}

func TestValidateInitOpts_ValidAISkills(t *testing.T) {
	repoDir := setupInitTest(t)
	err := validateInitOpts(&InitOpts{Directory: repoDir, AISkills: []string{"claude", "gemini"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateInitOpts_SamplesWithWorkflow(t *testing.T) {
	repoDir := setupInitTest(t)
	err := validateInitOpts(&InitOpts{Directory: repoDir, Samples: true, WorkflowName: "todo"})
	if err == nil || !strings.Contains(err.Error(), "--samples cannot be used with --workflow") {
		t.Fatalf("expected samples+workflow error, got %v", err)
	}
}

func TestValidateInitOpts_ValidWorkflowName(t *testing.T) {
	repoDir := setupInitTest(t)
	err := validateInitOpts(&InitOpts{Directory: repoDir, WorkflowName: "todo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateInitOpts_InvalidWorkflowName(t *testing.T) {
	repoDir := setupInitTest(t)
	// "../escape" is classified as a file path (contains "/"), so the error is file-not-found
	err := validateInitOpts(&InitOpts{Directory: repoDir, WorkflowName: "../escape"})
	if err == nil {
		t.Fatal("expected error for invalid workflow path, got nil")
	}
}

func TestValidateInitOpts_WorkflowFileSource(t *testing.T) {
	repoDir := setupInitTest(t)

	// must be a valid workflow (statuses + types + views) to pass full validation
	validWorkflow := `version: 0.5.3
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
views:
  plugins:
    - name: Board
      default: true
      key: "F1"
      lanes:
        - name: Todo
          filter: select where status = "todo" order by priority
          action: update where id = id() set status="todo"
        - name: Done
          filter: select where status = "done" order by priority
          action: update where id = id() set status="done"
`
	tmp := filepath.Join(t.TempDir(), "custom.yaml")
	if err := os.WriteFile(tmp, []byte(validWorkflow), 0644); err != nil {
		t.Fatal(err)
	}

	opts := &InitOpts{Directory: repoDir, WorkflowName: tmp}
	if err := validateInitOpts(opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.WorkflowSource.Kind != config.WorkflowSourceFile {
		t.Errorf("source kind = %d, want file", opts.WorkflowSource.Kind)
	}
	if !filepath.IsAbs(opts.WorkflowSource.Name) {
		t.Errorf("source path %q is not absolute", opts.WorkflowSource.Name)
	}
	if opts.WorkflowContent == "" {
		t.Error("expected pre-fetched workflow content")
	}
}

func TestValidateInitOpts_InvalidWorkflowFileContent(t *testing.T) {
	repoDir := setupInitTest(t)

	tmp := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(tmp, []byte("description: bad workflow\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := &InitOpts{Directory: repoDir, WorkflowName: tmp}
	err := validateInitOpts(opts)
	if err == nil {
		t.Fatal("expected error for invalid workflow file content, got nil")
	}
	if !strings.Contains(err.Error(), "invalid workflow") {
		t.Errorf("error = %q, want to contain 'invalid workflow'", err.Error())
	}
}

func TestValidateInitOpts_WorkflowURLSource(t *testing.T) {
	repoDir := setupInitTest(t)

	// serve valid workflow via test server
	todoContent, _ := config.LookupEmbeddedWorkflow("todo")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(todoContent))
	}))
	defer server.Close()

	opts := &InitOpts{Directory: repoDir, WorkflowName: server.URL + "/workflow.yaml"}
	if err := validateInitOpts(opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.WorkflowSource.Kind != config.WorkflowSourceURL {
		t.Errorf("source kind = %d, want URL", opts.WorkflowSource.Kind)
	}
	if opts.WorkflowContent == "" {
		t.Error("expected pre-fetched workflow content")
	}
}

func TestValidateInitOpts_WorkflowStoresResolvedSource(t *testing.T) {
	repoDir := setupInitTest(t)

	opts := &InitOpts{Directory: repoDir, WorkflowName: "todo"}
	if err := validateInitOpts(opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.WorkflowSource.Kind != config.WorkflowSourceEmbedded {
		t.Errorf("source kind = %d, want embedded", opts.WorkflowSource.Kind)
	}
	if opts.WorkflowSource.Name != "todo" {
		t.Errorf("source name = %q, want %q", opts.WorkflowSource.Name, "todo")
	}
}

func TestValidateInitOpts_WorkflowDirectoryRejected(t *testing.T) {
	repoDir := setupInitTest(t)
	dir := t.TempDir()

	opts := &InitOpts{Directory: repoDir, WorkflowName: dir}
	err := validateInitOpts(opts)
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected not-a-regular-file error, got %v", err)
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

	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		t.Fatalf("expected .git to exist: %v", err)
	}
}

// setupRunInitTest prepares a hermetic environment for runInit tests: isolated
// XDG dirs, fresh path manager, and a restore-cwd cleanup so runInit's chdir
// never leaks between tests. Mirrors setupDemoTest but does not chdir anywhere
// (runInit chdirs into the target itself).
func setupRunInitTest(t *testing.T) {
	t.Helper()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	config.ResetPathManager()
	t.Cleanup(config.ResetPathManager)
}

func TestRunInit_NoGitWhenDisabled_FreshDir(t *testing.T) {
	setupRunInitTest(t)
	t.Setenv("TIKI_STORE_GIT", "false")

	dir := filepath.Join(t.TempDir(), "fresh")

	if code := runInit([]string{dir, "-n"}); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	if _, err := os.Stat(filepath.Join(dir, ".doc", "tiki")); err != nil {
		t.Fatalf("expected .doc/tiki: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		t.Error(".git should not exist with TIKI_STORE_GIT=false")
	}
}

func TestRunInit_NoGitWhenDisabled_ExistingEmptyDir(t *testing.T) {
	setupRunInitTest(t)
	t.Setenv("TIKI_STORE_GIT", "false")

	dir := t.TempDir() // already exists, empty

	if code := runInit([]string{dir, "-n"}); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	if _, err := os.Stat(filepath.Join(dir, ".doc", "tiki")); err != nil {
		t.Fatalf("expected .doc/tiki: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		t.Error(".git should not exist with TIKI_STORE_GIT=false")
	}
}

// TestRunInit_IsolatedFromParentRepo_FlagOff verifies that when store.git is
// off and the parent directory is a git repo, runInit neither creates a
// repo under the target nor touches the parent's index.
func TestRunInit_IsolatedFromParentRepo_FlagOff(t *testing.T) {
	setupRunInitTest(t)
	t.Setenv("TIKI_STORE_GIT", "false")

	parent := t.TempDir()
	if err := tikistore.GitInit(parent); err != nil {
		t.Fatalf("git init parent: %v", err)
	}
	sub := filepath.Join(parent, "sub")

	if code := runInit([]string{sub, "-n"}); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	if _, err := os.Stat(filepath.Join(sub, ".git")); err == nil {
		t.Error("sub/.git should not exist with TIKI_STORE_GIT=false")
	}
	if _, err := os.Stat(filepath.Join(parent, ".git", "index")); err == nil {
		t.Error("parent .git/index created — init leaked into parent repo")
	}
}

// TestRunInit_IsolatedFromParentRepo_FlagOn is the regression guard for the
// os.Stat(".git") vs tikistore.IsGitRepo design decision. With IsGitRepo
// (ancestor walk), init would attach to the parent repo and sub/.git would
// be absent. With os.Stat(".git") (local check), it correctly creates its own.
func TestRunInit_IsolatedFromParentRepo_FlagOn(t *testing.T) {
	setupRunInitTest(t)
	t.Setenv("TIKI_STORE_GIT", "true")

	parent := t.TempDir()
	if err := tikistore.GitInit(parent); err != nil {
		t.Fatalf("git init parent: %v", err)
	}
	sub := filepath.Join(parent, "sub")

	if code := runInit([]string{sub, "-n"}); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	// resolve symlinks on macOS (/var vs /private/var).
	subResolved := resolvePath(t, sub)
	if _, err := os.Stat(filepath.Join(subResolved, ".git")); err != nil {
		t.Fatalf("expected sub/.git to exist: %v", err)
	}

	// belt and braces: parent repo's index must remain empty (a fresh git
	// init never creates .git/index until something is staged).
	if _, err := os.Stat(filepath.Join(parent, ".git", "index")); err == nil {
		t.Error("parent .git/index created — init leaked into parent repo")
	}
}

func TestRunInit_PreservesExistingGitWhenFlagOff(t *testing.T) {
	setupRunInitTest(t)
	t.Setenv("TIKI_STORE_GIT", "false")

	dir := t.TempDir()
	if err := tikistore.GitInit(dir); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// snapshot HEAD so we can assert it didn't change
	headBefore, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD before: %v", err)
	}

	if code := runInit([]string{dir, "-n"}); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf(".git disappeared: %v", err)
	}
	headAfter, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD after: %v", err)
	}
	if string(headBefore) != string(headAfter) {
		t.Errorf("HEAD changed: before=%q after=%q", headBefore, headAfter)
	}
}

// TestRunInit_ReconcilesGitOnSecondRun verifies the Option A design:
// reconcileInitGit runs before the IsProjectInitialized short-circuit, so
// flipping TIKI_STORE_GIT between runs reconciles git state even when the
// project is already initialized. Mirrors TestRunDemo_ReconcilesGitOnSecondRun.
func TestRunInit_ReconcilesGitOnSecondRun(t *testing.T) {
	setupRunInitTest(t)

	dir := t.TempDir()

	// first run: no git
	t.Setenv("TIKI_STORE_GIT", "false")
	if code := runInit([]string{dir, "-n"}); code != exitOK {
		t.Fatalf("first runInit: exit code = %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		t.Fatal(".git should not exist after first run")
	}

	// chdir back so the second call can parseInitArgs relative to a known cwd
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	if err := os.Chdir(filepath.Dir(dir)); err != nil {
		t.Fatalf("chdir parent: %v", err)
	}

	// second run: enable git. project is already initialized, so runInit
	// prints "project already initialized" after reconcileInitGit creates .git.
	t.Setenv("TIKI_STORE_GIT", "true")
	config.ResetPathManager()
	if code := runInit([]string{dir, "-n"}); code != exitOK {
		t.Fatalf("second runInit: exit code = %d", code)
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("expected .git after second run: %v", err)
	}
}
