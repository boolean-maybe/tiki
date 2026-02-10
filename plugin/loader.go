package plugin

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/boolean-maybe/tiki/config"
	"gopkg.in/yaml.v3"
)

// WorkflowFile represents the YAML structure of a workflow.yaml file
type WorkflowFile struct {
	Plugins []pluginFileConfig `yaml:"plugins"`
}

// loadConfiguredPlugins loads plugins defined in workflow.yaml
func loadConfiguredPlugins() []Plugin {
	workflowPath := config.FindWorkflowFile()
	if workflowPath == "" {
		slog.Debug("no workflow.yaml found")
		return nil
	}

	data, err := os.ReadFile(workflowPath)
	if err != nil {
		slog.Warn("failed to read workflow.yaml", "path", workflowPath, "error", err)
		return nil
	}

	var wf WorkflowFile
	if err := yaml.Unmarshal(data, &wf); err != nil {
		slog.Warn("failed to parse workflow.yaml", "path", workflowPath, "error", err)
		return nil
	}

	if len(wf.Plugins) == 0 {
		return nil
	}

	var plugins []Plugin
	for i, cfg := range wf.Plugins {
		if cfg.Name == "" {
			slog.Warn("skipping plugin with no name in workflow.yaml", "index", i)
			continue
		}

		source := fmt.Sprintf("%s:%s", workflowPath, cfg.Name)
		p, err := parsePluginConfig(cfg, source)
		if err != nil {
			slog.Warn("failed to load plugin from workflow.yaml", "name", cfg.Name, "error", err)
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
		slog.Info("loaded plugin", "name", p.GetName(), "key", keyName(pk, pr), "modifier", pm)
	}

	return plugins
}

// LoadPlugins loads all plugins: embedded defaults plus configured plugins from workflow.yaml.
// Configured plugins with the same name as embedded plugins will be merged (configured fields override embedded).
func LoadPlugins() ([]Plugin, error) {
	// load embedded default plugins first (maintains order)
	embedded := loadEmbeddedPlugins()
	embeddedByName := make(map[string]Plugin)
	for _, p := range embedded {
		embeddedByName[p.GetName()] = p
		pk, pr, pm := p.GetActivationKey()
		slog.Info("loaded embedded plugin", "name", p.GetName(), "key", keyName(pk, pr), "modifier", pm)
	}

	// load configured plugins (may override embedded ones)
	configured := loadConfiguredPlugins()

	// track which embedded plugins were overridden and merge them
	overridden := make(map[string]bool)
	mergedConfigured := make([]Plugin, 0, len(configured))

	for _, configPlugin := range configured {
		if embeddedPlugin, ok := embeddedByName[configPlugin.GetName()]; ok {
			// merge: embedded plugin fields + configured overrides
			merged := mergePluginDefinitions(embeddedPlugin, configPlugin)
			mergedConfigured = append(mergedConfigured, merged)
			overridden[configPlugin.GetName()] = true
			slog.Info("plugin override (merged)", "name", configPlugin.GetName(),
				"from", embeddedPlugin.GetFilePath(), "to", configPlugin.GetFilePath())
		} else {
			// new plugin (not an override)
			mergedConfigured = append(mergedConfigured, configPlugin)
		}
	}

	// build final list: non-overridden embedded plugins + merged configured plugins
	var plugins []Plugin
	for _, p := range embedded {
		if !overridden[p.GetName()] {
			plugins = append(plugins, p)
		}
	}
	plugins = append(plugins, mergedConfigured...)

	return plugins, nil
}
