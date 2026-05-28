package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetUserConfigDir(t *testing.T) {
	tests := []struct {
		name        string
		xdgConfig   string
		goos        string
		expectXDG   bool
		expectMacOS bool
	}{
		{
			name:      "XDG_CONFIG_HOME set",
			xdgConfig: "/custom/config",
			expectXDG: true,
		},
		{
			name:        "macOS without XDG",
			xdgConfig:   "",
			goos:        "darwin",
			expectMacOS: true,
		},
		{
			name:      "Linux without XDG",
			xdgConfig: "",
			goos:      "linux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment
			origXDG := os.Getenv("XDG_CONFIG_HOME")
			defer func() {
				if origXDG != "" {
					_ = os.Setenv("XDG_CONFIG_HOME", origXDG)
				} else {
					_ = os.Unsetenv("XDG_CONFIG_HOME")
				}
			}()

			if tt.xdgConfig != "" {
				_ = os.Setenv("XDG_CONFIG_HOME", tt.xdgConfig)
			} else {
				_ = os.Unsetenv("XDG_CONFIG_HOME")
			}

			dir, err := getUserConfigDir()
			if err != nil {
				t.Fatalf("getUserConfigDir() error = %v", err)
			}

			if tt.expectXDG {
				expected := filepath.Join(tt.xdgConfig, "tiki")
				if dir != expected {
					t.Errorf("getUserConfigDir() = %q, want %q", dir, expected)
				}
			} else if tt.expectMacOS && runtime.GOOS == "darwin" {
				// On macOS, should contain "Library/Application Support/tiki" or ".config/tiki"
				if !filepath.IsAbs(dir) {
					t.Errorf("getUserConfigDir() returned non-absolute path: %q", dir)
				}
				if filepath.Base(dir) != "tiki" {
					t.Errorf("getUserConfigDir() = %q, want basename 'tiki'", dir)
				}
			} else {
				// Should be absolute and end with /tiki
				if !filepath.IsAbs(dir) {
					t.Errorf("getUserConfigDir() returned non-absolute path: %q", dir)
				}
				if filepath.Base(dir) != "tiki" {
					t.Errorf("getUserConfigDir() = %q, want basename 'tiki'", dir)
				}
			}
		})
	}
}

func TestGetUserCacheDir(t *testing.T) {
	tests := []struct {
		name      string
		xdgCache  string
		expectXDG bool
	}{
		{
			name:      "XDG_CACHE_HOME set",
			xdgCache:  "/custom/cache",
			expectXDG: true,
		},
		{
			name:     "without XDG",
			xdgCache: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment
			origXDG := os.Getenv("XDG_CACHE_HOME")
			defer func() {
				if origXDG != "" {
					_ = os.Setenv("XDG_CACHE_HOME", origXDG)
				} else {
					_ = os.Unsetenv("XDG_CACHE_HOME")
				}
			}()

			if tt.xdgCache != "" {
				_ = os.Setenv("XDG_CACHE_HOME", tt.xdgCache)
			} else {
				_ = os.Unsetenv("XDG_CACHE_HOME")
			}

			dir, err := getUserCacheDir()
			if err != nil {
				t.Fatalf("getUserCacheDir() error = %v", err)
			}

			if tt.expectXDG {
				expected := filepath.Join(tt.xdgCache, "tiki")
				if dir != expected {
					t.Errorf("getUserCacheDir() = %q, want %q", dir, expected)
				}
			} else {
				// Should be absolute and end with /tiki
				if !filepath.IsAbs(dir) {
					t.Errorf("getUserCacheDir() returned non-absolute path: %q", dir)
				}
				if filepath.Base(dir) != "tiki" {
					t.Errorf("getUserCacheDir() = %q, want basename 'tiki'", dir)
				}
			}
		})
	}
}

func TestGetProjectRoot(t *testing.T) {
	root, err := getProjectRoot()
	if err != nil {
		t.Fatalf("getProjectRoot() error = %v", err)
	}

	if !filepath.IsAbs(root) {
		t.Errorf("getProjectRoot() = %q, want absolute path", root)
	}

	// Verify the directory exists
	if _, err := os.Stat(root); err != nil {
		t.Errorf("getProjectRoot() returned path that doesn't exist: %v", err)
	}
}

func TestPathManagerPaths(t *testing.T) {
	pm, err := newPathManager()
	if err != nil {
		t.Fatalf("newPathManager() error = %v", err)
	}

	tests := []struct {
		name   string
		getter func() string
		want   string
	}{
		{
			name:   "ConfigDir",
			getter: pm.ConfigDir,
		},
		{
			name:   "CacheDir",
			getter: pm.CacheDir,
		},
		{
			name:   "ConfigFile",
			getter: pm.ConfigFile,
		},
		{
			name:   "ProjectConfigFile",
			getter: pm.ProjectConfigFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.getter()
			if result == "" {
				t.Errorf("%s() returned empty string", tt.name)
			}
			if !filepath.IsAbs(result) {
				t.Errorf("%s() = %q, want absolute path", tt.name, result)
			}
		})
	}
}

func TestPathManagerPluginSearchPaths(t *testing.T) {
	pm, err := newPathManager()
	if err != nil {
		t.Fatalf("newPathManager() error = %v", err)
	}

	paths := pm.PluginSearchPaths()
	if len(paths) != 2 {
		t.Errorf("PluginSearchPaths() returned %d paths, want 2", len(paths))
	}

	// First should be project config dir (.doc/)
	if paths[0] != pm.ProjectConfigDir() {
		t.Errorf("PluginSearchPaths()[0] = %q, want %q", paths[0], pm.ProjectConfigDir())
	}

	// Second should be user config dir
	if paths[1] != pm.ConfigDir() {
		t.Errorf("PluginSearchPaths()[1] = %q, want %q", paths[1], pm.ConfigDir())
	}

	// All paths should be absolute
	for i, path := range paths {
		if !filepath.IsAbs(path) {
			t.Errorf("PluginSearchPaths()[%d] = %q, want absolute path", i, path)
		}
	}
}

func TestPathManagerEnsureDirs(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a PathManager with temporary paths
	pm := &PathManager{
		configDir:   filepath.Join(tmpDir, "config"),
		cacheDir:    filepath.Join(tmpDir, "cache"),
		projectRoot: tmpDir,
	}

	// Call EnsureDirs
	if err := pm.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error = %v", err)
	}

	// Phase 8: EnsureDirs creates the unified .doc/ root but no longer
	// provisions any legacy subdirectories.
	dirs := []string{
		pm.ConfigDir(),
		pm.CacheDir(),
		pm.DocDir(),
	}

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %q was not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
		// Check permissions (should be 0755) - skip on Windows as it uses ACL-based permissions
		if runtime.GOOS != "windows" {
			if info.Mode().Perm() != 0755 {
				t.Errorf("directory %q has permissions %o, want 0755", dir, info.Mode().Perm())
			}
		}
	}
}

func TestGlobalAccessorFunctions(t *testing.T) {
	// Test that all global accessor functions return non-empty absolute paths
	tests := []struct {
		name   string
		getter func() string
	}{
		{"GetConfigDir", GetConfigDir},
		{"GetCacheDir", GetCacheDir},
		{"GetConfigFile", GetConfigFile},
		{"GetProjectConfigFile", GetProjectConfigFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.getter()
			if result == "" {
				t.Errorf("%s() returned empty string", tt.name)
			}
			if !filepath.IsAbs(result) {
				t.Errorf("%s() = %q, want absolute path", tt.name, result)
			}
		})
	}
}

func TestGetPluginSearchPaths(t *testing.T) {
	paths := GetPluginSearchPaths()
	if len(paths) != 2 {
		t.Errorf("GetPluginSearchPaths() returned %d paths, want 2", len(paths))
	}

	for i, path := range paths {
		if path == "" {
			t.Errorf("GetPluginSearchPaths()[%d] is empty", i)
		}
		if !filepath.IsAbs(path) {
			t.Errorf("GetPluginSearchPaths()[%d] = %q, want absolute path", i, path)
		}
	}
}

func TestInitPaths(t *testing.T) {
	// Reset to test initialization
	ResetPathManager()
	defer ResetPathManager() // Clean up after test

	err := InitPaths()
	if err != nil {
		t.Fatalf("InitPaths() error = %v", err)
	}

	// After InitPaths, all accessors should work
	if GetConfigDir() == "" {
		t.Error("GetConfigDir() returned empty after InitPaths()")
	}
}

func TestResetPathManager(t *testing.T) {
	// Save original XDG
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if origXDG != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", origXDG)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
		ResetPathManager() // Clean up
	}()

	// First initialization
	ResetPathManager()
	_ = os.Setenv("XDG_CONFIG_HOME", "/first/config")
	if err := InitPaths(); err != nil {
		t.Fatalf("first InitPaths() error = %v", err)
	}
	first := GetConfigDir()
	expected1 := filepath.Join("/first/config", "tiki")
	if first != expected1 {
		t.Errorf("first GetConfigDir() = %q, want %q", first, expected1)
	}

	// Reset and reinitialize with different env
	ResetPathManager()
	_ = os.Setenv("XDG_CONFIG_HOME", "/second/config")
	if err := InitPaths(); err != nil {
		t.Fatalf("second InitPaths() error = %v", err)
	}
	second := GetConfigDir()
	expected2 := filepath.Join("/second/config", "tiki")
	if second != expected2 {
		t.Errorf("second GetConfigDir() = %q, want %q", second, expected2)
	}

	// Verify they're different (reset worked)
	if first == second {
		t.Error("ResetPathManager() did not allow re-initialization with different config")
	}
}

func TestFindWorkflowFileWithScope_CwdWins(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	userTikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(userTikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userTikiDir, "workflow.yaml"), []byte("version: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	cwdDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte("version: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = t.TempDir() // empty project dir

	path, scope := FindWorkflowFileWithScope()
	if scope != ScopeCurrent {
		t.Errorf("scope = %q, want %q", scope, ScopeCurrent)
	}
	pathAbs, _ := filepath.Abs(path)
	wantAbs, _ := filepath.Abs("workflow.yaml")
	if pathAbs != wantAbs {
		t.Errorf("path = %q, want cwd file %q", pathAbs, wantAbs)
	}
}

func TestFindWorkflowFileWithScope_ProjectWins(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	userTikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(userTikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userTikiDir, "workflow.yaml"), []byte("version: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir()
	docDir := filepath.Join(projectDir, ".doc")
	if err := os.MkdirAll(docDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docDir, "workflow.yaml"), []byte("version: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	cwdDir := t.TempDir() // no workflow.yaml here
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = projectDir

	path, scope := FindWorkflowFileWithScope()
	if scope != ScopeLocal {
		t.Errorf("scope = %q, want %q", scope, ScopeLocal)
	}
	want := filepath.Join(docDir, "workflow.yaml")
	if path != want {
		t.Errorf("path = %q, want project file %q", path, want)
	}
}

func TestFindWorkflowFileWithScope_GlobalFallback(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	userTikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(userTikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userTikiDir, "workflow.yaml"), []byte("version: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	cwdDir := t.TempDir() // no workflow.yaml
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = t.TempDir() // empty project dir

	path, scope := FindWorkflowFileWithScope()
	if scope != ScopeGlobal {
		t.Errorf("scope = %q, want %q", scope, ScopeGlobal)
	}
	want := filepath.Join(userTikiDir, "workflow.yaml")
	if path != want {
		t.Errorf("path = %q, want global file %q", path, want)
	}
}

func TestFindWorkflowFileWithScope_NoneFound(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	cwdDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = t.TempDir()

	path, scope := FindWorkflowFileWithScope()
	if path != "" {
		t.Errorf("path = %q, want empty", path)
	}
	if scope != "" {
		t.Errorf("scope = %q, want empty (zero value)", scope)
	}
}

func TestFindWorkflowFileWithScope_DedupCwdEqualsProjectDir(t *testing.T) {
	// when cwd == ProjectConfigDir, candidates 2 and 3 resolve to the same
	// absolute path. The project-dir candidate should win (ScopeLocal) because
	// it appears first and dedup skips the cwd candidate.
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	// resolve symlinks so both paths are canonical (macOS /var → /private/var)
	projectDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	docDir := filepath.Join(projectDir, ".doc")
	if err := os.MkdirAll(docDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docDir, "workflow.yaml"), []byte("version: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	// cd into .doc/ so cwd candidate resolves to the same file
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(docDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = projectDir

	_, scope := FindWorkflowFileWithScope()
	if scope != ScopeLocal {
		t.Errorf("scope = %q, want %q (project-dir candidate should win dedup)", scope, ScopeLocal)
	}
}

func TestWorkflowScopeLabel(t *testing.T) {
	tests := []struct {
		scope Scope
		want  string
	}{
		{ScopeGlobal, "global"},
		{ScopeLocal, "project"},
		{ScopeCurrent, "local"},
		{Scope("unknown"), "unknown"},
	}
	for _, tt := range tests {
		if got := WorkflowScopeLabel(tt.scope); got != tt.want {
			t.Errorf("WorkflowScopeLabel(%q) = %q, want %q", tt.scope, got, tt.want)
		}
	}
}

func TestValidateDocDir(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid .doc", ".doc", false},
		{"valid docs", "docs", false},
		{"valid .tiki", ".tiki", false},
		{"valid nested", "my-docs/store", false},
		{"empty", "", true},
		{"absolute", "/tmp/docs", true},
		{"dotdot simple", "..", true},
		{"dotdot nested", "foo/../bar", true},
		{"dotdot prefix", "../docs", true},
		{"resolves to root dot", ".", true},
		{"resolves to root via trailing slash", "./", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDocDir(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDocDir(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestResolveUserConfiguredDocDir_Missing(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveUserConfiguredDocDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveUserConfiguredDocDir_ValidValue(t *testing.T) {
	dir := t.TempDir()
	content := []byte("store:\n  dir: docs\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveUserConfiguredDocDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "docs" {
		t.Errorf("expected 'docs', got %q", got)
	}
}

func TestResolveUserConfiguredDocDir_Unset(t *testing.T) {
	dir := t.TempDir()
	content := []byte("logging:\n  level: debug\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveUserConfiguredDocDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveUserConfiguredDocDir_InvalidValue(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{"absolute", "/tmp/docs"},
		{"dotdot", "../docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			content := []byte("store:\n  dir: " + tt.dir + "\n")
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
				t.Fatal(err)
			}
			_, err := resolveUserConfiguredDocDir(dir)
			if err == nil {
				t.Errorf("expected error for dir=%q, got nil", tt.dir)
			}
		})
	}
}

func TestResolveUserConfiguredDocDir_EmptyTreatedAsUnset(t *testing.T) {
	dir := t.TempDir()
	content := []byte("store:\n  dir:\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveUserConfiguredDocDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty (unset), got %q", got)
	}
}

func TestResolveUserConfiguredDocDir_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	content := []byte("store: [\n  invalid yaml\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveUserConfiguredDocDir(dir)
	if err != nil {
		t.Fatalf("unexpected error (malformed YAML should be treated as unset): %v", err)
	}
	if got != "" {
		t.Errorf("expected empty for malformed YAML, got %q", got)
	}
}

func TestDocDirName_DefaultIsDotDoc(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	ResetPathManager()
	defer ResetPathManager()

	if err := InitPaths(); err != nil {
		t.Fatalf("InitPaths: %v", err)
	}
	if got := GetDocDirName(); got != ".doc" {
		t.Errorf("GetDocDirName() = %q, want '.doc'", got)
	}
}

func TestDocDirName_UserConfigOverride(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	tikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(tikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	content := []byte("store:\n  dir: docs\n")
	if err := os.WriteFile(filepath.Join(tikiDir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}

	ResetPathManager()
	defer ResetPathManager()

	if err := InitPaths(); err != nil {
		t.Fatalf("InitPaths: %v", err)
	}
	if got := GetDocDirName(); got != "docs" {
		t.Errorf("GetDocDirName() = %q, want 'docs'", got)
	}
	pm := mustGetPathManager()
	wantSuffix := filepath.Join(pm.projectRoot, "docs")
	if got := GetDocDir(); got != wantSuffix {
		t.Errorf("GetDocDir() = %q, want %q", got, wantSuffix)
	}
}

func TestDocDirName_InvalidUserConfigFails(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	tikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(tikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	content := []byte("store:\n  dir: /absolute/bad\n")
	if err := os.WriteFile(filepath.Join(tikiDir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}

	ResetPathManager()
	defer ResetPathManager()

	err := InitPaths()
	if err == nil {
		t.Fatal("expected InitPaths to fail with invalid store.dir")
	}
}

func TestDocDirName_DemoOverride(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	tikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(tikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	content := []byte("store:\n  dir: docs\n")
	if err := os.WriteFile(filepath.Join(tikiDir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}

	ResetPathManager()
	defer ResetPathManager()

	SetDemoDocDirOverride(".doc")
	if err := InitPaths(); err != nil {
		t.Fatalf("InitPaths: %v", err)
	}
	if got := GetDocDirName(); got != ".doc" {
		t.Errorf("GetDocDirName() = %q, want '.doc' (demo override)", got)
	}
}

func TestFindWorkflowFile_DelegatesToWithScope(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	userTikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(userTikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userTikiDir, "workflow.yaml"), []byte("version: 1"), 0644); err != nil {
		t.Fatal(err)
	}

	cwdDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = t.TempDir()

	path := FindWorkflowFile()
	pathWithScope, _ := FindWorkflowFileWithScope()
	if path != pathWithScope {
		t.Errorf("FindWorkflowFile() = %q, FindWorkflowFileWithScope() = %q — should match", path, pathWithScope)
	}

	files := FindWorkflowFiles()
	if len(files) != 1 || files[0] != path {
		t.Errorf("FindWorkflowFiles() = %v, want [%q]", files, path)
	}
}
