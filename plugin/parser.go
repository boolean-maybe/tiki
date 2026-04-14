package plugin

import (
	"fmt"
	"log/slog"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/ruki"
)

// parsePluginConfig parses a pluginFileConfig into a Plugin
func parsePluginConfig(cfg pluginFileConfig, source string, schema ruki.Schema) (Plugin, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("plugin must have a name (%s)", source)
	}

	// Common fields
	// caption colors are now auto-generated per theme; YAML fg/bg fields are silently ignored
	fg := config.DefaultColor()
	bg := config.DefaultColor()

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
		Description: cfg.Description,
		Key:         key,
		Rune:        r,
		Modifier:    mod,
		Foreground:  fg,
		Background:  bg,
		FilePath:    source,
		Type:        pluginType,
		ConfigIndex: -1, // default, will be set by caller if needed
		Default:     cfg.Default,
	}

	switch pluginType {
	case "doki":
		// Strict validation for Doki
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
		if len(cfg.Lanes) == 0 {
			return nil, fmt.Errorf("tiki plugin requires 'lanes'")
		}
		if len(cfg.Lanes) > 10 {
			return nil, fmt.Errorf("tiki plugin has too many lanes (%d), max is 10", len(cfg.Lanes))
		}

		parser := ruki.NewParser(schema)

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
			if lane.Width < 0 || lane.Width > 100 {
				return nil, fmt.Errorf("lane %q has invalid width %d (must be 0-100)", lane.Name, lane.Width)
			}

			var filterStmt *ruki.ValidatedStatement
			if lane.Filter != "" {
				filterStmt, err = parser.ParseAndValidateStatement(lane.Filter, ruki.ExecutorRuntimePlugin)
				if err != nil {
					return nil, fmt.Errorf("parsing filter for lane %q: %w", lane.Name, err)
				}
				if !filterStmt.IsSelect() {
					return nil, fmt.Errorf("lane %q filter must be a SELECT statement", lane.Name)
				}
			}

			var actionStmt *ruki.ValidatedStatement
			if lane.Action != "" {
				actionStmt, err = parser.ParseAndValidateStatement(lane.Action, ruki.ExecutorRuntimePlugin)
				if err != nil {
					return nil, fmt.Errorf("parsing action for lane %q: %w", lane.Name, err)
				}
				if !actionStmt.IsUpdate() {
					return nil, fmt.Errorf("lane %q action must be an UPDATE statement", lane.Name)
				}
			}

			lanes = append(lanes, TikiLane{
				Name:    lane.Name,
				Columns: columns,
				Width:   lane.Width,
				Filter:  filterStmt,
				Action:  actionStmt,
			})
		}

		// warn if explicit lane widths exceed 100%
		widthSum := 0
		for _, lane := range lanes {
			widthSum += lane.Width
		}
		if widthSum > 100 {
			slog.Warn("lane widths sum exceeds 100%", "plugin", cfg.Name, "sum", widthSum)
		}

		// Parse plugin actions
		actions, err := parsePluginActions(cfg.Actions, parser)
		if err != nil {
			return nil, fmt.Errorf("plugin %q (%s): %w", cfg.Name, source, err)
		}

		return &TikiPlugin{
			BasePlugin: base,
			Lanes:      lanes,
			ViewMode:   cfg.View,
			Actions:    actions,
		}, nil

	default:
		return nil, fmt.Errorf("unknown plugin type: %s", pluginType)
	}
}

// parsePluginActions parses and validates plugin action configs into PluginAction slice.
func parsePluginActions(configs []PluginActionConfig, parser *ruki.Parser) ([]PluginAction, error) {
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

		actionStmt, err := parser.ParseAndValidateStatement(cfg.Action, ruki.ExecutorRuntimePlugin)
		if err != nil {
			return nil, fmt.Errorf("parsing action %d (key %q): %w", i, cfg.Key, err)
		}
		actions = append(actions, PluginAction{
			Rune:   r,
			Label:  cfg.Label,
			Action: actionStmt,
		})
	}

	return actions, nil
}

// parsePluginYAML parses plugin YAML data into a Plugin
func parsePluginYAML(data []byte, source string, schema ruki.Schema) (Plugin, error) {
	var cfg pluginFileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	return parsePluginConfig(cfg, source, schema)
}
