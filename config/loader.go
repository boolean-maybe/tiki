package config

// Viper configuration loader: reads config.yaml from the binary's directory

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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
		Theme string `mapstructure:"theme"` // "dark", "light", "auto"
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

// SaveBoardViewMode saves the board view mode to config.yaml
// Deprecated: Use SavePluginViewMode("Board", -1, viewMode) instead
func SaveBoardViewMode(viewMode string) error {
	viper.Set("board.view", viewMode)
	return saveConfig()
}

// GetBoardViewMode loads the board view mode from config
// Priority: plugins array entry with name "Board", then default
func GetBoardViewMode() string {
	// Check plugins array
	var currentPlugins []map[string]interface{}
	if err := viper.UnmarshalKey("plugins", &currentPlugins); err == nil {
		for _, p := range currentPlugins {
			if name, ok := p["name"].(string); ok && name == "Board" {
				if view, ok := p["view"].(string); ok && view != "" {
					return view
				}
			}
		}
	}

	// Default
	return "expanded"
}

// SavePluginViewMode saves a plugin's view mode to config.yaml
// This function updates or creates the plugin entry in the plugins array
// configIndex: index in config array (-1 to create new entry by name)
func SavePluginViewMode(pluginName string, configIndex int, viewMode string) error {
	// Get current plugins configuration
	var currentPlugins []map[string]interface{}
	if err := viper.UnmarshalKey("plugins", &currentPlugins); err != nil {
		// If no plugins exist or unmarshal fails, start with empty array
		currentPlugins = []map[string]interface{}{}
	}

	if configIndex >= 0 && configIndex < len(currentPlugins) {
		// Update existing config entry (works for inline, file-based, or hybrid)
		currentPlugins[configIndex]["view"] = viewMode
	} else {
		// Embedded plugin or missing entry - check if name-based entry already exists
		existingIndex := -1
		for i, p := range currentPlugins {
			if name, ok := p["name"].(string); ok && name == pluginName {
				existingIndex = i
				break
			}
		}

		if existingIndex >= 0 {
			// Update existing name-based entry
			currentPlugins[existingIndex]["view"] = viewMode
		} else {
			// Create new name-based entry
			newEntry := map[string]interface{}{
				"name": pluginName,
				"view": viewMode,
			}
			currentPlugins = append(currentPlugins, newEntry)
		}
	}

	// Save back to viper
	viper.Set("plugins", currentPlugins)
	return saveConfig()
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
