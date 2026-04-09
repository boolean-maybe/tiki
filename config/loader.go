package config

// Viper configuration loader: merges config.yaml from multiple locations

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// lastConfigFile tracks the most recently merged config file path for saveConfig().
var lastConfigFile string

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
		MaxPoints    int `mapstructure:"maxPoints"`
		MaxImageRows int `mapstructure:"maxImageRows"`
	} `mapstructure:"tiki"`

	// Appearance configuration
	Appearance struct {
		Theme             string `mapstructure:"theme"`             // "dark", "light", "auto"
		GradientThreshold int    `mapstructure:"gradientThreshold"` // Minimum color count for gradients (16, 256, 16777216)
		CodeBlock         struct {
			Theme      string `mapstructure:"theme"`      // chroma syntax theme (e.g. "dracula", "monokai")
			Background string `mapstructure:"background"` // hex "#282a36" or ANSI "236"
			Border     string `mapstructure:"border"`     // hex "#6272a4" or ANSI "244"
		} `mapstructure:"codeBlock"`
	} `mapstructure:"appearance"`

	// AI agent configuration — valid keys defined in aitools.go via AITools()
	AI struct {
		Agent string `mapstructure:"agent"`
	} `mapstructure:"ai"`
}

var appConfig *Config

// LoadConfig loads configuration by merging config.yaml from multiple locations.
// Files are merged in precedence order (user → project → cwd); later files override
// earlier ones. Missing values fall back to built-in defaults.
func LoadConfig() (*Config, error) {
	viper.Reset()
	viper.SetConfigType("yaml")
	setDefaults()
	lastConfigFile = ""

	// merge config files in precedence order (first = base, last = highest priority)
	for _, path := range findConfigFiles() {
		f, err := os.Open(path)
		if err != nil {
			slog.Warn("failed to open config file", "path", path, "error", err)
			continue
		}
		mergeErr := viper.MergeConfig(f)
		_ = f.Close()
		if mergeErr != nil {
			return nil, fmt.Errorf("merging config from %s: %w", path, mergeErr)
		}
		lastConfigFile = path
		slog.Debug("merged configuration", "file", path)
	}

	if lastConfigFile == "" {
		slog.Debug("no config.yaml found, using defaults")
	}

	// environment variables and flags override everything
	viper.SetEnvPrefix("TIKI")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := bindFlags(); err != nil {
		slog.Warn("failed to bind command line flags", "error", err)
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	appConfig = cfg
	return cfg, nil
}

// findConfigFiles returns existing config.yaml paths in merge order
// (user config → project → cwd). Deduplicates by absolute path.
func findConfigFiles() []string {
	pm := mustGetPathManager()

	candidates := []string{
		pm.ConfigFile(), // user config (base)
		filepath.Join(pm.ProjectConfigDir(), "config.yaml"), // project override
		filepath.Join(".", "config.yaml"),                   // cwd override (highest)
	}

	var result []string
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
		result = append(result, path)
	}

	return result
}

// setDefaults sets default configuration values
func setDefaults() {
	// Logging defaults
	viper.SetDefault("logging.level", "error")

	// Header defaults
	viper.SetDefault("header.visible", true)

	// Tiki defaults
	viper.SetDefault("tiki.maxPoints", 10)
	viper.SetDefault("tiki.maxImageRows", 40)

	// Appearance defaults
	viper.SetDefault("appearance.theme", "auto")
	viper.SetDefault("appearance.gradientThreshold", 256)
	viper.SetDefault("appearance.codeBlock.theme", "nord")
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

// workflowFileData represents the YAML structure of workflow.yaml for read-modify-write.
// kept in config package to avoid import cycle with plugin package.
// all top-level sections must be listed here to survive round-trip serialization.
type workflowFileData struct {
	Statuses []map[string]interface{} `yaml:"statuses,omitempty"`
	Plugins  []map[string]interface{} `yaml:"views"`
	Triggers []map[string]interface{} `yaml:"triggers,omitempty"`
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

// GetPluginViewMode reads a plugin's view mode from workflow.yaml by name.
// Returns empty string if not found.
func GetPluginViewMode(pluginName string) string {
	return getPluginViewModeFromWorkflow(pluginName, "")
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

// GetMaxImageRows returns the maximum rows for inline image rendering
func GetMaxImageRows() int {
	rows := viper.GetInt("tiki.maxImageRows")
	if rows < 1 {
		return 40
	}
	return rows
}

// saveConfig writes the current viper configuration to config.yaml.
// Saves to the last merged config file, or the user config dir if none was loaded.
func saveConfig() error {
	configFile := lastConfigFile
	if configFile == "" {
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

// GetGradientThreshold returns the minimum color count required for gradients
// Valid values: 16, 256, 16777216 (truecolor)
func GetGradientThreshold() int {
	threshold := viper.GetInt("appearance.gradientThreshold")
	if threshold < 1 {
		return 256 // fallback to default
	}
	return threshold
}

// GetCodeBlockTheme returns the chroma syntax highlighting theme for code blocks
func GetCodeBlockTheme() string {
	return viper.GetString("appearance.codeBlock.theme")
}

// GetCodeBlockBackground returns the background color for code blocks
func GetCodeBlockBackground() string {
	return viper.GetString("appearance.codeBlock.background")
}

// GetCodeBlockBorder returns the border color for code blocks
func GetCodeBlockBorder() string {
	return viper.GetString("appearance.codeBlock.border")
}

// GetAIAgent returns the configured AI agent tool name, or empty string if not configured
func GetAIAgent() string {
	return viper.GetString("ai.agent")
}
