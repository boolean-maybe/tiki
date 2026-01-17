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

	// Reset appConfig to force a fresh load
	appConfig = nil

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
