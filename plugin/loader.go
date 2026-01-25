package plugin

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// loadConfiguredPlugins loads plugins defined in config.yaml
func loadConfiguredPlugins() []Plugin {
	// Get plugin refs from config.yaml
	var refs []PluginRef
	if err := viper.UnmarshalKey("plugins", &refs); err != nil {
		// Not an error if plugins key doesn't exist
		slog.Debug("no plugins configured or failed to parse", "error", err)
		return nil
	}

	if len(refs) == 0 {
		return nil // no plugins configured
	}

	var plugins []Plugin

	for i, ref := range refs {
		// Validate before loading
		if err := validatePluginRef(ref); err != nil {
			slog.Warn("invalid plugin configuration", "error", err)
			continue
		}

		plugin, err := loadPluginFromRef(ref)
		if err != nil {
			slog.Warn("failed to load plugin", "name", ref.Name, "file", ref.File, "error", err)
			continue // Skip failed plugins, continue with others
		}

		// Set config index (need type assertion or helper)
		if p, ok := plugin.(*TikiPlugin); ok {
			p.ConfigIndex = i
		} else if p, ok := plugin.(*DokiPlugin); ok {
			p.ConfigIndex = i
		}

		plugins = append(plugins, plugin)
		pk, pr, pm := plugin.GetActivationKey()
		slog.Info("loaded plugin", "name", plugin.GetName(), "key", keyName(pk, pr), "modifier", pm)
	}

	return plugins
}

// LoadPlugins loads all plugins: embedded defaults (Recent, Roadmap) plus configured plugins from config.yaml
// Configured plugins with the same name as embedded plugins will be merged (configured fields override embedded)
func LoadPlugins() ([]Plugin, error) {
	// Load embedded default plugins first (maintains order)
	embedded := loadEmbeddedPlugins()
	embeddedByName := make(map[string]Plugin)
	for _, p := range embedded {
		embeddedByName[p.GetName()] = p
		pk, pr, pm := p.GetActivationKey()
		slog.Info("loaded embedded plugin", "name", p.GetName(), "key", keyName(pk, pr), "modifier", pm)
	}

	// Load configured plugins (may override embedded ones)
	configured := loadConfiguredPlugins()

	// Track which embedded plugins were overridden and merge them
	overridden := make(map[string]bool)
	mergedConfigured := make([]Plugin, 0, len(configured))

	for _, configPlugin := range configured {
		if embeddedPlugin, ok := embeddedByName[configPlugin.GetName()]; ok {
			// Merge: embedded plugin fields + configured overrides
			merged := mergePluginDefinitions(embeddedPlugin, configPlugin)
			mergedConfigured = append(mergedConfigured, merged)
			overridden[configPlugin.GetName()] = true
			slog.Info("plugin override (merged)", "name", configPlugin.GetName(),
				"from", embeddedPlugin.GetFilePath(), "to", configPlugin.GetFilePath())
		} else {
			// New plugin (not an override)
			mergedConfigured = append(mergedConfigured, configPlugin)
		}
	}

	// Build final list: non-overridden embedded plugins + merged configured plugins
	// This preserves order: embedded plugins first (in their original order), then configured
	var plugins []Plugin
	for _, p := range embedded {
		if !overridden[p.GetName()] {
			plugins = append(plugins, p)
		}
	}
	plugins = append(plugins, mergedConfigured...)

	return plugins, nil
}

// loadPluginFromRef loads a single plugin from a PluginRef, handling three modes:
// 1. Fully inline (no file): all fields in config.yaml
// 2. File-based (file only): reference external YAML
// 3. Hybrid (file + overrides): file provides base, inline overrides
func loadPluginFromRef(ref PluginRef) (Plugin, error) {
	var cfg pluginFileConfig
	var source string

	if ref.File != "" {
		// File-based or hybrid mode
		pluginPath := findPluginFile(ref.File)
		if pluginPath == "" {
			return nil, fmt.Errorf("plugin file not found: %s", ref.File)
		}

		data, err := os.ReadFile(pluginPath)
		if err != nil {
			return nil, fmt.Errorf("reading file: %w", err)
		}

		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing yaml: %w", err)
		}

		source = pluginPath

		// Apply inline overrides
		cfg = mergePluginConfigs(cfg, ref)
	} else {
		// Fully inline mode
		cfg = pluginFileConfig{
			Name:       ref.Name,
			Foreground: ref.Foreground,
			Background: ref.Background,
			Key:        ref.Key,
			Filter:     ref.Filter,
			Sort:       ref.Sort,
			View:       ref.View,
			Type:       ref.Type,
			Fetcher:    ref.Fetcher,
			Text:       ref.Text,
			URL:        ref.URL,
			Panes:      ref.Panes,
		}
		source = "inline:" + ref.Name
	}

	// Validate: must have name
	if cfg.Name == "" {
		return nil, fmt.Errorf("plugin must have a name")
	}

	return parsePluginConfig(cfg, source)
}
