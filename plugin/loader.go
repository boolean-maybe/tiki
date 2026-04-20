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

// viewsSectionConfig represents the YAML structure of the views section.
// views:
//
//	actions: [...]   # global plugin actions
//	plugins: [...]   # plugin definitions
type viewsSectionConfig struct {
	Actions []PluginActionConfig `yaml:"actions"`
	Plugins []pluginFileConfig   `yaml:"plugins"`
}

// pluginFileConfig represents the YAML structure of a plugin file
type pluginFileConfig struct {
	Name        string               `yaml:"name"`
	Description string               `yaml:"description"` // short description shown in header info
	Foreground  string               `yaml:"foreground"`  // hex color like "#ff0000" or named color
	Background  string               `yaml:"background"`
	Key         string               `yaml:"key"`  // single character
	Sort        string               `yaml:"sort"` // deprecated: only for deserializing old configs; converted to order-by and cleared by LegacyConfigTransformer
	View        string               `yaml:"view"` // "compact" or "expanded" (default: compact)
	Type        string               `yaml:"type"` // "tiki" or "doki" (default: tiki)
	Fetcher     string               `yaml:"fetcher"`
	Text        string               `yaml:"text"`
	URL         string               `yaml:"url"`
	Lanes       []PluginLaneConfig   `yaml:"lanes"`
	Actions     []PluginActionConfig `yaml:"actions"`
	Default     bool                 `yaml:"default"`
}

// WorkflowFile represents the YAML structure of a workflow.yaml file
type WorkflowFile struct {
	Version     string             `yaml:"version,omitempty"`
	Description string             `yaml:"description,omitempty"`
	Views       viewsSectionConfig `yaml:"views"`
}

// loadPluginsFromFile loads plugins from a single workflow.yaml file.
// Returns the successfully loaded plugins, parsed global actions, and any validation errors encountered.
func loadPluginsFromFile(path string, schema ruki.Schema) ([]Plugin, []PluginAction, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("failed to read workflow.yaml", "path", path, "error", err)
		return nil, nil, []string{fmt.Sprintf("%s: %v", path, err)}
	}

	// pre-process raw YAML to handle legacy views format (list → map)
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		slog.Warn("failed to parse workflow.yaml", "path", path, "error", err)
		return nil, nil, []string{fmt.Sprintf("%s: %v", path, err)}
	}
	transformer := NewLegacyConfigTransformer()
	transformer.ConvertViewsFormat(raw)
	normalizedData, err := yaml.Marshal(raw)
	if err != nil {
		slog.Warn("failed to re-marshal workflow.yaml after legacy conversion", "path", path, "error", err)
		return nil, nil, []string{fmt.Sprintf("%s: %v", path, err)}
	}

	var wf WorkflowFile
	if err := yaml.Unmarshal(normalizedData, &wf); err != nil {
		slog.Warn("failed to parse workflow.yaml", "path", path, "error", err)
		return nil, nil, []string{fmt.Sprintf("%s: %v", path, err)}
	}

	if len(wf.Views.Plugins) == 0 && len(wf.Views.Actions) == 0 {
		return nil, nil, nil
	}

	// convert legacy expressions to ruki before parsing
	totalConverted := 0
	for i := range wf.Views.Plugins {
		totalConverted += transformer.ConvertPluginConfig(&wf.Views.Plugins[i])
	}
	totalConverted += convertLegacyGlobalActions(transformer, wf.Views.Actions)
	if totalConverted > 0 {
		slog.Info("converted legacy workflow expressions to ruki", "count", totalConverted, "path", path)
	}

	var plugins []Plugin
	var errs []string
	for i, cfg := range wf.Views.Plugins {
		if cfg.Name == "" {
			msg := fmt.Sprintf("%s: view at index %d has no name", path, i)
			slog.Warn("skipping plugin with no name in workflow.yaml", "index", i, "path", path)
			errs = append(errs, msg)
			continue
		}

		source := fmt.Sprintf("%s:%s", path, cfg.Name)
		p, err := parsePluginConfig(cfg, source, schema)
		if err != nil {
			msg := fmt.Sprintf("%s: view %q: %v", path, cfg.Name, err)
			slog.Warn("failed to load plugin from workflow.yaml", "name", cfg.Name, "error", err)
			errs = append(errs, msg)
			continue
		}

		// set config index to position in workflow.yaml
		if tp, ok := p.(*TikiPlugin); ok {
			tp.ConfigIndex = i
		} else if dp, ok := p.(*DokiPlugin); ok {
			dp.ConfigIndex = i
		}

		plugins = append(plugins, p)
		pk, pr, pm := p.GetActivationKey()
		slog.Info("loaded plugin", "name", p.GetName(), "path", path, "key", keyName(pk, pr), "modifier", pm)
	}

	// parse global plugin actions
	var globalActions []PluginAction
	if len(wf.Views.Actions) > 0 {
		parser := ruki.NewParser(schema)
		parsed, err := parsePluginActions(wf.Views.Actions, parser)
		if err != nil {
			slog.Warn("failed to parse global plugin actions", "path", path, "error", err)
			errs = append(errs, fmt.Sprintf("%s: global actions: %v", path, err))
		} else {
			globalActions = parsed
			slog.Info("loaded global plugin actions", "count", len(globalActions), "path", path)
		}
	}

	return plugins, globalActions, errs
}

// LoadPlugins loads plugins from the single highest-priority workflow.yaml file.
// Returns an error when the workflow file was found but no valid plugins could be loaded,
// or when type-reference errors indicate an inconsistent workflow.
func LoadPlugins(schema ruki.Schema) ([]Plugin, error) {
	files := config.FindWorkflowFiles()
	if len(files) == 0 {
		slog.Debug("no workflow.yaml files found")
		return nil, nil
	}

	path := files[0]
	plugins, globalActions, errs := loadPluginsFromFile(path, schema)

	if typeErrs := filterTypeErrors(errs); len(typeErrs) > 0 {
		return nil, fmt.Errorf("workflow references invalid types:\n  %s",
			strings.Join(typeErrs, "\n  "))
	}

	mergeGlobalActionsIntoPlugins(plugins, globalActions)

	if len(plugins) == 0 {
		if len(errs) > 0 {
			return nil, fmt.Errorf("no valid views loaded:\n  %s\n\nTo install fresh defaults, remove the workflow file and restart tiki:\n\n  rm %s",
				strings.Join(errs, "\n  "), path)
		}
		return nil, fmt.Errorf("no views defined in %s\n\nTo install fresh defaults, remove the workflow file and restart tiki:\n\n  rm %s",
			path, path)
	}

	return plugins, nil
}

// filterTypeErrors extracts errors that mention unknown type references.
func filterTypeErrors(errs []string) []string {
	var typeErrs []string
	for _, e := range errs {
		if strings.Contains(e, "unknown type") {
			typeErrs = append(typeErrs, e)
		}
	}
	return typeErrs
}

// mergeGlobalActionsIntoPlugins appends global actions to each TikiPlugin.
// Per-plugin actions with the same KeyStr take precedence over globals (global is skipped).
func mergeGlobalActionsIntoPlugins(plugins []Plugin, globalActions []PluginAction) {
	if len(globalActions) == 0 {
		return
	}
	for _, p := range plugins {
		tp, ok := p.(*TikiPlugin)
		if !ok {
			continue
		}
		localKeys := make(map[string]bool, len(tp.Actions))
		for _, a := range tp.Actions {
			localKeys[a.KeyStr] = true
		}
		for _, ga := range globalActions {
			if localKeys[ga.KeyStr] {
				slog.Info("per-plugin action overrides global action",
					"plugin", tp.Name, "key", ga.KeyStr, "global_label", ga.Label)
				continue
			}
			tp.Actions = append(tp.Actions, ga)
		}
	}
}

// convertLegacyGlobalActions converts legacy action expressions in global views.actions
// to ruki format, matching the same conversion applied to per-plugin actions.
func convertLegacyGlobalActions(transformer *LegacyConfigTransformer, actions []PluginActionConfig) int {
	count := 0
	for i := range actions {
		action := &actions[i]
		if action.Action != "" && !isRukiAction(action.Action) {
			newAction, err := transformer.ConvertAction(action.Action)
			if err != nil {
				slog.Warn("failed to convert legacy global action, passing through",
					"error", err, "action", action.Action, "key", action.Key)
				continue
			}
			slog.Debug("converted legacy global action", "old", action.Action, "new", newAction, "key", action.Key)
			action.Action = newAction
			count++
		}
	}
	return count
}

// DefaultPlugin returns the first plugin marked as default, or the first plugin
// in the list if none are marked. The caller must ensure plugins is non-empty.
func DefaultPlugin(plugins []Plugin) Plugin {
	for _, p := range plugins {
		if p.IsDefault() {
			return p
		}
	}
	return plugins[0]
}
