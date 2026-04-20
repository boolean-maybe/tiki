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

// mergePluginLists merges override plugins on top of base plugins.
// Overrides with the same name as base plugins are merged; new overrides are appended.
func mergePluginLists(base, overrides []Plugin) []Plugin {
	baseByName := make(map[string]Plugin)
	for _, p := range base {
		baseByName[p.GetName()] = p
	}

	overridden := make(map[string]bool)
	mergedOverrides := make([]Plugin, 0, len(overrides))

	for _, overridePlugin := range overrides {
		if basePlugin, ok := baseByName[overridePlugin.GetName()]; ok {
			merged := mergePluginDefinitions(basePlugin, overridePlugin)
			mergedOverrides = append(mergedOverrides, merged)
			overridden[overridePlugin.GetName()] = true
			slog.Info("plugin override (merged)", "name", overridePlugin.GetName(),
				"from", basePlugin.GetFilePath(), "to", overridePlugin.GetFilePath())
		} else {
			mergedOverrides = append(mergedOverrides, overridePlugin)
		}
	}

	// Build final list: non-overridden base plugins + merged overrides
	var result []Plugin
	for _, p := range base {
		if !overridden[p.GetName()] {
			result = append(result, p)
		}
	}
	result = append(result, mergedOverrides...)

	return result
}

// LoadPlugins loads all plugins from workflow.yaml files: user config (base) + project config (overrides).
// Files are discovered via config.FindWorkflowFiles() which returns user config first, then project config.
// Plugins from later files override same-named plugins from earlier files via field merging.
// Global actions are merged by key across files (later files override same-keyed globals from earlier files).
// Returns an error when workflow files were found but no valid plugins could be loaded,
// or when type-reference errors indicate an inconsistent merged workflow.
func LoadPlugins(schema ruki.Schema) ([]Plugin, error) {
	files := config.FindWorkflowFiles()
	if len(files) == 0 {
		slog.Debug("no workflow.yaml files found")
		return nil, nil
	}

	var allErrors []string
	var allGlobalActions []PluginAction

	// First file is the base (typically user config)
	base, globalActions, errs := loadPluginsFromFile(files[0], schema)
	allErrors = append(allErrors, errs...)
	allGlobalActions = append(allGlobalActions, globalActions...)

	// Remaining files are overrides, merged in order
	for _, path := range files[1:] {
		overrides, moreGlobals, errs := loadPluginsFromFile(path, schema)
		allErrors = append(allErrors, errs...)
		if len(overrides) > 0 {
			base = mergePluginLists(base, overrides)
		}
		allGlobalActions = mergeGlobalActions(allGlobalActions, moreGlobals)
	}

	// type-reference errors in views/actions are fatal merged-workflow errors,
	// not ordinary per-view parse errors that can be skipped
	if typeErrs := filterTypeErrors(allErrors); len(typeErrs) > 0 {
		return nil, fmt.Errorf("merged workflow references invalid types:\n  %s\n\nIf you redefined types: in a later workflow file, update views/actions/triggers to match",
			strings.Join(typeErrs, "\n  "))
	}

	// merge global actions into each TikiPlugin
	mergeGlobalActionsIntoPlugins(base, allGlobalActions)

	if len(base) == 0 {
		if len(allErrors) > 0 {
			return nil, fmt.Errorf("no valid views loaded:\n  %s\n\nTo install fresh defaults, remove the workflow file(s) and restart tiki:\n\n  rm %s",
				strings.Join(allErrors, "\n  "),
				strings.Join(files, "\n  rm "))
		}
		return nil, fmt.Errorf("no views defined in %s\n\nTo install fresh defaults, remove the workflow file(s) and restart tiki:\n\n  rm %s",
			strings.Join(files, ", "),
			strings.Join(files, "\n  rm "))
	}

	return base, nil
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

// mergeGlobalActions merges override global actions into base by canonical KeyStr.
// Overrides with the same KeyStr replace the base action.
func mergeGlobalActions(base, overrides []PluginAction) []PluginAction {
	if len(overrides) == 0 {
		return base
	}
	byKeyStr := make(map[string]int, len(base))
	result := make([]PluginAction, len(base))
	copy(result, base)
	for i, a := range result {
		byKeyStr[a.KeyStr] = i
	}
	for _, o := range overrides {
		if idx, ok := byKeyStr[o.KeyStr]; ok {
			result[idx] = o
		} else {
			byKeyStr[o.KeyStr] = len(result)
			result = append(result, o)
		}
	}
	return result
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
