package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/ruki"
	"gopkg.in/yaml.v3"
)

// pluginFileConfig represents the YAML structure of a single view definition.
// Field-level legacy detection happens in rejectLegacyTopLevel so user-visible
// errors point at the specific field that changed.
type pluginFileConfig struct {
	Name        string               `yaml:"name"`
	Label       string               `yaml:"label"`
	Description string               `yaml:"description"`
	Foreground  string               `yaml:"foreground"`
	Background  string               `yaml:"background"`
	Key         string               `yaml:"key"`
	Kind        string               `yaml:"kind"`
	Mode        string               `yaml:"mode"`
	Document    string               `yaml:"document"`
	Path        string               `yaml:"path"`
	Lanes       []PluginLaneConfig   `yaml:"lanes"`
	Actions     []PluginActionConfig `yaml:"actions"`
	Require     []string             `yaml:"require"`
	Default     bool                 `yaml:"default"`

	// Legacy fields retained only for rejection diagnostics.
	Type    string `yaml:"type"`
	View    string `yaml:"view"`
	Fetcher string `yaml:"fetcher"`
	Text    string `yaml:"text"`
	URL     string `yaml:"url"`
	Sort    string `yaml:"sort"`
}

// WorkflowFile represents the new Phase-6 YAML structure.
// views: and actions: are top-level; the old views.plugins wrapper is rejected.
type WorkflowFile struct {
	Version     string               `yaml:"version,omitempty"`
	Description string               `yaml:"description,omitempty"`
	Views       []pluginFileConfig   `yaml:"views"`
	Actions     []PluginActionConfig `yaml:"actions"`
}

// loadPluginsFromFile loads plugins from a single workflow.yaml file.
// Returns the successfully loaded plugins, parsed global actions, and any
// validation errors encountered.
func loadPluginsFromFile(path string, schema ruki.Schema) ([]Plugin, []PluginAction, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("failed to read workflow.yaml", "path", path, "error", err)
		return nil, nil, []string{fmt.Sprintf("%s: %v", path, err)}
	}

	// detect legacy top-level shapes before unmarshaling so users get a clear
	// "your workflow uses the old schema" message instead of a YAML-level confusion.
	if msg := detectLegacyTopLevelShape(data); msg != "" {
		return nil, nil, []string{fmt.Sprintf("%s: %s", path, msg)}
	}

	var wf WorkflowFile
	if err := yaml.Unmarshal(data, &wf); err != nil {
		slog.Warn("failed to parse workflow.yaml", "path", path, "error", err)
		return nil, nil, []string{fmt.Sprintf("%s: %v", path, err)}
	}

	if len(wf.Views) == 0 && len(wf.Actions) == 0 {
		return nil, nil, nil
	}

	// first pass: collect view names for cross-validation of kind:view actions
	viewNames, firstPassErrs := collectViewNames(wf.Views, path)

	// second pass: parse each view
	var plugins []Plugin
	errs := append([]string{}, firstPassErrs...)
	for i, cfg := range wf.Views {
		if cfg.Name == "" {
			// already reported in first pass
			continue
		}
		source := fmt.Sprintf("%s:%s", path, cfg.Name)
		p, err := parsePluginConfig(cfg, source, schema, viewNames)
		if err != nil {
			msg := fmt.Sprintf("%s: view %q: %v", path, cfg.Name, err)
			slog.Warn("failed to load plugin from workflow.yaml", "name", cfg.Name, "error", err)
			errs = append(errs, msg)
			continue
		}

		setConfigIndex(p, i)
		plugins = append(plugins, p)

		pk, pr, pm := p.GetActivationKey()
		slog.Info("loaded plugin", "name", p.GetName(), "path", path, "key", keyName(pk, pr), "modifier", pm)
	}

	if err := validateSingleDefault(plugins); err != nil {
		errs = append(errs, fmt.Sprintf("%s: %v", path, err))
	}

	// parse global actions with access to view names for kind:view validation
	globalActions, err := parseGlobalActions(wf.Actions, schema, viewNames)
	if err != nil {
		slog.Warn("failed to parse global actions", "path", path, "error", err)
		errs = append(errs, fmt.Sprintf("%s: global actions: %v", path, err))
	} else if len(globalActions) > 0 {
		slog.Info("loaded global actions", "count", len(globalActions), "path", path)
	}

	return plugins, globalActions, errs
}

// detectLegacyTopLevelShape returns a non-empty diagnostic when the workflow
// file uses a pre-Phase-6 top-level shape. Pre-unmarshal so the error is
// specific rather than a cryptic yaml type mismatch.
func detectLegacyTopLevelShape(data []byte) string {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ""
	}
	views, ok := raw["views"]
	if !ok {
		return ""
	}
	// old list-at-top form: views: [ {name: ..., type: tiki}, ... ]
	if _, isList := views.([]interface{}); isList {
		// the new shape is also a list, but the distinguishing marker of the
		// *legacy* shape is the presence of `type:` or `fetcher:` on any
		// element. The parser-level rejection catches those cleanly; we only
		// flag here if `views` is wrapped in a map with a `plugins:` key
		// (the other old form).
		return ""
	}
	viewsMap, isMap := views.(map[string]interface{})
	if !isMap {
		return ""
	}
	if _, hasPlugins := viewsMap["plugins"]; hasPlugins {
		return "`views:` must be a top-level list — the `views.plugins` wrapper is no longer supported. " +
			"Move views to a top-level `views: [...]` list and move global actions to a top-level `actions: [...]` list."
	}
	return ""
}

// collectViewNames walks views once to build the set of unique names. Missing
// or duplicate names are reported as errors so the second pass can skip them.
func collectViewNames(views []pluginFileConfig, path string) (map[string]struct{}, []string) {
	names := make(map[string]struct{}, len(views))
	var errs []string
	for i, cfg := range views {
		if cfg.Name == "" {
			errs = append(errs, fmt.Sprintf("%s: view at index %d has no name", path, i))
			continue
		}
		if _, dup := names[cfg.Name]; dup {
			errs = append(errs, fmt.Sprintf("%s: duplicate view name %q", path, cfg.Name))
			continue
		}
		names[cfg.Name] = struct{}{}
	}
	return names, errs
}

// setConfigIndex records the view's position in the workflow file.
func setConfigIndex(p Plugin, i int) {
	switch v := p.(type) {
	case *TikiPlugin:
		v.ConfigIndex = i
	case *DokiPlugin:
		v.ConfigIndex = i
	}
}

// validateSingleDefault enforces the "at most one default: true view" rule.
func validateSingleDefault(plugins []Plugin) error {
	defaults := make([]string, 0, 1)
	for _, p := range plugins {
		if p.IsDefault() {
			defaults = append(defaults, p.GetName())
		}
	}
	if len(defaults) > 1 {
		return fmt.Errorf("multiple views marked `default: true`: %s — at most one is allowed",
			strings.Join(defaults, ", "))
	}
	return nil
}

// parseGlobalActions validates and parses the top-level actions list.
func parseGlobalActions(configs []PluginActionConfig, schema ruki.Schema, viewNames map[string]struct{}) ([]PluginAction, error) {
	if len(configs) == 0 {
		return nil, nil
	}
	parser := ruki.NewParser(schema)
	return parsePluginActions(configs, parser, viewNames)
}

// LoadPlugins loads plugins from the single highest-priority workflow.yaml file.
// Returns an error when the workflow file was found but no valid plugins could
// be loaded, or when type-reference errors indicate an inconsistent workflow.
// For callers that also need the top-level global actions, use
// LoadPluginsAndGlobals.
func LoadPlugins(schema ruki.Schema) ([]Plugin, error) {
	plugins, _, err := LoadPluginsAndGlobals(schema)
	return plugins, err
}

// LoadPluginsAndGlobals returns both the parsed views and the workflow's
// top-level global actions. Globals are also merged into every board/list
// view's per-view Actions slice (legacy behavior); non-board controllers
// receive them directly via this return value.
//
// Load is fail-closed: any parse error — duplicate view name, invalid lane,
// bogus require token, malformed global action, legacy field — fails the
// whole load. A partial workflow would diverge from what the user declared,
// so we refuse to boot on a broken file.
func LoadPluginsAndGlobals(schema ruki.Schema) ([]Plugin, []PluginAction, error) {
	files := config.FindWorkflowFiles()
	if len(files) == 0 {
		slog.Debug("no workflow.yaml files found")
		return nil, nil, nil
	}

	path := files[0]
	plugins, globalActions, errs := loadPluginsFromFile(path, schema)

	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("workflow did not load cleanly:\n  %s\n\nFix the reported issues, or remove the workflow file and restart tiki to reinstall defaults:\n\n  rm %s",
			strings.Join(errs, "\n  "), path)
	}

	mergeGlobalActionsIntoPlugins(plugins, globalActions)

	if len(plugins) == 0 {
		return nil, nil, fmt.Errorf("no views defined in %s\n\nTo install fresh defaults, remove the workflow file and restart tiki:\n\n  rm %s",
			path, path)
	}

	return plugins, globalActions, nil
}

// mergeGlobalActionsIntoPlugins appends global actions to every view that can
// host them. Per-view actions with the same KeyStr take precedence — the
// global is skipped for that view.
func mergeGlobalActionsIntoPlugins(plugins []Plugin, globalActions []PluginAction) {
	if len(globalActions) == 0 {
		return
	}
	for _, p := range plugins {
		tp, ok := p.(*TikiPlugin)
		if !ok {
			// Non-board/list views receive globals through the action
			// registry at runtime; they have no per-view Actions slice.
			continue
		}
		localKeys := make(map[string]bool, len(tp.Actions))
		for _, a := range tp.Actions {
			localKeys[a.KeyStr] = true
		}
		for _, ga := range globalActions {
			if localKeys[ga.KeyStr] {
				slog.Info("per-view action overrides global action",
					"view", tp.Name, "key", ga.KeyStr, "global_label", ga.Label)
				continue
			}
			tp.Actions = append(tp.Actions, ga)
		}
	}
}

// LoadPluginsFromFile validates and loads plugins from an explicit workflow
// file path using the provided schema. Fail-closed: any parse error fails
// the load. Used by init to validate a candidate workflow file without
// global path discovery.
func LoadPluginsFromFile(path string, schema ruki.Schema) ([]Plugin, error) {
	plugins, globalActions, errs := loadPluginsFromFile(path, schema)

	if len(errs) > 0 {
		return nil, fmt.Errorf("workflow did not load cleanly:\n  %s",
			strings.Join(errs, "\n  "))
	}

	mergeGlobalActionsIntoPlugins(plugins, globalActions)

	if len(plugins) == 0 {
		return nil, fmt.Errorf("no views defined in %s", path)
	}

	return plugins, nil
}

// DefaultPlugin returns the first plugin marked as default, or the first
// plugin in the list if none are marked. The caller must ensure plugins is
// non-empty.
func DefaultPlugin(plugins []Plugin) Plugin {
	for _, p := range plugins {
		if p.IsDefault() {
			return p
		}
	}
	return plugins[0]
}

// GlobalActions returns the top-level actions parsed from a workflow file.
// Exposed for controllers/views that are not board/list and therefore don't
// receive globals via mergeGlobalActionsIntoPlugins.
func GlobalActions(schema ruki.Schema) ([]PluginAction, error) {
	files := config.FindWorkflowFiles()
	if len(files) == 0 {
		return nil, nil
	}
	_, actions, errs := loadPluginsFromFile(files[0], schema)
	if len(errs) > 0 {
		slog.Debug("workflow load warnings while extracting globals", "count", len(errs))
	}
	return actions, nil
}
