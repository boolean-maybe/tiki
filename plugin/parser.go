package plugin

import (
	"fmt"
	"log/slog"
	"strings"

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

	key, r, mod, _, err := parseCanonicalKey(cfg.Key)
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
				if filterStmt.HasAnyInteractive() {
					return nil, fmt.Errorf("lane %q filter cannot use interactive builtins (input/choose)", lane.Name)
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
				if actionStmt.HasAnyInteractive() {
					return nil, fmt.Errorf("lane %q action cannot use interactive builtins (input/choose)", lane.Name)
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

	seen := make(map[string]bool, len(configs))
	actions := make([]PluginAction, 0, len(configs))

	for i, cfg := range configs {
		if cfg.Key == "" {
			return nil, fmt.Errorf("action %d missing 'key'", i)
		}
		key, r, mod, keyStr, err := parseCanonicalKey(cfg.Key)
		if err != nil {
			return nil, fmt.Errorf("action %d key %q: %w", i, cfg.Key, err)
		}
		if seen[keyStr] {
			return nil, fmt.Errorf("duplicate action key %q", cfg.Key)
		}
		seen[keyStr] = true

		if cfg.Label == "" {
			return nil, fmt.Errorf("action %d (key %q) missing 'label'", i, cfg.Key)
		}
		if cfg.Action == "" {
			return nil, fmt.Errorf("action %d (key %q) missing 'action'", i, cfg.Key)
		}

		var (
			actionStmt   *ruki.ValidatedStatement
			inputType    ruki.ValueType
			hasInput     bool
			hasChoose    bool
			chooseFilter *ruki.SubQuery
		)

		if cfg.Input != "" {
			typ, err := ruki.ParseScalarTypeName(cfg.Input)
			if err != nil {
				return nil, fmt.Errorf("action %d (key %q) input: %w", i, cfg.Key, err)
			}
			actionStmt, err = parser.ParseAndValidateStatementWithInput(cfg.Action, ruki.ExecutorRuntimePlugin, typ)
			if err != nil {
				return nil, fmt.Errorf("parsing action %d (key %q): %w", i, cfg.Key, err)
			}
			if !actionStmt.UsesInputBuiltin() {
				return nil, fmt.Errorf("action %d (key %q) declares 'input: %s' but does not use input()", i, cfg.Key, cfg.Input)
			}
			inputType = typ
			hasInput = true
		} else {
			var err error
			actionStmt, err = parser.ParseAndValidateStatement(cfg.Action, ruki.ExecutorRuntimePlugin)
			if err != nil {
				return nil, fmt.Errorf("parsing action %d (key %q): %w", i, cfg.Key, err)
			}
		}

		if actionStmt.UsesChooseBuiltin() {
			hasChoose = true
			chooseFilter = actionStmt.ChooseFilter()
		}

		require, err := inferRequirements(cfg.Require, actionStmt, i, cfg.Key)
		if err != nil {
			return nil, err
		}

		showInHeader := true
		if cfg.Hot != nil {
			showInHeader = *cfg.Hot
		}
		actions = append(actions, PluginAction{
			Key:          key,
			Rune:         r,
			Modifier:     mod,
			KeyStr:       keyStr,
			Label:        cfg.Label,
			Action:       actionStmt,
			ShowInHeader: showInHeader,
			InputType:    inputType,
			HasInput:     hasInput,
			HasChoose:    hasChoose,
			ChooseFilter: chooseFilter,
			Require:      require,
		})
	}

	return actions, nil
}

// inferRequirements validates explicit requirements and auto-infers selection
// requirements from builtin usage:
//   - id() → "id" (legacy alias for selection:one)
//   - ids() → "selection:any" (at least one selection)
//
// selected_count() deliberately does NOT auto-infer anything: its whole
// purpose is to let ruki branch on cardinality, including the zero case
// (e.g. `where selected_count() = 0`). Gating the action on a non-zero
// selection would make that branch unreachable. Authors who want tighter
// gating can add `require: ["selection:any"]` (or `selection:many`)
// explicitly.
func inferRequirements(explicit []string, stmt *ruki.ValidatedStatement, idx int, key string) ([]string, error) {
	for _, r := range explicit {
		if err := validateRequirement(r); err != nil {
			return nil, fmt.Errorf("action %d (key %q) require: %w", idx, key, err)
		}
	}

	reqs := make([]string, len(explicit))
	copy(reqs, explicit)

	if stmt.UsesIDBuiltin() && !containsRequirement(reqs, "id") {
		reqs = append(reqs, "id")
	}
	if stmt.UsesIDsBuiltin() && !hasAnySelectionRequirement(reqs) {
		reqs = append(reqs, "selection:any")
	}

	if len(reqs) == 0 {
		return nil, nil
	}
	return dedup(reqs), nil
}

func containsRequirement(reqs []string, target string) bool {
	for _, r := range reqs {
		if r == target {
			return true
		}
	}
	return false
}

// hasAnySelectionRequirement returns true when the requirement list already
// constrains selection cardinality (positive or negated), so auto-inference
// should not layer another one on top. A negated token like !selection:many
// is still a cardinality constraint — it means "fewer than two" — so stacking
// selection:any on top of it would silently change the author's intent.
func hasAnySelectionRequirement(reqs []string) bool {
	for _, r := range reqs {
		attr := r
		if len(attr) > 0 && attr[0] == '!' {
			attr = attr[1:]
		}
		switch attr {
		case "id", "selection:one", "selection:any", "selection:many":
			return true
		}
	}
	return false
}

// validateRequirement checks that a requirement token is well-formed.
func validateRequirement(r string) error {
	if r == "" {
		return fmt.Errorf("empty requirement")
	}
	if r == "!" {
		return fmt.Errorf("bare '!' is not a valid requirement")
	}
	attr := r
	if attr[0] == '!' {
		attr = attr[1:]
	}
	if len(attr) > 0 && attr[0] == '!' {
		return fmt.Errorf("requirement %q has multiple '!' prefixes", r)
	}
	if strings.TrimSpace(attr) != attr || strings.ContainsAny(attr, " \t\n") {
		return fmt.Errorf("requirement %q contains invalid whitespace", r)
	}
	return nil
}

func dedup(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// parsePluginYAML parses plugin YAML data into a Plugin
func parsePluginYAML(data []byte, source string, schema ruki.Schema) (Plugin, error) {
	var cfg pluginFileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	return parsePluginConfig(cfg, source, schema)
}
