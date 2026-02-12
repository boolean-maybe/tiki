package config

// Viper configuration loader: reads config.yaml from the binary's directory

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds all application configuration loaded from config.yaml
type Config struct {
	// Logging configuration
	Logging struct {
		Level string `mapstructure:"level"` // "debug", "info", "warn", "error"
	} `mapstructure:"logging"`

	// Board view configuration
	Board struct {
		View string `mapstructure:"view"` // "compact" or "expanded"
	} `mapstructure:"board"`

	// Header configuration
	Header struct {
		Visible bool `mapstructure:"visible"`
	} `mapstructure:"header"`

	// Tiki configuration
	Tiki struct {
		MaxPoints int `mapstructure:"maxPoints"`
	} `mapstructure:"tiki"`

	// Appearance configuration
	Appearance struct {
		Theme             string `mapstructure:"theme"`             // "dark", "light", "auto"
		GradientThreshold int    `mapstructure:"gradientThreshold"` // Minimum color count for gradients (16, 256, 16777216)
	} `mapstructure:"appearance"`
}

var appConfig *Config

// LoadConfig loads configuration from config.yaml
// Priority order (first found wins): project config → user config → current directory (dev)
// If config.yaml doesn't exist, it uses default values
func LoadConfig() (*Config, error) {
	// Reset viper to clear any previous configuration
	viper.Reset()

	// Configure viper to look for config.yaml
	// Viper uses first-found priority, so project config takes precedence
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Add search paths in priority order (first added = highest priority)
	projectConfigDir := filepath.Dir(GetProjectConfigFile())
	viper.AddConfigPath(projectConfigDir) // Project config (highest priority)
	viper.AddConfigPath(GetConfigDir())   // User config
	viper.AddConfigPath(".")              // Current directory (development)

	// Set default values
	setDefaults()

	// Read the config file (if it exists)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Debug("no config.yaml found, using defaults")
		} else {
			slog.Error("error reading config file", "error", err)
			return nil, err
		}
	} else {
		slog.Debug("loaded configuration", "file", viper.ConfigFileUsed())
	}

	// Allow environment variables to override config file
	viper.SetEnvPrefix("TIKI")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := bindFlags(); err != nil {
		slog.Warn("failed to bind command line flags", "error", err)
	}

	// Unmarshal config into struct
	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		slog.Error("failed to unmarshal config", "error", err)
		return nil, err
	}

	appConfig = cfg
	return cfg, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Logging defaults
	viper.SetDefault("logging.level", "error")

	// Header defaults
	viper.SetDefault("header.visible", true)

	// Tiki defaults
	viper.SetDefault("tiki.maxPoints", 10)

	// Appearance defaults
	viper.SetDefault("appearance.theme", "auto")
	viper.SetDefault("appearance.gradientThreshold", 256)
}

// bindFlags binds supported command line flags to viper so they can override config values.
func bindFlags() error {
	flagSet := pflag.NewFlagSet("tiki", pflag.ContinueOnError)
	flagSet.ParseErrorsWhitelist.UnknownFlags = true
	flagSet.SetOutput(io.Discard)

	flagSet.String("log-level", "", "Log level (debug, info, warn, error)")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return err
	}

	return viper.BindPFlag("logging.level", flagSet.Lookup("log-level"))
}

// GetConfig returns the loaded configuration
// If config hasn't been loaded yet, it loads it first
func GetConfig() *Config {
	if appConfig == nil {
		cfg, err := LoadConfig()
		if err != nil {
			// If loading fails, return a config with defaults
			slog.Warn("failed to load config, using defaults", "error", err)
			setDefaults()
			cfg = &Config{}
			_ = viper.Unmarshal(cfg)
		}
		appConfig = cfg
	}
	return appConfig
}

// GetString is a convenience method to get a string value from config
func GetString(key string) string {
	return viper.GetString(key)
}

// GetBool is a convenience method to get a boolean value from config
func GetBool(key string) bool {
	return viper.GetBool(key)
}

// GetInt is a convenience method to get an integer value from config
func GetInt(key string) int {
	return viper.GetInt(key)
}

// workflowFileData represents the YAML structure of workflow.yaml for read-modify-write.
// kept in config package to avoid import cycle with plugin package.
type workflowFileData struct {
	Plugins []map[string]interface{} `yaml:"views"`
}

// readWorkflowFile reads and unmarshals workflow.yaml from the given path.
func readWorkflowFile(path string) (*workflowFileData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow.yaml: %w", err)
	}
	var wf workflowFileData
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing workflow.yaml: %w", err)
	}
	return &wf, nil
}

// writeWorkflowFile marshals and writes workflow.yaml to the given path.
func writeWorkflowFile(path string, wf *workflowFileData) error {
	data, err := yaml.Marshal(wf)
	if err != nil {
		return fmt.Errorf("marshaling workflow.yaml: %w", err)
	}
	//nolint:gosec // G306: 0644 is appropriate for config file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing workflow.yaml: %w", err)
	}
	return nil
}

// GetBoardViewMode loads the board view mode from workflow.yaml.
// Returns "expanded" as default if not found.
func GetBoardViewMode() string {
	return getPluginViewModeFromWorkflow("Board", "expanded")
}

// getPluginViewModeFromWorkflow reads a plugin's view mode from workflow.yaml by name.
func getPluginViewModeFromWorkflow(pluginName string, defaultValue string) string {
	path := FindWorkflowFile()
	if path == "" {
		return defaultValue
	}

	wf, err := readWorkflowFile(path)
	if err != nil {
		slog.Debug("failed to read workflow.yaml for view mode", "error", err)
		return defaultValue
	}

	for _, p := range wf.Plugins {
		if name, ok := p["name"].(string); ok && name == pluginName {
			if view, ok := p["view"].(string); ok && view != "" {
				return view
			}
		}
	}

	return defaultValue
}

// SavePluginViewMode saves a plugin's view mode to workflow.yaml.
// configIndex: index in workflow.yaml plugins array (-1 to find/create by name)
func SavePluginViewMode(pluginName string, configIndex int, viewMode string) error {
	path := FindWorkflowFile()
	if path == "" {
		// create workflow.yaml in user config dir
		path = GetUserConfigWorkflowFile()
	}

	var wf *workflowFileData

	// try to read existing file
	if existing, err := readWorkflowFile(path); err == nil {
		wf = existing
	} else {
		wf = &workflowFileData{}
	}

	if configIndex >= 0 && configIndex < len(wf.Plugins) {
		// update existing entry by index
		wf.Plugins[configIndex]["view"] = viewMode
	} else {
		// find by name or create new entry
		existingIndex := -1
		for i, p := range wf.Plugins {
			if name, ok := p["name"].(string); ok && name == pluginName {
				existingIndex = i
				break
			}
		}

		if existingIndex >= 0 {
			wf.Plugins[existingIndex]["view"] = viewMode
		} else {
			newEntry := map[string]interface{}{
				"name": pluginName,
				"view": viewMode,
			}
			wf.Plugins = append(wf.Plugins, newEntry)
		}
	}

	return writeWorkflowFile(path, wf)
}

// SaveHeaderVisible saves the header visibility setting to config.yaml
func SaveHeaderVisible(visible bool) error {
	viper.Set("header.visible", visible)
	return saveConfig()
}

// GetHeaderVisible returns the header visibility setting
func GetHeaderVisible() bool {
	return viper.GetBool("header.visible")
}

// GetMaxPoints returns the maximum points value for tasks
func GetMaxPoints() int {
	maxPoints := viper.GetInt("tiki.maxPoints")
	// Ensure minimum of 1
	if maxPoints < 1 {
		return 10 // fallback to default
	}
	return maxPoints
}

// saveConfig writes the current viper configuration to config.yaml
func saveConfig() error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		// If no config file was loaded, save to user config directory
		configFile = GetConfigFile()
	}

	return viper.WriteConfigAs(configFile)
}

// GetTheme returns the appearance theme setting
func GetTheme() string {
	theme := viper.GetString("appearance.theme")
	if theme == "" {
		return "auto"
	}
	return theme
}

// GetEffectiveTheme resolves "auto" to actual theme based on terminal detection
func GetEffectiveTheme() string {
	theme := GetTheme()
	if theme != "auto" {
		return theme
	}
	// Detect via COLORFGBG env var (format: "fg;bg")
	if colorfgbg := os.Getenv("COLORFGBG"); colorfgbg != "" {
		parts := strings.Split(colorfgbg, ";")
		if len(parts) >= 2 {
			bg := parts[len(parts)-1]
			// 0-7 = dark colors, 8+ = light colors
			if bg >= "8" {
				return "light"
			}
		}
	}
	return "dark" // default fallback
}

// GetContentBackgroundColor returns the background color for markdown content areas
// Dark theme needs black background for light text; light theme uses terminal default
func GetContentBackgroundColor() tcell.Color {
	if GetEffectiveTheme() == "dark" {
		return tcell.ColorBlack
	}
	return tcell.ColorDefault
}

// GetContentTextColor returns the appropriate text color for content areas
// Dark theme uses white text; light theme uses black text
func GetContentTextColor() tcell.Color {
	if GetEffectiveTheme() == "dark" {
		return tcell.ColorWhite
	}
	return tcell.ColorBlack
}

// GetGradientThreshold returns the minimum color count required for gradients
// Valid values: 16, 256, 16777216 (truecolor)
func GetGradientThreshold() int {
	threshold := viper.GetInt("appearance.gradientThreshold")
	if threshold < 1 {
		return 256 // fallback to default
	}
	return threshold
}
