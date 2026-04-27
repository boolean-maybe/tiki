package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
logging:
  level: "debug"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Change to temp directory so viper can find the config
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	// Temporarily override XDG_CONFIG_HOME to prevent loading user config
	// This ensures the test uses only the config.yaml in tmpDir
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Reset appConfig to force a fresh load
	appConfig = nil
	// Reset PathManager so it picks up the new current directory and XDG_CONFIG_HOME
	ResetPathManager()

	// Load configuration
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify values
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected Logging.Level 'debug', got '%s'", cfg.Logging.Level)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// Create a temp directory without a config file
	tmpDir := t.TempDir()

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	// Reset viper and appConfig to force a fresh load
	appConfig = nil
	// Create a new viper instance to avoid state pollution from previous test
	// We need to call LoadConfig which will reset viper's state
	// But first we need to make sure previous config is cleared

	// Load configuration (should use defaults since no config.yaml exists)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify default values are applied (checking a known default)
	if cfg.Logging.Level != "error" {
		t.Errorf("Expected default Logging.Level 'error', got '%s'", cfg.Logging.Level)
	}
}

func TestLoadConfigEnvOverrideLoggingLevel(t *testing.T) {
	tmpDir := t.TempDir()

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	appConfig = nil
	t.Setenv("TIKI_LOGGING_LEVEL", "debug")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected Logging.Level 'debug', got '%s'", cfg.Logging.Level)
	}
}

func TestLoadConfigFlagOverrideLoggingLevel(t *testing.T) {
	tmpDir := t.TempDir()

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	originalArgs := os.Args
	os.Args = []string{originalArgs[0], "--log-level=warn"}
	defer func() { os.Args = originalArgs }()

	appConfig = nil

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Logging.Level != "warn" {
		t.Errorf("Expected Logging.Level 'warn', got '%s'", cfg.Logging.Level)
	}
}

func TestLoadConfigCodeBlock(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
appearance:
  codeBlock:
    theme: dracula
    background: "#282a36"
    border: "#6272a4"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	appConfig = nil
	ResetPathManager()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Appearance.CodeBlock.Theme != "dracula" {
		t.Errorf("expected codeBlock.theme 'dracula', got '%s'", cfg.Appearance.CodeBlock.Theme)
	}
	if cfg.Appearance.CodeBlock.Background != "#282a36" {
		t.Errorf("expected codeBlock.background '#282a36', got '%s'", cfg.Appearance.CodeBlock.Background)
	}
	if cfg.Appearance.CodeBlock.Border != "#6272a4" {
		t.Errorf("expected codeBlock.border '#6272a4', got '%s'", cfg.Appearance.CodeBlock.Border)
	}

	// verify getters
	if got := GetCodeBlockTheme(); got != "dracula" {
		t.Errorf("GetCodeBlockTheme() = '%s', want 'dracula'", got)
	}
	if got := GetCodeBlockBackground(); got != "#282a36" {
		t.Errorf("GetCodeBlockBackground() = '%s', want '#282a36'", got)
	}
	if got := GetCodeBlockBorder(); got != "#6272a4" {
		t.Errorf("GetCodeBlockBorder() = '%s', want '#6272a4'", got)
	}
}

func TestLoadConfigCodeBlockDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	appConfig = nil

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// codeBlock.theme is empty in config (resolved dynamically by GetCodeBlockTheme)
	if cfg.Appearance.CodeBlock.Theme != "" {
		t.Errorf("expected empty default codeBlock.theme, got '%s'", cfg.Appearance.CodeBlock.Theme)
	}
	// GetCodeBlockTheme resolves to "nord" for dark (default) theme
	if got := GetCodeBlockTheme(); got != "nord" {
		t.Errorf("expected GetCodeBlockTheme() 'nord' for dark theme, got '%s'", got)
	}
	if cfg.Appearance.CodeBlock.Background != "" {
		t.Errorf("expected empty default codeBlock.background, got '%s'", cfg.Appearance.CodeBlock.Background)
	}
	if cfg.Appearance.CodeBlock.Border != "" {
		t.Errorf("expected empty default codeBlock.border, got '%s'", cfg.Appearance.CodeBlock.Border)
	}
}

func TestLoadConfig_ProjectOverridesUser(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	userTikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(userTikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userTikiDir, "config.yaml"), []byte(`
logging:
  level: error
header:
  visible: false
`), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir()
	docDir := filepath.Join(projectDir, ".doc")
	if err := os.MkdirAll(docDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docDir, "config.yaml"), []byte(`
logging:
  level: debug
`), 0644); err != nil {
		t.Fatal(err)
	}

	cwdDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	appConfig = nil
	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = projectDir

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// project file wins exclusively
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected logging.level 'debug' from project, got %q", cfg.Logging.Level)
	}
	// header.visible falls back to built-in default (true), not inherited from user config
	if cfg.Header.Visible != true {
		t.Errorf("expected header.visible true (built-in default), got %v", cfg.Header.Visible)
	}
}

func TestLoadConfig_CwdOverridesProject(t *testing.T) {
	// user config
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	userTikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(userTikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userTikiDir, "config.yaml"), []byte(`
logging:
  level: error
`), 0644); err != nil {
		t.Fatal(err)
	}

	// project config
	projectDir := t.TempDir()
	docDir := filepath.Join(projectDir, ".doc")
	if err := os.MkdirAll(docDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docDir, "config.yaml"), []byte(`
logging:
  level: warn
`), 0644); err != nil {
		t.Fatal(err)
	}

	// cwd config (highest priority)
	cwdDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwdDir, "config.yaml"), []byte(`
logging:
  level: debug
`), 0644); err != nil {
		t.Fatal(err)
	}

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	appConfig = nil
	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = projectDir

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected logging.level 'debug' from cwd, got %q", cfg.Logging.Level)
	}
}

func TestLoadConfig_UserOnlyFallback(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	userTikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(userTikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userTikiDir, "config.yaml"), []byte(`
logging:
  level: info
header:
  visible: false
`), 0644); err != nil {
		t.Fatal(err)
	}

	// cwd with no config, project with no config
	cwdDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	appConfig = nil
	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = cwdDir // no .doc/ dir here

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("expected logging.level 'info' from user config, got %q", cfg.Logging.Level)
	}
	if cfg.Header.Visible != false {
		t.Errorf("expected header.visible false from user config, got %v", cfg.Header.Visible)
	}
}

func TestLoadConfigAIAgent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
ai:
  agent: claude
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	appConfig = nil
	ResetPathManager()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.AI.Agent != "claude" {
		t.Errorf("expected ai.agent 'claude', got '%s'", cfg.AI.Agent)
	}
	if got := GetAIAgent(); got != "claude" {
		t.Errorf("GetAIAgent() = '%s', want 'claude'", got)
	}
}

func TestLoadConfigAIAgentDefault(t *testing.T) {
	tmpDir := t.TempDir()

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	appConfig = nil

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.AI.Agent != "" {
		t.Errorf("expected empty ai.agent by default, got '%s'", cfg.AI.Agent)
	}
	if got := GetAIAgent(); got != "" {
		t.Errorf("GetAIAgent() = '%s', want ''", got)
	}
}

func TestGetPluginViewMode_ReadsFromWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	workflowContent := `views:
  plugins:
    - name: Kanban
      key: "F1"
    - name: Dependency
      view: expanded
`
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	ResetPathManager()

	if got := GetPluginViewMode("Dependency"); got != "expanded" {
		t.Errorf("GetPluginViewMode(Dependency) = %q, want %q", got, "expanded")
	}
	if got := GetPluginViewMode("Kanban"); got != "" {
		t.Errorf("GetPluginViewMode(Kanban) = %q, want empty (no view field)", got)
	}
	if got := GetPluginViewMode("NonExistent"); got != "" {
		t.Errorf("GetPluginViewMode(NonExistent) = %q, want empty", got)
	}
}

func TestGetConfig(t *testing.T) {
	// Reset appConfig
	appConfig = nil

	// First call should load config
	cfg1 := GetConfig()
	if cfg1 == nil {
		t.Fatal("GetConfig returned nil")
	}

	// Second call should return same instance
	cfg2 := GetConfig()
	if cfg1 != cfg2 {
		t.Error("GetConfig should return the same instance")
	}
}

func TestLoadConfigStore(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
store:
  name: tiki
  git: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	appConfig = nil
	ResetPathManager()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Store.Name != "tiki" {
		t.Errorf("expected store.name 'tiki', got '%s'", cfg.Store.Name)
	}
	if cfg.Store.Git != false {
		t.Errorf("expected store.git false, got %v", cfg.Store.Git)
	}
	if got := GetStoreName(); got != "tiki" {
		t.Errorf("GetStoreName() = '%s', want 'tiki'", got)
	}
	if got := GetStoreGit(); got != false {
		t.Errorf("GetStoreGit() = %v, want false", got)
	}
}

func TestLoadConfigStoreDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	appConfig = nil

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Store.Name != "tiki" {
		t.Errorf("expected default store.name 'tiki', got '%s'", cfg.Store.Name)
	}
	if cfg.Store.Git != true {
		t.Errorf("expected default store.git true, got %v", cfg.Store.Git)
	}
	if got := GetStoreName(); got != "tiki" {
		t.Errorf("GetStoreName() = '%s', want 'tiki'", got)
	}
	if got := GetStoreGit(); got != true {
		t.Errorf("GetStoreGit() = %v, want true", got)
	}
}

func TestLoadConfigIdentity(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
identity:
  name: "Alice Example"
  email: "alice@example.com"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	appConfig = nil
	ResetPathManager()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Identity.Name != "Alice Example" {
		t.Errorf("expected identity.name 'Alice Example', got %q", cfg.Identity.Name)
	}
	if cfg.Identity.Email != "alice@example.com" {
		t.Errorf("expected identity.email 'alice@example.com', got %q", cfg.Identity.Email)
	}
	if got := GetIdentityName(); got != "Alice Example" {
		t.Errorf("GetIdentityName() = %q, want 'Alice Example'", got)
	}
	if got := GetIdentityEmail(); got != "alice@example.com" {
		t.Errorf("GetIdentityEmail() = %q, want 'alice@example.com'", got)
	}
}

func TestLoadConfigIdentityDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	appConfig = nil

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Identity.Name != "" {
		t.Errorf("expected empty identity.name by default, got %q", cfg.Identity.Name)
	}
	if cfg.Identity.Email != "" {
		t.Errorf("expected empty identity.email by default, got %q", cfg.Identity.Email)
	}
	if got := GetIdentityName(); got != "" {
		t.Errorf("GetIdentityName() = %q, want empty", got)
	}
	if got := GetIdentityEmail(); got != "" {
		t.Errorf("GetIdentityEmail() = %q, want empty", got)
	}
}

func TestLoadConfigIdentityEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	appConfig = nil
	t.Setenv("TIKI_IDENTITY_NAME", "Env Name")
	t.Setenv("TIKI_IDENTITY_EMAIL", "env@example.com")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Identity.Name != "Env Name" {
		t.Errorf("expected identity.name 'Env Name' from env, got %q", cfg.Identity.Name)
	}
	if cfg.Identity.Email != "env@example.com" {
		t.Errorf("expected identity.email 'env@example.com' from env, got %q", cfg.Identity.Email)
	}
}

func TestLoadConfigStoreEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	appConfig = nil
	t.Setenv("TIKI_STORE_GIT", "false")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Store.Git != false {
		t.Errorf("expected store.git false from env override, got %v", cfg.Store.Git)
	}
	if got := GetStoreGit(); got != false {
		t.Errorf("GetStoreGit() = %v, want false", got)
	}
}
