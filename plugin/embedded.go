package plugin

import (
	_ "embed"
	"log/slog"
)

//go:embed embed/kanban.yaml
var kanbanYAML string

//go:embed embed/recent.yaml
var recentYAML string

//go:embed embed/roadmap.yaml
var roadmapYAML string

//go:embed embed/backlog.yaml
var backlogYAML string

//go:embed embed/help.yaml
var helpYAML string

//go:embed embed/documentation.yaml
var documentationYAML string

// loadEmbeddedPlugin parses a single embedded plugin and sets its ConfigIndex to -1
func loadEmbeddedPlugin(yamlContent string, sourceName string) Plugin {
	p, err := parsePluginYAML([]byte(yamlContent), sourceName)
	if err != nil {
		slog.Error("failed to parse embedded plugin", "source", sourceName, "error", err)
		return nil
	}

	// Set ConfigIndex = -1 for both TikiPlugin and DokiPlugin
	switch plugin := p.(type) {
	case *TikiPlugin:
		plugin.ConfigIndex = -1
	case *DokiPlugin:
		plugin.ConfigIndex = -1
	}

	return p
}

// loadEmbeddedPlugins loads the built-in default plugins (Kanban, Backlog, Recent, Roadmap, Help, and Documentation)
func loadEmbeddedPlugins() []Plugin {
	var plugins []Plugin

	// Define embedded plugins with their YAML content and source names
	// Kanban is first so it becomes the default view
	embeddedPlugins := []struct {
		yaml   string
		source string
	}{
		{kanbanYAML, "embedded:kanban"},
		{backlogYAML, "embedded:backlog"},
		{recentYAML, "embedded:recent"},
		{roadmapYAML, "embedded:roadmap"},
		{helpYAML, "embedded:help"},
		{documentationYAML, "embedded:documentation"},
	}

	// Load each embedded plugin
	for _, ep := range embeddedPlugins {
		if p := loadEmbeddedPlugin(ep.yaml, ep.source); p != nil {
			plugins = append(plugins, p)
		}
	}

	return plugins
}
