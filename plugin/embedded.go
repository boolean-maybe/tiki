package plugin

import (
	_ "embed"
	"log/slog"

	"gopkg.in/yaml.v3"
)

//go:embed embed/workflow.yaml
var embeddedWorkflowYAML string

// loadEmbeddedPlugins loads the built-in default plugins from the embedded workflow.yaml
func loadEmbeddedPlugins() []Plugin {
	var wf WorkflowFile
	if err := yaml.Unmarshal([]byte(embeddedWorkflowYAML), &wf); err != nil {
		slog.Error("failed to parse embedded workflow.yaml", "error", err)
		return nil
	}

	var plugins []Plugin
	for _, cfg := range wf.Plugins {
		if cfg.Name == "" {
			slog.Warn("skipping embedded plugin with no name")
			continue
		}

		p, err := parsePluginConfig(cfg, "embedded:"+cfg.Name)
		if err != nil {
			slog.Error("failed to parse embedded plugin", "name", cfg.Name, "error", err)
			continue
		}

		// mark as embedded (not from user config)
		switch plugin := p.(type) {
		case *TikiPlugin:
			plugin.ConfigIndex = -1
		case *DokiPlugin:
			plugin.ConfigIndex = -1
		}

		plugins = append(plugins, p)
	}

	return plugins
}
