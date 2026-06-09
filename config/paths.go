package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	// ErrNoHome indicates that the user's home directory could not be determined
	ErrNoHome = errors.New("unable to determine home directory")

	// ErrPathManagerInit indicates that the PathManager failed to initialize
	ErrPathManagerInit = errors.New("failed to initialize path manager")

	// ErrInvalidDocDir indicates that the configured store.dir value is invalid
	ErrInvalidDocDir = errors.New("invalid store.dir")
)

// PathManager manages all file system paths for tiki
type PathManager struct {
	configDir   string // User config directory
	cacheDir    string // User cache directory
	projectRoot string // Current working directory
	docDirName  string // Relative document directory name (default ".doc")
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

	docDirName := defaultDocDirName
	if demoDocDirOverride != "" {
		docDirName = demoDocDirOverride
	} else {
		resolved, resolveErr := resolveUserConfiguredDocDir(configDir)
		if resolveErr != nil {
			return nil, fmt.Errorf("resolve store.dir: %w", resolveErr)
		}
		if resolved != "" {
			docDirName = resolved
		}
	}

	return &PathManager{
		configDir:   configDir,
		cacheDir:    cacheDir,
		projectRoot: projectRoot,
		docDirName:  docDirName,
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

// DocDir returns the unified document root. All managed markdown
// documents live somewhere under this directory — this is the single scan
// root for the document store. Defaults to ".doc" but configurable via
// user-level store.dir.
func (pm *PathManager) DocDir() string {
	return filepath.Join(pm.projectRoot, pm.docDirName)
}

// ProjectConfigDir returns the project-level config directory.
// Identical to DocDir by construction — workflow.yaml and config.yaml live
// alongside document files at the unified root.
func (pm *PathManager) ProjectConfigDir() string {
	return filepath.Join(pm.projectRoot, pm.docDirName)
}

// UserConfigWorkflowFile returns the path to workflow.yaml in the user config directory
func (pm *PathManager) UserConfigWorkflowFile() string {
	return filepath.Join(pm.configDir, defaultWorkflowFilename)
}

// EnsureDirs creates all necessary directories with appropriate permissions.
// Fresh projects get a flat `.doc/` root with no subdirectories created by
// init — users organize content however they want underneath. Existing
// projects with subdirectories under `.doc/` continue to work; nothing
// removes them and the loader walks the tree recursively.
func (pm *PathManager) EnsureDirs() error {
	// Create user config directory
	//nolint:gosec // G301: 0755 is appropriate for config directory
	if err := os.MkdirAll(pm.configDir, 0755); err != nil {
		return fmt.Errorf("create config directory %s: %w", pm.configDir, err)
	}

	// Create user cache directory (non-fatal if it fails)
	//nolint:gosec // G301: 0755 is appropriate for cache directory
	_ = os.MkdirAll(pm.cacheDir, 0755)

	// Create the unified document root.
	//nolint:gosec // G301: 0755 is appropriate for project document directory
	if err := os.MkdirAll(pm.DocDir(), 0755); err != nil {
		return fmt.Errorf("create document directory %s: %w", pm.DocDir(), err)
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
// Also clears the demo doc-dir override so tests do not leak state.
func ResetPathManager() {
	pathManagerMu.Lock()
	defer pathManagerMu.Unlock()
	pathManager = nil
	pathManagerErr = nil
	pathManagerOnce = sync.Once{}
	demoDocDirOverride = ""
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

// GetDocDir returns the unified document root (default ".doc", configurable
// via user-level store.dir). This is the single scan root for the document
// store; brand-new documents are written at <doc-dir>/<ID>.md, while loading
// is filename-agnostic — every `.md` under the doc dir is loaded and the id
// comes from the frontmatter `id:` field.
func GetDocDir() string {
	return mustGetPathManager().DocDir()
}

// GetProjectConfigDir returns the project-level config directory (same as doc dir)
func GetProjectConfigDir() string {
	return mustGetPathManager().ProjectConfigDir()
}

// GetUserConfigWorkflowFile returns the path to workflow.yaml in the user config directory
func GetUserConfigWorkflowFile() string {
	return mustGetPathManager().UserConfigWorkflowFile()
}

// configFilename is the default name for the configuration file
const configFilename = "config.yaml"

// defaultWorkflowFilename is the default name for the workflow configuration file
const defaultWorkflowFilename = "workflow.yaml"

// defaultDocDirName is the built-in document directory name when no user config overrides it.
const defaultDocDirName = ".doc"

// demoDocDirOverride forces a specific doc dir name during demo mode.
// Set via setDemoDocDirOverride, cleared via ResetPathManager.
var demoDocDirOverride string

// SetDemoDocDirOverride forces the path manager to use the given doc dir name,
// bypassing user config. Used only by the demo command. Must be called after
// ResetPathManager and before InitPaths.
func SetDemoDocDirOverride(name string) {
	demoDocDirOverride = name
}

// GetDocDirName returns the configured relative document directory name (e.g. ".doc", "docs").
func GetDocDirName() string {
	return mustGetPathManager().docDirName
}

// ValidateDocDir checks that name is a valid relative document directory name.
// Rejects empty, absolute, ".." segments, and paths that resolve to the project root.
func ValidateDocDir(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty directory name", ErrInvalidDocDir)
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("%w: must be a relative path", ErrInvalidDocDir)
	}
	if containsDotDot(name) {
		return fmt.Errorf("%w: must not contain '..' segments", ErrInvalidDocDir)
	}
	cleaned := filepath.Clean(name)
	if cleaned == "." {
		return fmt.Errorf("%w: must not resolve to the project root", ErrInvalidDocDir)
	}
	return nil
}

// containsDotDot reports whether the path contains a ".." segment.
func containsDotDot(path string) bool {
	cleaned := filepath.ToSlash(path)
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// userDocDirConfig is the minimal YAML shape for the early user-config pre-read.
type userDocDirConfig struct {
	Store struct {
		Dir string `yaml:"dir"`
	} `yaml:"store"`
}

// resolveUserConfiguredDocDir reads only the user config file and returns the
// configured store.dir value, or empty string if unset/missing. Returns an error
// for invalid directory names. YAML parse errors or type mismatches are treated
// as "unset" (the full config loader will report them later).
func resolveUserConfiguredDocDir(configDir string) (string, error) {
	path := filepath.Join(configDir, configFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading user config %s: %w", path, err)
	}

	var cfg userDocDirConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// YAML that doesn't fit our minimal shape (e.g. a bare scalar) is not
		// fatal here — the full config loader will catch actual parse errors.
		return "", nil
	}

	dir := cfg.Store.Dir
	if dir == "" {
		return "", nil
	}

	if err := ValidateDocDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// findHighestPriorityFile returns the highest-priority existing file from
// the standard search order: user config → project config → cwd.
// The last existing (deduplicated) candidate wins.
func findHighestPriorityFile(candidates []string) string {
	var best string
	seen := make(map[string]bool)

	for _, path := range candidates {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if seen[abs] {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		seen[abs] = true
		best = path
	}

	return best
}

// workflowCandidates returns the standard workflow.yaml search paths
// in priority order: user config (lowest) → project config → cwd (highest).
func workflowCandidates() []string {
	pm := mustGetPathManager()
	return []string{
		pm.UserConfigWorkflowFile(),
		filepath.Join(pm.ProjectConfigDir(), defaultWorkflowFilename),
		defaultWorkflowFilename, // relative to cwd
	}
}

// findWorkflowFileWithScope walks the workflow candidates in priority order
// and returns both the winning path and its classified scope. This is the
// single source of truth — FindWorkflowFile and FindWorkflowFiles delegate here.
func findWorkflowFileWithScope() (string, Scope) {
	scopes := []Scope{ScopeGlobal, ScopeLocal, ScopeCurrent}
	candidates := workflowCandidates()

	var bestPath string
	var bestScope Scope
	seen := make(map[string]bool)

	for i, path := range candidates {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if seen[abs] {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		seen[abs] = true
		bestPath = path
		bestScope = scopes[i]
	}

	return bestPath, bestScope
}

// FindWorkflowFileWithScope returns the highest-priority workflow.yaml path
// and its classified scope. If no file is found, returns ("", "").
func FindWorkflowFileWithScope() (string, Scope) {
	return findWorkflowFileWithScope()
}

// FindWorkflowFiles returns a single-element slice with the highest-priority
// workflow.yaml that exists, or nil if none found. All workflow-backed sections
// (views, statuses, types, fields, triggers) come from this one file.
func FindWorkflowFiles() []string {
	path, _ := findWorkflowFileWithScope()
	if path == "" {
		return nil
	}
	return []string{path}
}

// FindWorkflowFile returns the highest-priority workflow.yaml path,
// or empty string if none found.
func FindWorkflowFile() string {
	path, _ := findWorkflowFileWithScope()
	return path
}

// WorkflowScopeLabel returns a user-friendly label for display in the statusline.
func WorkflowScopeLabel(scope Scope) string {
	switch scope {
	case ScopeGlobal:
		return "global"
	case ScopeLocal:
		return "project"
	case ScopeCurrent:
		return "local"
	default:
		return string(scope)
	}
}

// EnsureDirs creates all necessary directories with appropriate permissions
func EnsureDirs() error {
	return mustGetPathManager().EnsureDirs()
}
