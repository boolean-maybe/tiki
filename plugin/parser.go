package plugin

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"gopkg.in/yaml.v3"

	"github.com/boolean-maybe/tiki/plugin/filter"
)

// parsePluginConfig parses a pluginFileConfig into a Plugin
func parsePluginConfig(cfg pluginFileConfig, source string) (Plugin, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("plugin must have a name (%s)", source)
	}

	// Common fields
	// Use ColorDefault as sentinel so views can detect "not specified" and use theme-appropriate colors
	fg := parseColor(cfg.Foreground, tcell.ColorDefault)
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
		if len(cfg.Lanes) > 0 {
			return nil, fmt.Errorf("doki plugin cannot have 'lanes'")
		}
		if len(cfg.Actions) > 0 {
			return nil, fmt.Errorf("doki plugin cannot have 'actions'")
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
		if cfg.Filter != "" {
			return nil, fmt.Errorf("tiki plugin cannot have 'filter'")
		}
		if len(cfg.Lanes) == 0 {
			return nil, fmt.Errorf("tiki plugin requires 'lanes'")
		}
		if len(cfg.Lanes) > 10 {
			return nil, fmt.Errorf("tiki plugin has too many lanes (%d), max is 10", len(cfg.Lanes))
		}

		lanes := make([]TikiLane, 0, len(cfg.Lanes))
		for i, lane := range cfg.Lanes {
			if lane.Name == "" {
				return nil, fmt.Errorf("lane %d missing name", i)
			}
			columns := lane.Columns
			if columns == 0 {
				columns = 1
			}
			if columns < 0 {
				return nil, fmt.Errorf("lane %q has invalid columns %d", lane.Name, columns)
			}
			filterExpr, err := filter.ParseFilter(lane.Filter)
			if err != nil {
				return nil, fmt.Errorf("parsing filter for lane %q: %w", lane.Name, err)
			}
			action, err := ParseLaneAction(lane.Action)
			if err != nil {
				return nil, fmt.Errorf("parsing action for lane %q: %w", lane.Name, err)
			}
			lanes = append(lanes, TikiLane{
				Name:    lane.Name,
				Columns: columns,
				Filter:  filterExpr,
				Action:  action,
			})
		}

		// Parse sort rules
		sortRules, err := ParseSort(cfg.Sort)
		if err != nil {
			return nil, fmt.Errorf("parsing sort: %w", err)
		}

		// Parse plugin actions
		actions, err := parsePluginActions(cfg.Actions)
		if err != nil {
			return nil, fmt.Errorf("plugin %q (%s): %w", cfg.Name, source, err)
		}

		return &TikiPlugin{
			BasePlugin: base,
			Lanes:      lanes,
			Sort:       sortRules,
			ViewMode:   cfg.View,
			Actions:    actions,
		}, nil

	default:
		return nil, fmt.Errorf("unknown plugin type: %s", pluginType)
	}
}

// parsePluginActions parses and validates plugin action configs into PluginAction slice.
func parsePluginActions(configs []PluginActionConfig) ([]PluginAction, error) {
	if len(configs) == 0 {
		return nil, nil
	}
	if len(configs) > 10 {
		return nil, fmt.Errorf("too many actions (%d), max is 10", len(configs))
	}

	seen := make(map[rune]bool, len(configs))
	actions := make([]PluginAction, 0, len(configs))

	for i, cfg := range configs {
		if cfg.Key == "" {
			return nil, fmt.Errorf("action %d missing 'key'", i)
		}
		r, size := utf8.DecodeRuneInString(cfg.Key)
		if r == utf8.RuneError || size != len(cfg.Key) {
			return nil, fmt.Errorf("action %d key must be a single character, got %q", i, cfg.Key)
		}
		if !unicode.IsPrint(r) {
			return nil, fmt.Errorf("action %d key must be a printable character, got %q", i, cfg.Key)
		}
		if seen[r] {
			return nil, fmt.Errorf("duplicate action key %q", cfg.Key)
		}
		seen[r] = true

		if cfg.Label == "" {
			return nil, fmt.Errorf("action %d (key %q) missing 'label'", i, cfg.Key)
		}
		if cfg.Action == "" {
			return nil, fmt.Errorf("action %d (key %q) missing 'action'", i, cfg.Key)
		}

		action, err := ParseLaneAction(cfg.Action)
		if err != nil {
			return nil, fmt.Errorf("parsing action %d (key %q): %w", i, cfg.Key, err)
		}
		if len(action.Ops) == 0 {
			return nil, fmt.Errorf("action %d (key %q) has empty action expression", i, cfg.Key)
		}

		actions = append(actions, PluginAction{
			Rune:   r,
			Label:  cfg.Label,
			Action: action,
		})
	}

	return actions, nil
}

// parsePluginYAML parses plugin YAML data into a Plugin
func parsePluginYAML(data []byte, source string) (Plugin, error) {
	var cfg pluginFileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	return parsePluginConfig(cfg, source)
}
