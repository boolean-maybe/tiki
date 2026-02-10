package plugin

import (
	"github.com/gdamore/tcell/v2"

	"github.com/boolean-maybe/tiki/plugin/filter"
)

// Plugin interface defines the common methods for all plugins
type Plugin interface {
	GetName() string
	GetActivationKey() (tcell.Key, rune, tcell.ModMask)
	GetForeground() tcell.Color
	GetBackground() tcell.Color
	GetFilePath() string
	GetConfigIndex() int
	GetType() string
}

// BasePlugin holds the common fields for all plugins
type BasePlugin struct {
	Name        string        // display name shown in caption
	Key         tcell.Key     // tcell key constant (e.g. KeyCtrlH)
	Rune        rune          // printable character (e.g. 'L')
	Modifier    tcell.ModMask // modifier keys (Alt, Shift, Ctrl, etc.)
	Foreground  tcell.Color   // caption text color
	Background  tcell.Color   // caption background color
	FilePath    string        // source file path (for error messages)
	ConfigIndex int           // index in config.yaml plugins array (-1 if embedded/not in config)
	Type        string        // plugin type: "tiki" or "doki"
}

func (p *BasePlugin) GetName() string {
	return p.Name
}

func (p *BasePlugin) GetActivationKey() (tcell.Key, rune, tcell.ModMask) {
	return p.Key, p.Rune, p.Modifier
}

func (p *BasePlugin) GetForeground() tcell.Color {
	return p.Foreground
}

func (p *BasePlugin) GetBackground() tcell.Color {
	return p.Background
}

func (p *BasePlugin) GetFilePath() string {
	return p.FilePath
}

func (p *BasePlugin) GetConfigIndex() int {
	return p.ConfigIndex
}

func (p *BasePlugin) GetType() string {
	return p.Type
}

// TikiPlugin is a task-based plugin (like default Kanban board)
type TikiPlugin struct {
	BasePlugin
	Lanes    []TikiLane     // lane definitions for this plugin
	Sort     []SortRule     // parsed sort rules (nil = default sort)
	ViewMode string         // default view mode: "compact" or "expanded" (empty = compact)
	Actions  []PluginAction // shortcut actions applied to the selected task
}

// DokiPlugin is a documentation-based plugin
type DokiPlugin struct {
	BasePlugin
	Fetcher string // "file" or "internal"
	Text    string // content text (for internal)
	URL     string // resource URL (for file)
}

// PluginActionConfig represents a shortcut action in YAML or config definitions.
type PluginActionConfig struct {
	Key    string `yaml:"key" mapstructure:"key"`
	Label  string `yaml:"label" mapstructure:"label"`
	Action string `yaml:"action" mapstructure:"action"`
}

// PluginAction represents a parsed shortcut action bound to a key.
type PluginAction struct {
	Rune   rune
	Label  string
	Action LaneAction
}

// PluginLaneConfig represents a lane in YAML or config definitions.
type PluginLaneConfig struct {
	Name    string `yaml:"name" mapstructure:"name"`
	Columns int    `yaml:"columns" mapstructure:"columns"`
	Filter  string `yaml:"filter" mapstructure:"filter"`
	Action  string `yaml:"action" mapstructure:"action"`
}

// TikiLane represents a parsed lane definition.
type TikiLane struct {
	Name    string
	Columns int
	Filter  filter.FilterExpr
	Action  LaneAction
}
