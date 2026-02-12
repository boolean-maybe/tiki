package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"gopkg.in/yaml.v3"
)

// WorkflowFile represents the YAML structure of a workflow.yaml file
type WorkflowFile struct {
	Plugins []pluginFileConfig `yaml:"views"`
}

// loadPluginsFromFile loads plugins from a single workflow.yaml file.
// Returns the successfully loaded plugins and any validation errors encountered.
func loadPluginsFromFile(path string) ([]Plugin, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("failed to read workflow.yaml", "path", path, "error", err)
		return nil, []string{fmt.Sprintf("%s: %v", path, err)}
	}

	var wf WorkflowFile
	if err := yaml.Unmarshal(data, &wf); err != nil {
		slog.Warn("failed to parse workflow.yaml", "path", path, "error", err)
		return nil, []string{fmt.Sprintf("%s: %v", path, err)}
	}

	if len(wf.Plugins) == 0 {
		return nil, nil
	}

	var plugins []Plugin
	var errs []string
	for i, cfg := range wf.Plugins {
		if cfg.Name == "" {
			msg := fmt.Sprintf("%s: view at index %d has no name", path, i)
			slog.Warn("skipping plugin with no name in workflow.yaml", "index", i, "path", path)
			errs = append(errs, msg)
			continue
		}

		source := fmt.Sprintf("%s:%s", path, cfg.Name)
		p, err := parsePluginConfig(cfg, source)
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

	return plugins, errs
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
// Returns an error when workflow files were found but no valid plugins could be loaded.
func LoadPlugins() ([]Plugin, error) {
	files := config.FindWorkflowFiles()
	if len(files) == 0 {
		slog.Debug("no workflow.yaml files found")
		return nil, nil
	}

	var allErrors []string

	// First file is the base (typically user config)
	base, errs := loadPluginsFromFile(files[0])
	allErrors = append(allErrors, errs...)

	// Remaining files are overrides, merged in order
	for _, path := range files[1:] {
		overrides, errs := loadPluginsFromFile(path)
		allErrors = append(allErrors, errs...)
		if len(overrides) > 0 {
			base = mergePluginLists(base, overrides)
		}
	}

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
