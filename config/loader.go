package config

// Viper configuration loader: loads config.yaml from the highest-priority location

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/muesli/termenv"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds all application configuration loaded from config.yaml
type Config struct {
	Version string `mapstructure:"version"`

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
		Theme             string `mapstructure:"theme"`             // "auto", "dark", "light", or a named theme (see ThemeNames())
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

// LoadConfig loads configuration from the highest-priority config.yaml.
// Only one file is used (cwd > project > user config). Missing values
// fall back to built-in defaults.
func LoadConfig() (*Config, error) {
	viper.Reset()
	viper.SetConfigType("yaml")
	setDefaults()

	path := findConfigFile()
	if path == "" {
		slog.Debug("no config.yaml found, using defaults")
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening config %s: %w", path, err)
		}
		readErr := viper.ReadConfig(f)
		_ = f.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading config from %s: %w", path, readErr)
		}
		slog.Debug("loaded configuration", "file", path)
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

// findConfigFile returns the highest-priority config.yaml that exists,
// or empty string if none found. Priority: cwd > project > user config.
func findConfigFile() string {
	pm := mustGetPathManager()
	return findHighestPriorityFile([]string{
		pm.ConfigFile(),
		filepath.Join(pm.ProjectConfigDir(), "config.yaml"),
		filepath.Join(".", "config.yaml"),
	})
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
	// code block theme resolved dynamically in GetCodeBlockTheme()
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

// viewsFileData represents the views section of workflow.yaml for read-modify-write.
type viewsFileData struct {
	Actions []map[string]interface{} `yaml:"actions,omitempty"`
	Plugins []map[string]interface{} `yaml:"plugins"`
}

// workflowFileData represents the YAML structure of workflow.yaml for read-modify-write.
// kept in config package to avoid import cycle with plugin package.
// all top-level sections must be listed here to survive round-trip serialization.
type workflowFileData struct {
	Version     string                   `yaml:"version,omitempty"`
	Description string                   `yaml:"description,omitempty"`
	Statuses    []map[string]interface{} `yaml:"statuses,omitempty"`
	Types       []map[string]interface{} `yaml:"types,omitempty"`
	Views       viewsFileData            `yaml:"views,omitempty"`
	Triggers    []map[string]interface{} `yaml:"triggers,omitempty"`
	Fields      []map[string]interface{} `yaml:"fields,omitempty"`
}

// readWorkflowFile reads and unmarshals workflow.yaml from the given path.
// Handles both old list format (views: [...]) and new map format (views: {plugins: [...]}).
func readWorkflowFile(path string) (*workflowFileData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow.yaml: %w", err)
	}

	// convert legacy views list format to map format before unmarshaling
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing workflow.yaml: %w", err)
	}
	ConvertViewsListToMap(raw)
	data, err = yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling workflow.yaml: %w", err)
	}

	var wf workflowFileData
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing workflow.yaml: %w", err)
	}
	return &wf, nil
}

// ConvertViewsListToMap converts old views list format to new map format in-place.
// Old: views: [{name: Kanban, ...}]  →  New: views: {plugins: [{name: Kanban, ...}]}
func ConvertViewsListToMap(raw map[string]interface{}) {
	views, ok := raw["views"]
	if !ok {
		return
	}
	if _, isMap := views.(map[string]interface{}); isMap {
		return
	}
	if list, isList := views.([]interface{}); isList {
		raw["views"] = map[string]interface{}{
			"plugins": list,
		}
	}
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

	for _, p := range wf.Views.Plugins {
		if name, ok := p["name"].(string); ok && name == pluginName {
			if view, ok := p["view"].(string); ok && view != "" {
				return view
			}
		}
	}

	return defaultValue
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

// GetTheme returns the appearance theme setting
func GetTheme() string {
	theme := viper.GetString("appearance.theme")
	if theme == "" {
		return "auto"
	}
	return theme
}

var cachedEffectiveTheme string

// GetEffectiveTheme resolves "auto" to actual theme based on terminal detection.
// Uses termenv OSC 11 query to detect the terminal's actual background color,
// falling back to COLORFGBG env var, then dark.
// Result is cached — safe to call after tview takes over the terminal.
func GetEffectiveTheme() string {
	if cachedEffectiveTheme != "" {
		return cachedEffectiveTheme
	}
	theme := GetTheme()
	if theme != "auto" {
		cachedEffectiveTheme = theme
		return theme
	}
	output := termenv.NewOutput(os.Stdout)
	if output.HasDarkBackground() {
		cachedEffectiveTheme = "dark"
	} else {
		cachedEffectiveTheme = "light"
	}
	return cachedEffectiveTheme
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

// GetCodeBlockTheme returns the chroma syntax highlighting theme for code blocks.
// Defaults to the theme registry's chroma mapping when not explicitly configured.
func GetCodeBlockTheme() string {
	if t := viper.GetString("appearance.codeBlock.theme"); t != "" {
		return t
	}
	return ChromaThemeForEffective()
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
