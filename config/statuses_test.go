package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/workflow"
)

func defaultTestStatuses() []workflow.StatusDef {
	return []workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "in_progress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "review", Label: "Review", Emoji: "👀", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	}
}

func setupTestRegistry(t *testing.T, defs []workflow.StatusDef) {
	t.Helper()
	ResetStatusRegistry(defs)
	t.Cleanup(func() { ClearStatusRegistry() })
}

func TestBuildRegistry_DefaultStatuses(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	if len(reg.All()) != 5 {
		t.Fatalf("expected 5 statuses, got %d", len(reg.All()))
	}

	if reg.DefaultKey() != "backlog" {
		t.Errorf("expected default key 'backlog', got %q", reg.DefaultKey())
	}

	if reg.DoneKey() != "done" {
		t.Errorf("expected done key 'done', got %q", reg.DoneKey())
	}
}

func TestBuildRegistry_CustomStatuses(t *testing.T) {
	custom := []workflow.StatusDef{
		{Key: "new", Label: "New", Emoji: "🆕", Default: true},
		{Key: "wip", Label: "Work In Progress", Emoji: "🔧", Active: true},
		{Key: "closed", Label: "Closed", Emoji: "🔒", Done: true},
	}
	setupTestRegistry(t, custom)
	reg := GetStatusRegistry()

	if len(reg.All()) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(reg.All()))
	}
	if reg.DefaultKey() != "new" {
		t.Errorf("expected default key 'new', got %q", reg.DefaultKey())
	}
	if reg.DoneKey() != "closed" {
		t.Errorf("expected done key 'closed', got %q", reg.DoneKey())
	}
}

func TestRegistry_IsValid(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	tests := []struct {
		key  string
		want bool
	}{
		{"backlog", true},
		{"ready", true},
		{"in_progress", true},
		{"In-Progress", true}, // normalization
		{"review", true},
		{"done", true},
		{"unknown", false},
		{"", false},
		{"todo", false}, // no aliases
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := reg.IsValid(tt.key); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestRegistry_IsActive(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	tests := []struct {
		key  string
		want bool
	}{
		{"backlog", false},
		{"ready", true},
		{"in_progress", true},
		{"review", true},
		{"done", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := reg.IsActive(tt.key); got != tt.want {
				t.Errorf("IsActive(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestRegistry_Lookup(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	def, ok := reg.Lookup("ready")
	if !ok {
		t.Fatal("expected to find 'ready'")
	}
	if def.Label != "Ready" {
		t.Errorf("expected label 'Ready', got %q", def.Label)
	}
	if def.Emoji != "📋" {
		t.Errorf("expected emoji '📋', got %q", def.Emoji)
	}

	_, ok = reg.Lookup("nonexistent")
	if ok {
		t.Error("expected Lookup to return false for nonexistent key")
	}
}

func TestRegistry_Keys(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	keys := reg.Keys()
	expected := []workflow.StatusKey{"backlog", "ready", "in_progress", "review", "done"}

	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, key := range keys {
		if key != expected[i] {
			t.Errorf("keys[%d] = %q, want %q", i, key, expected[i])
		}
	}
}

func TestRegistry_NormalizesKeys(t *testing.T) {
	custom := []workflow.StatusDef{
		{Key: "In-Progress", Label: "In Progress", Default: true},
		{Key: "  DONE  ", Label: "Done", Done: true},
	}
	setupTestRegistry(t, custom)
	reg := GetStatusRegistry()

	if !reg.IsValid("in_progress") {
		t.Error("expected 'in_progress' to be valid after normalization")
	}
	if !reg.IsValid("done") {
		t.Error("expected 'done' to be valid after normalization")
	}
}

func TestBuildRegistry_EmptyKey(t *testing.T) {
	defs := []workflow.StatusDef{
		{Key: "", Label: "No Key"},
	}
	_, err := workflow.NewStatusRegistry(defs)
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestBuildRegistry_DuplicateKey(t *testing.T) {
	defs := []workflow.StatusDef{
		{Key: "ready", Label: "Ready", Default: true},
		{Key: "ready", Label: "Ready 2"},
	}
	_, err := workflow.NewStatusRegistry(defs)
	if err == nil {
		t.Error("expected error for duplicate key")
	}
}

func TestBuildRegistry_Empty(t *testing.T) {
	_, err := workflow.NewStatusRegistry(nil)
	if err == nil {
		t.Error("expected error for empty statuses")
	}
}

func TestBuildRegistry_DefaultFallsToFirst(t *testing.T) {
	defs := []workflow.StatusDef{
		{Key: "alpha", Label: "Alpha"},
		{Key: "beta", Label: "Beta"},
	}
	reg, err := workflow.NewStatusRegistry(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.DefaultKey() != "alpha" {
		t.Errorf("expected default to fall back to first status 'alpha', got %q", reg.DefaultKey())
	}
}

func TestNormalizeStatusKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"backlog", "backlog"},
		{"BACKLOG", "backlog"},
		{"In-Progress", "in_progress"},
		{"in progress", "in_progress"},
		{"  DONE  ", "done"},
		{"In_Review", "in_review"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := NormalizeStatusKey(tt.input); got != tt.want {
				t.Errorf("NormalizeStatusKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// writeTempWorkflow creates a temp workflow.yaml with the given content and returns its path.
func writeTempWorkflow(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing workflow file: %v", err)
	}
	return path
}

// setupLoadRegistryTest creates temp dirs and configures the path manager so
// LoadStatusRegistry can discover workflow.yaml files via FindWorkflowFiles.
func setupLoadRegistryTest(t *testing.T) (cwdDir string) {
	t.Helper()
	ClearStatusRegistry()
	t.Cleanup(func() { ResetStatusRegistry(defaultTestStatuses()) })

	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir()
	docDir := filepath.Join(projectDir, ".doc")
	if err := os.MkdirAll(docDir, 0750); err != nil {
		t.Fatal(err)
	}

	cwdDir = t.TempDir()
	originalDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	_ = os.Chdir(cwdDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = projectDir

	return cwdDir
}

func TestLoadStatusRegistry_HappyPath(t *testing.T) {
	cwdDir := setupLoadRegistryTest(t)

	content := `
statuses:
  - key: open
    label: Open
    emoji: "🔓"
    default: true
  - key: closed
    label: Closed
    emoji: "🔒"
    done: true
views:
  - name: board
    lanes:
      - name: Open
        filter: "status = 'open'"
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := LoadStatusRegistry(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reg := GetStatusRegistry()
	if reg.DefaultKey() != "open" {
		t.Errorf("expected default key 'open', got %q", reg.DefaultKey())
	}
	if reg.DoneKey() != "closed" {
		t.Errorf("expected done key 'closed', got %q", reg.DoneKey())
	}

	// type registry should also be initialized
	typeReg := GetTypeRegistry()
	if !typeReg.IsValid("story") {
		t.Error("expected type 'story' to be valid after LoadStatusRegistry")
	}
}

func TestLoadStatusRegistry_NoWorkflowFiles(t *testing.T) {
	_ = setupLoadRegistryTest(t)
	// no workflow.yaml files anywhere

	err := LoadStatusRegistry()
	if err == nil {
		t.Fatal("expected error when no workflow files found")
	}
}

func TestLoadStatusRegistry_NoStatusesDefined(t *testing.T) {
	cwdDir := setupLoadRegistryTest(t)

	// workflow.yaml exists with views but no statuses
	content := `
views:
  - name: board
    lanes:
      - name: All
        filter: "status = 'ready'"
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := LoadStatusRegistry()
	if err == nil {
		t.Fatal("expected error when no statuses defined")
	}
}

func TestLoadStatusRegistryFromFiles_LastFileWins(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	f1 := writeTempWorkflow(t, dir1, `
statuses:
  - key: alpha
    label: Alpha
    default: true
  - key: beta
    label: Beta
    done: true
`)
	f2 := writeTempWorkflow(t, dir2, `
statuses:
  - key: gamma
    label: Gamma
    default: true
  - key: delta
    label: Delta
    done: true
`)

	reg, path, err := loadStatusRegistryFromFiles([]string{f1, f2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}
	if path != f2 {
		t.Errorf("expected path %q, got %q", f2, path)
	}
	if reg.DefaultKey() != "gamma" {
		t.Errorf("expected default key 'gamma' from last file, got %q", reg.DefaultKey())
	}
	if len(reg.All()) != 2 {
		t.Errorf("expected 2 statuses from last file, got %d", len(reg.All()))
	}
}

func TestLoadStatusRegistryFromFiles_SkipsFileWithoutStatuses(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	f1 := writeTempWorkflow(t, dir1, `
statuses:
  - key: alpha
    label: Alpha
    default: true
  - key: beta
    label: Beta
    done: true
`)
	// second file has views but no statuses
	f2 := writeTempWorkflow(t, dir2, `
views:
  - name: backlog
`)

	reg, path, err := loadStatusRegistryFromFiles([]string{f1, f2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}
	if path != f1 {
		t.Errorf("expected path %q (first file with statuses), got %q", f1, path)
	}
	if reg.DefaultKey() != "alpha" {
		t.Errorf("expected default key 'alpha', got %q", reg.DefaultKey())
	}
}

func TestLoadStatusRegistryFromFiles_ParseErrorStopsEarly(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	f1 := writeTempWorkflow(t, dir1, `
statuses:
  - key: alpha
    label: Alpha
    default: true
`)
	f2 := writeTempWorkflow(t, dir2, `
statuses: [[[invalid yaml
`)

	_, _, err := loadStatusRegistryFromFiles([]string{f1, f2})
	if err == nil {
		t.Fatal("expected error for malformed YAML in second file")
	}
}

func TestRegistry_IsDone(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	if !reg.IsDone("done") {
		t.Error("expected 'done' to be marked as done")
	}
	if reg.IsDone("backlog") {
		t.Error("expected 'backlog' to not be marked as done")
	}
}

func TestGetTypeRegistry(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetTypeRegistry()

	if !reg.IsValid("story") {
		t.Error("expected 'story' to be valid")
	}
	if !reg.IsValid("bug") {
		t.Error("expected 'bug' to be valid")
	}
}

func TestMaybeGetTypeRegistry_Initialized(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())

	reg, ok := MaybeGetTypeRegistry()
	if !ok {
		t.Fatal("expected MaybeGetTypeRegistry to return true after init")
	}
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}
	if !reg.IsValid("story") {
		t.Error("expected 'story' to be valid")
	}
}

func TestMaybeGetTypeRegistry_Uninitialized(t *testing.T) {
	ClearStatusRegistry()
	t.Cleanup(func() {
		ResetStatusRegistry(defaultTestStatuses())
	})

	reg, ok := MaybeGetTypeRegistry()
	if ok {
		t.Error("expected MaybeGetTypeRegistry to return false when uninitialized")
	}
	if reg != nil {
		t.Error("expected nil registry when uninitialized")
	}
}

func TestGetStatusRegistry_PanicsWhenUninitialized(t *testing.T) {
	ClearStatusRegistry()
	t.Cleanup(func() {
		ResetStatusRegistry(defaultTestStatuses())
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from GetStatusRegistry when uninitialized")
		}
	}()
	GetStatusRegistry()
}

func TestGetTypeRegistry_PanicsWhenUninitialized(t *testing.T) {
	ClearStatusRegistry()
	t.Cleanup(func() {
		ResetStatusRegistry(defaultTestStatuses())
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from GetTypeRegistry when uninitialized")
		}
	}()
	GetTypeRegistry()
}

func TestLoadStatusRegistryFromFiles_NoStatuses(t *testing.T) {
	dir := t.TempDir()
	f := writeTempWorkflow(t, dir, `
views:
  - name: backlog
`)

	reg, path, err := loadStatusRegistryFromFiles([]string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg != nil {
		t.Error("expected nil registry when no statuses defined")
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
}

func TestLoadStatusRegistryFromFiles_ReadError(t *testing.T) {
	_, _, err := loadStatusRegistryFromFiles([]string{"/nonexistent/path/workflow.yaml"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadStatusesFromFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(f, []byte("{{{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadStatusesFromFile(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadStatusesFromFile_EmptyStatuses(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(f, []byte("statuses: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	reg, err := loadStatusesFromFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg != nil {
		t.Error("expected nil registry for empty statuses list")
	}
}

func TestLoadStatusRegistryFromFiles_AllFilesEmpty(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "workflow1.yaml")
	f2 := filepath.Join(dir, "workflow2.yaml")
	if err := os.WriteFile(f1, []byte("other_key: true\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.WriteFile(f2, []byte("statuses: []\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	reg, path, err := loadStatusRegistryFromFiles([]string{f1, f2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg != nil {
		t.Error("expected nil registry when all files have empty statuses")
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
}
