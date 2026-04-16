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
		{Key: "inProgress", Label: "In Progress", Emoji: "⚙️", Active: true},
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
		{"inProgress", true},
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
		{"inProgress", true},
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
		return
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
	expected := []workflow.StatusKey{"backlog", "ready", "inProgress", "review", "done"}

	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, key := range keys {
		if key != expected[i] {
			t.Errorf("keys[%d] = %q, want %q", i, key, expected[i])
		}
	}
}

func TestRegistry_RejectsNonCanonicalKeys(t *testing.T) {
	_, err := workflow.NewStatusRegistry([]workflow.StatusDef{
		{Key: "In-Progress", Label: "In Progress", Default: true},
		{Key: "done", Label: "Done", Done: true},
	})
	if err == nil {
		t.Fatal("expected error for non-canonical key")
	}
	if got := err.Error(); got != `status key "In-Progress" is not canonical; use "inProgress"` {
		t.Errorf("got error %q", got)
	}
}

func TestRegistry_CanonicalKeysWork(t *testing.T) {
	custom := []workflow.StatusDef{
		{Key: "inProgress", Label: "In Progress", Default: true},
		{Key: "done", Label: "Done", Done: true},
	}
	setupTestRegistry(t, custom)
	reg := GetStatusRegistry()

	if !reg.IsValid("inProgress") {
		t.Error("expected 'inProgress' to be valid")
	}
	if !reg.IsValid("done") {
		t.Error("expected 'done' to be valid")
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

func TestBuildRegistry_RequiresExplicitDefault(t *testing.T) {
	defs := []workflow.StatusDef{
		{Key: "alpha", Label: "Alpha", Done: true},
		{Key: "beta", Label: "Beta"},
	}
	_, err := workflow.NewStatusRegistry(defs)
	if err == nil {
		t.Fatal("expected error when no status is marked default")
	}
}

func TestBuildRegistry_RequiresExplicitDone(t *testing.T) {
	defs := []workflow.StatusDef{
		{Key: "alpha", Label: "Alpha", Default: true},
		{Key: "beta", Label: "Beta"},
	}
	_, err := workflow.NewStatusRegistry(defs)
	if err == nil {
		t.Fatal("expected error when no status is marked done")
	}
}

func TestBuildRegistry_RejectsDuplicateDefault(t *testing.T) {
	defs := []workflow.StatusDef{
		{Key: "alpha", Label: "Alpha", Default: true},
		{Key: "beta", Label: "Beta", Default: true, Done: true},
	}
	_, err := workflow.NewStatusRegistry(defs)
	if err == nil {
		t.Fatal("expected error for duplicate default")
	}
}

func TestBuildRegistry_RejectsDuplicateDone(t *testing.T) {
	defs := []workflow.StatusDef{
		{Key: "alpha", Label: "Alpha", Default: true, Done: true},
		{Key: "beta", Label: "Beta", Done: true},
	}
	_, err := workflow.NewStatusRegistry(defs)
	if err == nil {
		t.Fatal("expected error for duplicate done")
	}
}

func TestBuildRegistry_RejectsDuplicateDisplay(t *testing.T) {
	defs := []workflow.StatusDef{
		{Key: "alpha", Label: "Open", Emoji: "🟢", Default: true},
		{Key: "beta", Label: "Open", Emoji: "🟢", Done: true},
	}
	_, err := workflow.NewStatusRegistry(defs)
	if err == nil {
		t.Fatal("expected error for duplicate display")
	}
}

func TestBuildRegistry_LabelDefaultsToKey(t *testing.T) {
	defs := []workflow.StatusDef{
		{Key: "alpha", Emoji: "🔵", Default: true},
		{Key: "beta", Emoji: "🔴", Done: true},
	}
	reg, err := workflow.NewStatusRegistry(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	def, ok := reg.Lookup("alpha")
	if !ok {
		t.Fatal("expected to find alpha")
	}
	if def.Label != "alpha" {
		t.Errorf("expected label to default to key, got %q", def.Label)
	}
}

func TestBuildRegistry_EmptyWhitespaceLabel(t *testing.T) {
	defs := []workflow.StatusDef{
		{Key: "alpha", Label: "  ", Default: true},
		{Key: "beta", Done: true},
	}
	_, err := workflow.NewStatusRegistry(defs)
	if err == nil {
		t.Fatal("expected error for whitespace-only label")
	}
}

func TestNormalizeStatusKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"backlog", "backlog"},
		{"BACKLOG", "backlog"},
		{"In-Progress", "inProgress"},
		{"in progress", "inProgress"},
		{"  DONE  ", "done"},
		{"In_Review", "inReview"},
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
  - key: beta
    label: Beta
    done: true
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

func TestLoadStatusRegistry_MalformedFile(t *testing.T) {
	cwdDir := setupLoadRegistryTest(t)

	// write a workflow.yaml with invalid YAML so loadStatusRegistryFromFiles returns an error
	content := `statuses: [[[not valid yaml`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := LoadStatusRegistry()
	if err == nil {
		t.Fatal("expected error for malformed workflow.yaml")
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

// --- type loading tests ---

func TestLoadTypesFromFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	f := writeTempWorkflow(t, dir, `
types:
  - key: story
    label: Story
    emoji: "🌀"
  - key: bug
    label: Bug
    emoji: "💥"
`)
	reg, present, err := loadTypesFromFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !present {
		t.Fatal("expected present=true")
	}
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}
	if !reg.IsValid("story") {
		t.Error("expected story to be valid")
	}
	if !reg.IsValid("bug") {
		t.Error("expected bug to be valid")
	}
}

func TestLoadTypesFromFile_Absent(t *testing.T) {
	dir := t.TempDir()
	f := writeTempWorkflow(t, dir, `
statuses:
  - key: open
    label: Open
    default: true
  - key: done
    label: Done
    done: true
`)
	reg, present, err := loadTypesFromFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if present {
		t.Error("expected present=false when types: key is absent")
	}
	if reg != nil {
		t.Error("expected nil registry when absent")
	}
}

func TestLoadTypesFromFile_EmptyList(t *testing.T) {
	dir := t.TempDir()
	f := writeTempWorkflow(t, dir, `types: []`)
	_, present, err := loadTypesFromFile(f)
	if err == nil {
		t.Fatal("expected error for types: []")
	}
	if !present {
		t.Error("expected present=true even for empty list")
	}
}

func TestLoadTypesFromFile_NonCanonicalKey(t *testing.T) {
	dir := t.TempDir()
	f := writeTempWorkflow(t, dir, `
types:
  - key: Story
    label: Story
`)
	_, _, err := loadTypesFromFile(f)
	if err == nil {
		t.Fatal("expected error for non-canonical key")
	}
}

func TestLoadTypesFromFile_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	f := writeTempWorkflow(t, dir, `
types:
  - key: story
    label: Story
    aliases:
      - feature
`)
	_, _, err := loadTypesFromFile(f)
	if err == nil {
		t.Fatal("expected error for unknown key 'aliases'")
	}
}

func TestLoadTypesFromFile_MissingLabelDefaultsToKey(t *testing.T) {
	dir := t.TempDir()
	f := writeTempWorkflow(t, dir, `
types:
  - key: task
    emoji: "📋"
`)
	reg, _, err := loadTypesFromFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := reg.TypeLabel("task"); got != "task" {
		t.Errorf("expected label to default to key, got %q", got)
	}
}

func TestLoadTypesFromFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := writeTempWorkflow(t, dir, `types: [[[invalid`)
	_, _, err := loadTypesFromFile(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadTypeRegistryFromFiles_LastFileWithTypesWins(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	f1 := writeTempWorkflow(t, dir1, `
types:
  - key: alpha
    label: Alpha
`)
	f2 := writeTempWorkflow(t, dir2, `
types:
  - key: beta
    label: Beta
  - key: gamma
    label: Gamma
`)

	reg, path, err := loadTypeRegistryFromFiles([]string{f1, f2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != f2 {
		t.Errorf("expected path %q, got %q", f2, path)
	}
	if reg.IsValid("alpha") {
		t.Error("expected alpha to NOT be valid (overridden)")
	}
	if !reg.IsValid("beta") {
		t.Error("expected beta to be valid")
	}
}

func TestLoadTypeRegistryFromFiles_SkipsFilesWithoutTypes(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	f1 := writeTempWorkflow(t, dir1, `
types:
  - key: alpha
    label: Alpha
`)
	f2 := writeTempWorkflow(t, dir2, `
statuses:
  - key: open
    label: Open
    default: true
  - key: done
    label: Done
    done: true
`)

	reg, path, err := loadTypeRegistryFromFiles([]string{f1, f2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != f1 {
		t.Errorf("expected path %q (file with types), got %q", f1, path)
	}
	if !reg.IsValid("alpha") {
		t.Error("expected alpha to be valid")
	}
}

func TestLoadTypeRegistryFromFiles_FallbackToBuiltins(t *testing.T) {
	dir := t.TempDir()
	f := writeTempWorkflow(t, dir, `
statuses:
  - key: open
    label: Open
    default: true
  - key: done
    label: Done
    done: true
`)

	reg, path, err := loadTypeRegistryFromFiles([]string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "<built-in>" {
		t.Errorf("expected built-in path, got %q", path)
	}
	if !reg.IsValid("story") {
		t.Error("expected built-in story type")
	}
}

func TestLoadTypeRegistryFromFiles_ParseErrorStops(t *testing.T) {
	dir := t.TempDir()
	f := writeTempWorkflow(t, dir, `
types:
  - key: Story
    label: Story
`)
	_, _, err := loadTypeRegistryFromFiles([]string{f})
	if err == nil {
		t.Fatal("expected error for non-canonical key")
	}
}

func TestResetTypeRegistry(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())

	custom := []workflow.TypeDef{
		{Key: "task", Label: "Task"},
		{Key: "incident", Label: "Incident"},
	}
	ResetTypeRegistry(custom)

	reg := GetTypeRegistry()
	if !reg.IsValid("task") {
		t.Error("expected 'task' to be valid after ResetTypeRegistry")
	}
	if reg.IsValid("story") {
		t.Error("expected 'story' to NOT be valid after ResetTypeRegistry")
	}
}
