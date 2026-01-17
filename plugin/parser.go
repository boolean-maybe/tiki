package plugin

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"gopkg.in/yaml.v3"

	"github.com/boolean-maybe/tiki/plugin/filter"
)

// parsePluginConfig parses a pluginFileConfig into a Plugin
func parsePluginConfig(cfg pluginFileConfig, source string) (Plugin, error) {
	// Common fields
	fg := parseColor(cfg.Foreground, tcell.ColorWhite)
	bg := parseColor(cfg.Background, tcell.ColorDefault)

	key, r, mod, err := parseKey(cfg.Key)
	if err != nil {
		return nil, fmt.Errorf("plugin %q (%s): parsing key: %w", cfg.Name, source, err)
	}

	pluginType := cfg.Type
	if pluginType == "" {
		pluginType = "tiki"
	}

	base := BasePlugin{
		Name:        cfg.Name,
		Key:         key,
		Rune:        r,
		Modifier:    mod,
		Foreground:  fg,
		Background:  bg,
		FilePath:    source,
		Type:        pluginType,
		ConfigIndex: -1, // default, will be set by caller if needed
	}

	switch pluginType {
	case "doki":
		// Strict validation for Doki
		if cfg.Filter != "" {
			return nil, fmt.Errorf("doki plugin cannot have 'filter'")
		}
		if cfg.Sort != "" {
			return nil, fmt.Errorf("doki plugin cannot have 'sort'")
		}
		if cfg.View != "" {
			return nil, fmt.Errorf("doki plugin cannot have 'view'")
		}

		if cfg.Fetcher != "file" && cfg.Fetcher != "internal" {
			return nil, fmt.Errorf("doki plugin fetcher must be 'file' or 'internal', got '%s'", cfg.Fetcher)
		}
		if cfg.Fetcher == "file" && cfg.URL == "" {
			return nil, fmt.Errorf("doki plugin with file fetcher requires 'url'")
		}
		if cfg.Fetcher == "internal" && cfg.Text == "" {
			return nil, fmt.Errorf("doki plugin with internal fetcher requires 'text'")
		}

		return &DokiPlugin{
			BasePlugin: base,
			Fetcher:    cfg.Fetcher,
			Text:       cfg.Text,
			URL:        cfg.URL,
		}, nil

	case "tiki":
		// Strict validation for Tiki
		if cfg.Fetcher != "" {
			return nil, fmt.Errorf("tiki plugin cannot have 'fetcher'")
		}
		if cfg.Text != "" {
			return nil, fmt.Errorf("tiki plugin cannot have 'text'")
		}
		if cfg.URL != "" {
			return nil, fmt.Errorf("tiki plugin cannot have 'url'")
		}

		// Parse filter expression
		filterExpr, err := filter.ParseFilter(cfg.Filter)
		if err != nil {
			return nil, fmt.Errorf("parsing filter: %w", err)
		}

		// Parse sort rules
		sortRules, err := ParseSort(cfg.Sort)
		if err != nil {
			return nil, fmt.Errorf("parsing sort: %w", err)
		}

		return &TikiPlugin{
			BasePlugin: base,
			Filter:     filterExpr,
			Sort:       sortRules,
			ViewMode:   cfg.View,
		}, nil

	default:
		return nil, fmt.Errorf("unknown plugin type: %s", pluginType)
	}
}

// parsePluginYAML parses plugin YAML data into a Plugin
func parsePluginYAML(data []byte, source string) (Plugin, error) {
	var cfg pluginFileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	return parsePluginConfig(cfg, source)
}
