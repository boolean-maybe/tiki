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
			name:   "TaskDir",
			getter: pm.TaskDir,
		},
		{
			name:   "DokiDir",
			getter: pm.DokiDir,
		},
		{
			name:   "ProjectConfigFile",
			getter: pm.ProjectConfigFile,
		},
		{
			name:   "TemplateFile",
			getter: pm.TemplateFile,
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

	// Verify directories were created
	dirs := []string{
		pm.ConfigDir(),
		pm.CacheDir(),
		pm.TaskDir(),
		pm.DokiDir(),
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
		{"GetTaskDir", GetTaskDir},
		{"GetDokiDir", GetDokiDir},
		{"GetProjectConfigFile", GetProjectConfigFile},
		{"GetTemplateFile", GetTemplateFile},
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
	if GetTaskDir() == "" {
		t.Error("GetTaskDir() returned empty after InitPaths()")
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
