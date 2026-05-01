package plugin

import (
	"github.com/gdamore/tcell/v2"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/ruki"
)

// ViewKind identifies the behavior of a view declared in workflow.yaml.
// Values are the canonical strings accepted in the `kind:` field.
type ViewKind string

const (
	KindBoard  ViewKind = "board"
	KindList   ViewKind = "list"
	KindWiki   ViewKind = "wiki"
	KindDetail ViewKind = "detail"
	KindSearch ViewKind = "search"

	// KindTimeline is reserved for a later phase; parser rejects it with a
	// dedicated "not yet implemented" error so users don't confuse the
	// rejection with the generic unknown-kind diagnostic.
	KindTimeline ViewKind = "timeline"
)

// IsValidKind reports whether s is a currently-implemented view kind.
// Reserved-but-unimplemented kinds (e.g. timeline, search) return false here
// and are handled by a dedicated rejection message.
func IsValidKind(s string) bool {
	switch ViewKind(s) {
	case KindBoard, KindList, KindWiki, KindDetail:
		return true
	}
	return false
}

// Plugin interface defines the common methods for all plugins
type Plugin interface {
	GetName() string
	GetLabel() string
	GetDescription() string
	GetActivationKey() (tcell.Key, rune, tcell.ModMask)
	GetFilePath() string
	GetConfigIndex() int
	GetKind() ViewKind
	IsDefault() bool
	// GetRequire returns the view's own `require:` list from workflow.yaml.
	// Consumed by the direct activation-key gate and `kind: view` action
	// dispatch so a view's declared requirements are honored regardless of
	// how navigation was triggered (6B.15).
	GetRequire() []string
}

// BasePlugin holds the common fields for all plugins
type BasePlugin struct {
	Name        string        // stable identifier (used by actions[].view, plugin-view-id)
	Label       string        // caption/display text (falls back to Name when empty)
	Description string        // short description shown in header info section
	Key         tcell.Key     // tcell key constant (e.g. KeyCtrlH)
	Rune        rune          // printable character (e.g. 'L')
	Modifier    tcell.ModMask // modifier keys (Alt, Shift, Ctrl, etc.)
	Foreground  config.Color  // caption text color
	Background  config.Color  // caption background color
	FilePath    string        // source file path (for error messages)
	ConfigIndex int           // index in workflow.yaml views array (-1 if not from a config file)
	Kind        ViewKind      // view kind: board, list, wiki, detail, search
	Default     bool          // true if this view should open on startup
	Require     []string      // view-level requirements (e.g. selection:one for detail)
}

func (p *BasePlugin) GetName() string {
	return p.Name
}

// GetLabel returns the display label, falling back to Name when unset.
func (p *BasePlugin) GetLabel() string {
	if p.Label != "" {
		return p.Label
	}
	return p.Name
}

func (p *BasePlugin) GetDescription() string {
	return p.Description
}

func (p *BasePlugin) GetActivationKey() (tcell.Key, rune, tcell.ModMask) {
	return p.Key, p.Rune, p.Modifier
}

func (p *BasePlugin) GetFilePath() string {
	return p.FilePath
}

func (p *BasePlugin) GetConfigIndex() int {
	return p.ConfigIndex
}

func (p *BasePlugin) GetKind() ViewKind {
	return p.Kind
}

func (p *BasePlugin) IsDefault() bool {
	return p.Default
}

// GetRequire returns the view-level require: list declared in workflow.yaml.
// Empty (nil or zero-length) means the view has no declared requirements.
func (p *BasePlugin) GetRequire() []string {
	return p.Require
}

// TikiPlugin backs board and list view kinds. The name is retained from the
// pre-unification codebase; Phase 6 keeps it to limit churn. New kinds share
// this struct — GetKind() distinguishes board vs list.
type TikiPlugin struct {
	BasePlugin
	Lanes   []TikiLane     // lane definitions for this plugin
	Mode    string         // display mode: "compact" or "expanded" (empty = compact)
	Actions []PluginAction // shortcut actions applied to the selected task
	TaskID  string         // optional tiki associated with this plugin (code-only, not from workflow config)
}

// DokiPlugin backs wiki and detail view kinds (markdown document rendering).
// For wiki views DocumentPath is set; for detail views it is empty and the
// rendered document follows the current selection.
//
// Note: DocumentID (ID-based resolution) is reserved for Phase 6B and is
// rejected by the parser today. The field is kept on the struct so Phase 6B
// can wire it in without another schema change.
type DokiPlugin struct {
	BasePlugin
	DocumentID   string // Phase 6B: resolve this ID through the document store
	DocumentPath string // wiki: relative path under .doc/
}

// PluginActionConfig represents a shortcut action in YAML or config definitions.
// A PluginActionConfig models either a ruki-executing action (Action is set)
// or a view-switching action (View is set). Exactly one must be set.
type PluginActionConfig struct {
	Key     string   `yaml:"key" mapstructure:"key"`
	Label   string   `yaml:"label" mapstructure:"label"`
	Kind    string   `yaml:"kind,omitempty" mapstructure:"kind"`
	Action  string   `yaml:"action" mapstructure:"action"`
	View    string   `yaml:"view,omitempty" mapstructure:"view"`
	Hot     *bool    `yaml:"hot,omitempty" mapstructure:"hot"`
	Input   string   `yaml:"input,omitempty" mapstructure:"input"`
	Require []string `yaml:"require,omitempty" mapstructure:"require"`
}

// ActionKind distinguishes ruki-executing actions from view-switching actions.
type ActionKind string

const (
	ActionKindRuki ActionKind = "ruki"
	ActionKindView ActionKind = "view"
)

// PluginAction represents a parsed shortcut action bound to a key.
// Kind selects which of Action (ruki) or TargetView (view navigation) is used.
type PluginAction struct {
	Key          tcell.Key
	Rune         rune
	Modifier     tcell.ModMask
	KeyStr       string // canonical key string for IDs, duplicate detection, and logging
	Label        string
	Kind         ActionKind
	Action       *ruki.ValidatedStatement
	TargetView   string // for Kind == ActionKindView: name of the view to open
	ShowInHeader bool
	InputType    ruki.ValueType
	HasInput     bool
	HasChoose    bool
	ChooseFilter *ruki.SubQuery
	Require      []string // effective requirements after auto-inference from id() usage
}

// PluginLaneConfig represents a lane in YAML or config definitions.
type PluginLaneConfig struct {
	Name    string `yaml:"name" mapstructure:"name"`
	Columns int    `yaml:"columns" mapstructure:"columns"`
	Width   int    `yaml:"width" mapstructure:"width"`
	Filter  string `yaml:"filter" mapstructure:"filter"`
	Action  string `yaml:"action" mapstructure:"action"`
}

// TikiLane represents a parsed lane definition.
type TikiLane struct {
	Name    string
	Columns int
	Width   int // lane width as a percentage (0 = equal share of remaining space)
	Filter  *ruki.ValidatedStatement
	Action  *ruki.ValidatedStatement
}
