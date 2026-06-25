package plugin

import (
	"github.com/gdamore/tcell/v2"

	"github.com/boolean-maybe/ruki"
	"github.com/boolean-maybe/tiki/gridlayout"
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

// WorkflowPlugin backs board and list view kinds. New kinds share this struct —
// GetKind() distinguishes board vs list.
type WorkflowPlugin struct {
	BasePlugin
	Lanes   []TikiLane          // lane definitions for this plugin
	Layout  gridlayout.GridSpec // parsed layout grid for tiki-box rendering (see workflow-format.md)
	Actions []PluginAction      // shortcut actions applied to the selected tiki
}

// WikiPlugin backs the wiki view kind (markdown document rendering bound to a
// specific document at `path:`).
//
// Note: DocumentID (ID-based resolution) is reserved for a later phase and is
// rejected by the parser today. The field is kept on the struct so the future
// implementation can wire it in without another schema change.
type WikiPlugin struct {
	BasePlugin
	DocumentID   string // future: resolve this ID through the document store
	DocumentPath string // wiki: relative path under .doc/
}

// DetailPlugin backs the configurable detail view kind. Layout is the
// parsed 2D layout grid for the box between the title and description
// sections. Per-view Actions are surfaced alongside built-in detail
// actions and global actions.
type DetailPlugin struct {
	BasePlugin
	Layout  gridlayout.GridSpec // parsed layout grid (see workflow-format.md)
	Actions []PluginAction      // per-view shortcut actions (merged with globals at runtime)
}

// PluginActionConfig represents a shortcut action in YAML or config definitions.
// A PluginActionConfig models either a ruki-executing action (Action is set)
// or a view-switching action (View is set). Exactly one must be set.
//
// Choose is optional on kind: view actions only. When set, it carries a bare
// ruki "select [where <cond>]" that produces the candidate set for a
// QuickSelect picker; the chosen tiki id is then carried into the target
// view's navigation params instead of the source view's cursor selection.
type PluginActionConfig struct {
	Key     string   `yaml:"key" mapstructure:"key"`
	Label   string   `yaml:"label" mapstructure:"label"`
	Kind    string   `yaml:"kind,omitempty" mapstructure:"kind"`
	Mode    string   `yaml:"mode,omitempty" mapstructure:"mode"`
	Action  string   `yaml:"action" mapstructure:"action"`
	View    string   `yaml:"view,omitempty" mapstructure:"view"`
	Choose  string   `yaml:"choose,omitempty" mapstructure:"choose"`
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

// DetailMode is the closed vocabulary for the optional `mode:` field on
// kind: view actions targeting a kind: detail view. View opens read-only,
// edit / edit-desc / edit-tags enter in-place edit mode focused on a
// specific field, and new synthesizes a draft tiki and enters edit mode
// focused on Title.
type DetailMode string

const (
	DetailModeView     DetailMode = "view"
	DetailModeEdit     DetailMode = "edit"
	DetailModeNew      DetailMode = "new"
	DetailModeEditDesc DetailMode = "edit-desc"
	DetailModeEditTags DetailMode = "edit-tags"
)

// SuppliesOwnSubject reports whether the mode produces its own tiki rather
// than operating on the carried selection. Only mode: new does — it
// synthesizes a fresh draft (see buildDetailViewParams). Such a mode must
// not be gated on the target view's selection:one requirement, otherwise
// "New" can never fire on an empty board (no selection to satisfy it).
func (m DetailMode) SuppliesOwnSubject() bool {
	return m == DetailModeNew
}

// PluginAction represents a parsed shortcut action bound to a key.
// Kind selects which of Action (ruki) or TargetView (view navigation) is used.
type PluginAction struct {
	Key          tcell.Key
	Rune         rune
	Modifier     tcell.ModMask
	KeyStr       string // canonical key string for IDs, duplicate detection, and logging
	Label        string
	Kind         ActionKind
	Mode         DetailMode
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
