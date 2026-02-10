package plugin

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// pluginFileConfig represents the YAML structure of a plugin file
type pluginFileConfig struct {
	Name       string               `yaml:"name"`
	Foreground string               `yaml:"foreground"` // hex color like "#ff0000" or named color
	Background string               `yaml:"background"`
	Key        string               `yaml:"key"` // single character
	Filter     string               `yaml:"filter"`
	Sort       string               `yaml:"sort"`
	View       string               `yaml:"view"` // "compact" or "expanded" (default: compact)
	Type       string               `yaml:"type"` // "tiki" or "doki" (default: tiki)
	Fetcher    string               `yaml:"fetcher"`
	Text       string               `yaml:"text"`
	URL        string               `yaml:"url"`
	Panes      []PluginPaneConfig   `yaml:"panes"`
	Actions    []PluginActionConfig `yaml:"actions"`
}

// mergePluginConfigs merges file-based config (base) with inline overrides
// Inline values override file values for any non-empty field
func mergePluginConfigs(base pluginFileConfig, overrides PluginRef) pluginFileConfig {
	result := base

	if overrides.Name != "" {
		result.Name = overrides.Name
	}
	if overrides.Foreground != "" {
		result.Foreground = overrides.Foreground
	}
	if overrides.Background != "" {
		result.Background = overrides.Background
	}
	if overrides.Key != "" {
		result.Key = overrides.Key
	}
	if overrides.Filter != "" {
		result.Filter = overrides.Filter
	}
	if overrides.Sort != "" {
		result.Sort = overrides.Sort
	}
	if overrides.View != "" {
		result.View = overrides.View
	}
	if overrides.Type != "" {
		result.Type = overrides.Type
	}
	if overrides.Fetcher != "" {
		result.Fetcher = overrides.Fetcher
	}
	if overrides.Text != "" {
		result.Text = overrides.Text
	}
	if overrides.URL != "" {
		result.URL = overrides.URL
	}
	if len(overrides.Panes) > 0 {
		result.Panes = overrides.Panes
	}
	if len(overrides.Actions) > 0 {
		result.Actions = overrides.Actions
	}

	return result
}

// mergePluginDefinitions merges an embedded plugin (base) with a configured override
// Override fields replace base fields only if they are non-zero/non-empty
func mergePluginDefinitions(base Plugin, override Plugin) Plugin {
	// Currently only Tiki plugins are embedded, so we primarily handle Tiki merging
	baseTiki, baseIsTiki := base.(*TikiPlugin)
	overrideTiki, overrideIsTiki := override.(*TikiPlugin)

	if baseIsTiki && overrideIsTiki {
		result := &TikiPlugin{
			BasePlugin: BasePlugin{
				Name:        baseTiki.Name,
				Key:         baseTiki.Key,
				Rune:        baseTiki.Rune,
				Modifier:    baseTiki.Modifier, // FIXED: Copy modifier from base
				Foreground:  baseTiki.Foreground,
				Background:  baseTiki.Background,
				FilePath:    overrideTiki.FilePath,    // Use override's filepath for tracking
				ConfigIndex: overrideTiki.ConfigIndex, // Use override's config index
				Type:        baseTiki.Type,
			},
			Panes:    baseTiki.Panes,
			Sort:     baseTiki.Sort,
			ViewMode: baseTiki.ViewMode,
			Actions:  baseTiki.Actions,
		}

		// Apply overrides for non-zero values
		if overrideTiki.Key != 0 || overrideTiki.Rune != 0 || overrideTiki.Modifier != 0 {
			result.Key = overrideTiki.Key
			result.Rune = overrideTiki.Rune
			result.Modifier = overrideTiki.Modifier
		}
		if overrideTiki.Foreground != tcell.ColorDefault {
			result.Foreground = overrideTiki.Foreground
		}
		if overrideTiki.Background != tcell.ColorDefault {
			result.Background = overrideTiki.Background
		}
		if len(overrideTiki.Panes) > 0 {
			result.Panes = overrideTiki.Panes
		}
		if overrideTiki.Sort != nil {
			result.Sort = overrideTiki.Sort
		}
		if overrideTiki.ViewMode != "" {
			result.ViewMode = overrideTiki.ViewMode
		}
		if len(overrideTiki.Actions) > 0 {
			result.Actions = overrideTiki.Actions
		}
		// Type is usually "tiki" for both, but if overridden to "tiki" explicitly, it's fine.

		return result
	}

	// If we are replacing an embedded plugin with a Doki plugin (or vice versa, effectively replacing it),
	// just return the override.
	return override
}

// validatePluginRef validates a PluginRef before loading
func validatePluginRef(ref PluginRef) error {
	if ref.File != "" {
		// File-based or hybrid - name is optional (can come from file)
		return nil
	}

	// Fully inline - must have name
	if ref.Name == "" {
		return fmt.Errorf("inline plugin must specify 'name' field")
	}

	// Should have at least one configuration field
	hasContent := ref.Key != "" || ref.Filter != "" ||
		ref.Sort != "" || ref.Foreground != "" ||
		ref.Background != "" || ref.View != "" || ref.Type != "" ||
		ref.Fetcher != "" || ref.Text != "" || ref.URL != "" ||
		len(ref.Panes) > 0 || len(ref.Actions) > 0

	if !hasContent {
		return fmt.Errorf("inline plugin '%s' has no configuration fields", ref.Name)
	}

	return nil
}
