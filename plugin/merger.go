package plugin

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

// mergePluginDefinitions merges a base plugin with a configured override.
// Override fields replace base fields only if they are non-zero/non-empty.
func mergePluginDefinitions(base Plugin, override Plugin) Plugin {
	// Currently only Tiki plugins support field-level merging
	baseTiki, baseIsTiki := base.(*TikiPlugin)
	overrideTiki, overrideIsTiki := override.(*TikiPlugin)

	if baseIsTiki && overrideIsTiki {
		result := &TikiPlugin{
			BasePlugin: BasePlugin{
				Name:        baseTiki.Name,
				Description: baseTiki.Description,
				Key:         baseTiki.Key,
				Rune:        baseTiki.Rune,
				Modifier:    baseTiki.Modifier, // FIXED: Copy modifier from base
				Foreground:  baseTiki.Foreground,
				Background:  baseTiki.Background,
				ForegroundSet: baseTiki.ForegroundSet,
				BackgroundSet: baseTiki.BackgroundSet,
				FilePath:    overrideTiki.FilePath,    // Use override's filepath for tracking
				ConfigIndex: overrideTiki.ConfigIndex, // Use override's config index
				Type:        baseTiki.Type,
				Default:     baseTiki.Default,
			},
			Lanes:    baseTiki.Lanes,
			ViewMode: baseTiki.ViewMode,
			Actions:  baseTiki.Actions,
		}

		// Apply overrides for non-zero values
		if overrideTiki.Description != "" {
			result.Description = overrideTiki.Description
		}
		if overrideTiki.Key != 0 || overrideTiki.Rune != 0 || overrideTiki.Modifier != 0 {
			result.Key = overrideTiki.Key
			result.Rune = overrideTiki.Rune
			result.Modifier = overrideTiki.Modifier
		}
		if overrideTiki.ForegroundSet {
			result.Foreground = overrideTiki.Foreground
			result.ForegroundSet = true
		}
		if overrideTiki.BackgroundSet {
			result.Background = overrideTiki.Background
			result.BackgroundSet = true
		}
		if len(overrideTiki.Lanes) > 0 {
			result.Lanes = overrideTiki.Lanes
		}
		if overrideTiki.ViewMode != "" {
			result.ViewMode = overrideTiki.ViewMode
		}
		if len(overrideTiki.Actions) > 0 {
			result.Actions = overrideTiki.Actions
		}
		if overrideTiki.Default {
			result.Default = true
		}

		return result
	}

	// If the base and override are different types (e.g. replacing a Tiki with a Doki plugin),
	// just return the override.
	return override
}
