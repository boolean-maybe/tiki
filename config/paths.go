package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	// ErrNoHome indicates that the user's home directory could not be determined
	ErrNoHome = errors.New("unable to determine home directory")

	// ErrPathManagerInit indicates that the PathManager failed to initialize
	ErrPathManagerInit = errors.New("failed to initialize path manager")
)

// PathManager manages all file system paths for tiki
type PathManager struct {
	configDir   string // User config directory
	cacheDir    string // User cache directory
	projectRoot string // Current working directory
}

// newPathManager creates and initializes a new PathManager
func newPathManager() (*PathManager, error) {
	configDir, err := getUserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("get config directory: %w", err)
	}

	cacheDir, err := getUserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("get cache directory: %w", err)
	}

	projectRoot, err := getProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("get project root: %w", err)
	}

	return &PathManager{
		configDir:   configDir,
		cacheDir:    cacheDir,
		projectRoot: projectRoot,
	}, nil
}

// getUserConfigDir returns the platform-appropriate user config directory
func getUserConfigDir() (string, error) {
	// Check XDG_CONFIG_HOME first (works on all platforms)
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "tiki"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoHome
	}

	switch runtime.GOOS {
	case "darwin":
		// macOS: prefer ~/.config/tiki if it exists, else ~/Library/Application Support/tiki
		// Note: We only check for existence here; directory creation happens in EnsureDirs()
		tikiConfigDir := filepath.Join(homeDir, ".config", "tiki")

		// If ~/.config/tiki already exists, use it
		if info, err := os.Stat(tikiConfigDir); err == nil && info.IsDir() {
			return tikiConfigDir, nil
		}

		// If ~/.config exists (even without tiki subdir), prefer XDG-style
		dotConfigDir := filepath.Join(homeDir, ".config")
		if info, err := os.Stat(dotConfigDir); err == nil && info.IsDir() {
			return tikiConfigDir, nil
		}

		// Fall back to macOS native location
		return filepath.Join(homeDir, "Library", "Application Support", "tiki"), nil

	case "windows":
		// Windows: %APPDATA%\tiki
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "tiki"), nil
		}
		return filepath.Join(homeDir, "AppData", "Roaming", "tiki"), nil

	default:
		// Linux and other Unix-like: ~/.config/tiki
		return filepath.Join(homeDir, ".config", "tiki"), nil
	}
}

// getUserCacheDir returns the platform-appropriate user cache directory
func getUserCacheDir() (string, error) {
	// Check XDG_CACHE_HOME first (works on all platforms)
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "tiki"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoHome
	}

	switch runtime.GOOS {
	case "darwin":
		// macOS: ~/Library/Caches/tiki
		return filepath.Join(homeDir, "Library", "Caches", "tiki"), nil

	case "windows":
		// Windows: %LOCALAPPDATA%\tiki
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "tiki"), nil
		}
		return filepath.Join(homeDir, "AppData", "Local", "tiki"), nil

	default:
		// Linux and other Unix-like: ~/.cache/tiki
		return filepath.Join(homeDir, ".cache", "tiki"), nil
	}
}

// getProjectRoot returns the current working directory
func getProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get current directory: %w", err)
	}
	return cwd, nil
}

// ConfigDir returns the user config directory
func (pm *PathManager) ConfigDir() string {
	return pm.configDir
}

// CacheDir returns the user cache directory
func (pm *PathManager) CacheDir() string {
	return pm.cacheDir
}

// ConfigFile returns the path to the user config file
func (pm *PathManager) ConfigFile() string {
	return filepath.Join(pm.configDir, "config.yaml")
}

// TaskDir returns the project-local task directory
func (pm *PathManager) TaskDir() string {
	return filepath.Join(pm.projectRoot, ".doc", "tiki")
}

// DokiDir returns the project-local documentation directory
func (pm *PathManager) DokiDir() string {
	return filepath.Join(pm.projectRoot, ".doc", "doki")
}

// ProjectConfigFile returns the path to the project-local config file
func (pm *PathManager) ProjectConfigFile() string {
	return filepath.Join(pm.TaskDir(), "config.yaml")
}

// PluginSearchPaths returns directories to search for plugin files
// Search order: project config dir → user config dir
func (pm *PathManager) PluginSearchPaths() []string {
	return []string{
		pm.TaskDir(), // Project config directory (for project-specific plugins)
		pm.configDir, // User config directory
	}
}

// TemplateFile returns the path to the user's custom new.md template
func (pm *PathManager) TemplateFile() string {
	return filepath.Join(pm.configDir, "new.md")
}

// EnsureDirs creates all necessary directories with appropriate permissions
func (pm *PathManager) EnsureDirs() error {
	// Create user config directory
	//nolint:gosec // G301: 0755 is appropriate for config directory
	if err := os.MkdirAll(pm.configDir, 0755); err != nil {
		return fmt.Errorf("create config directory %s: %w", pm.configDir, err)
	}

	// Create user cache directory (non-fatal if it fails)
	//nolint:gosec // G301: 0755 is appropriate for cache directory
	_ = os.MkdirAll(pm.cacheDir, 0755)

	// Create project directories
	//nolint:gosec // G301: 0755 is appropriate for task directory
	if err := os.MkdirAll(pm.TaskDir(), 0755); err != nil {
		return fmt.Errorf("create task directory %s: %w", pm.TaskDir(), err)
	}

	//nolint:gosec // G301: 0755 is appropriate for doki directory
	if err := os.MkdirAll(pm.DokiDir(), 0755); err != nil {
		return fmt.Errorf("create doki directory %s: %w", pm.DokiDir(), err)
	}

	return nil
}

// Package-level singleton with lazy initialization
var (
	pathManager     *PathManager
	pathManagerOnce sync.Once
	pathManagerErr  error
	pathManagerMu   sync.RWMutex // Protects pathManager for reset operations
)

// getPathManager returns the global PathManager, initializing it on first call
func getPathManager() (*PathManager, error) {
	pathManagerMu.RLock()
	if pathManager != nil {
		defer pathManagerMu.RUnlock()
		return pathManager, pathManagerErr
	}
	pathManagerMu.RUnlock()

	pathManagerMu.Lock()
	defer pathManagerMu.Unlock()

	// Double-check after acquiring write lock
	if pathManager != nil {
		return pathManager, pathManagerErr
	}

	pathManagerOnce.Do(func() {
		pathManager, pathManagerErr = newPathManager()
	})
	return pathManager, pathManagerErr
}

// InitPaths initializes the path manager. Must be called early in application startup.
// Returns an error if path initialization fails (e.g., cannot determine home directory).
func InitPaths() error {
	_, err := getPathManager()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPathManagerInit, err)
	}
	return nil
}

// ResetPathManager resets the path manager singleton for testing purposes.
// This allows tests to reinitialize paths with different environment variables.
func ResetPathManager() {
	pathManagerMu.Lock()
	defer pathManagerMu.Unlock()
	pathManager = nil
	pathManagerErr = nil
	pathManagerOnce = sync.Once{}
}

// mustGetPathManager returns the global PathManager or panics if not initialized.
// Callers should ensure InitPaths() was called successfully before using accessor functions.
func mustGetPathManager() *PathManager {
	pm, err := getPathManager()
	if err != nil {
		panic(fmt.Sprintf("path manager not initialized: %v (call InitPaths() first)", err))
	}
	return pm
}

// Exported accessor functions
// Note: These functions panic if InitPaths() has not been called successfully.
// The application should call InitPaths() early in main() and handle any error.

// GetConfigDir returns the user config directory
func GetConfigDir() string {
	return mustGetPathManager().ConfigDir()
}

// GetCacheDir returns the user cache directory
func GetCacheDir() string {
	return mustGetPathManager().CacheDir()
}

// GetConfigFile returns the path to the user config file
func GetConfigFile() string {
	return mustGetPathManager().ConfigFile()
}

// GetTaskDir returns the project-local task directory
func GetTaskDir() string {
	return mustGetPathManager().TaskDir()
}

// GetDokiDir returns the project-local documentation directory
func GetDokiDir() string {
	return mustGetPathManager().DokiDir()
}

// GetProjectConfigFile returns the path to the project-local config file
func GetProjectConfigFile() string {
	return mustGetPathManager().ProjectConfigFile()
}

// GetPluginSearchPaths returns directories to search for plugin files
func GetPluginSearchPaths() []string {
	return mustGetPathManager().PluginSearchPaths()
}

// GetTemplateFile returns the path to the user's custom new.md template
func GetTemplateFile() string {
	return mustGetPathManager().TemplateFile()
}

// EnsureDirs creates all necessary directories with appropriate permissions
func EnsureDirs() error {
	return mustGetPathManager().EnsureDirs()
}
