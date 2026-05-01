package plugin

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"gopkg.in/yaml.v3"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/ruki"
)

// legacyFieldError returns a canonical rejection message when the user's YAML
// uses a pre-Phase-6 field. The second argument points them at the new syntax.
func legacyFieldError(field, replacement string) error {
	return fmt.Errorf("%q is no longer supported — %s", field, replacement)
}

// rejectLegacyTopLevel reports legacy fields at the view level before we
// dispatch on kind. Any legacy field produces an error even when kind is set
// correctly, because silently accepting them would mask user intent.
func rejectLegacyTopLevel(cfg pluginFileConfig) error {
	if cfg.Type != "" {
		switch strings.ToLower(cfg.Type) {
		case "tiki":
			return legacyFieldError("type", "use `kind: board` instead")
		case "doki":
			return legacyFieldError("type", "use `kind: wiki` instead")
		default:
			return legacyFieldError("type", "use `kind:` (board, list, wiki, detail, or search) instead")
		}
	}
	if cfg.View != "" {
		return legacyFieldError("view",
			"`view:` as a display mode is no longer supported — use `mode: compact` or `mode: expanded` on a board/list view")
	}
	if cfg.Fetcher != "" {
		return legacyFieldError("fetcher", "use `document:` or `path:` on a `kind: wiki` view")
	}
	if cfg.Text != "" {
		return legacyFieldError("text", "use `document:` or `path:` on a `kind: wiki` view")
	}
	if cfg.URL != "" {
		return legacyFieldError("url", "use `document:` or `path:` on a `kind: wiki` view")
	}
	if cfg.Sort != "" {
		return legacyFieldError("sort", "use `order by` inside a lane's `filter:` ruki statement")
	}
	return nil
}

// parsePluginConfig parses a pluginFileConfig into a Plugin.
// viewNames is the set of names declared in the enclosing workflow file, used
// to validate `kind: view` action targets. Pass nil for one-off parsing (e.g.
// tests); in that case view-target validation is skipped.
func parsePluginConfig(cfg pluginFileConfig, source string, schema ruki.Schema, viewNames map[string]struct{}) (Plugin, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("plugin must have a name (%s)", source)
	}

	if err := rejectLegacyTopLevel(cfg); err != nil {
		return nil, fmt.Errorf("plugin %q (%s): %w", cfg.Name, source, err)
	}

	if cfg.Kind == "" {
		return nil, fmt.Errorf("plugin %q (%s): missing `kind:` — expected board, list, wiki, detail, or search",
			cfg.Name, source)
	}
	if strings.ToLower(cfg.Kind) == string(KindTimeline) {
		return nil, fmt.Errorf("plugin %q (%s): `kind: timeline` is reserved but not yet implemented",
			cfg.Name, source)
	}
	if strings.ToLower(cfg.Kind) == string(KindSearch) {
		return nil, fmt.Errorf("plugin %q (%s): `kind: search` is reserved but not yet implemented "+
			"(the built-in global search UI is not plugin-instantiable; declaring it as a workflow view is a future enhancement)",
			cfg.Name, source)
	}
	if !IsValidKind(cfg.Kind) {
		return nil, fmt.Errorf("plugin %q (%s): unknown view kind %q — expected board, list, wiki, or detail",
			cfg.Name, source, cfg.Kind)
	}

	kind := ViewKind(cfg.Kind)

	// caption colors are auto-generated per theme; YAML fg/bg fields are silently ignored
	fg := config.DefaultColor()
	bg := config.DefaultColor()

	key, r, mod, _, err := parseCanonicalKey(cfg.Key)
	if err != nil {
		return nil, fmt.Errorf("plugin %q (%s): parsing key: %w", cfg.Name, source, err)
	}

	require, err := validateRequireList(cfg.Require)
	if err != nil {
		return nil, fmt.Errorf("plugin %q (%s): require: %w", cfg.Name, source, err)
	}

	base := BasePlugin{
		Name:        cfg.Name,
		Label:       cfg.Label,
		Description: cfg.Description,
		Key:         key,
		Rune:        r,
		Modifier:    mod,
		Foreground:  fg,
		Background:  bg,
		FilePath:    source,
		Kind:        kind,
		ConfigIndex: -1, // default, will be set by caller if needed
		Default:     cfg.Default,
		Require:     require,
	}

	switch kind {
	case KindBoard, KindList:
		return parseBoardOrListPlugin(cfg, base, schema, viewNames)
	case KindWiki:
		return parseWikiPlugin(cfg, base)
	case KindDetail:
		return parseDetailPlugin(cfg, base)
	default:
		// unreachable: IsValidKind already gated this
		return nil, fmt.Errorf("plugin %q (%s): unhandled kind %q", cfg.Name, source, kind)
	}
}

// parseBoardOrListPlugin handles board and list kinds.
func parseBoardOrListPlugin(cfg pluginFileConfig, base BasePlugin, schema ruki.Schema, viewNames map[string]struct{}) (Plugin, error) {
	if cfg.Document != "" {
		return nil, fmt.Errorf("plugin %q: `document:` only valid on kind: wiki", cfg.Name)
	}
	if cfg.Path != "" {
		return nil, fmt.Errorf("plugin %q: `path:` only valid on kind: wiki", cfg.Name)
	}

	mode := cfg.Mode
	if mode != "" && mode != "compact" && mode != "expanded" {
		return nil, fmt.Errorf("plugin %q: mode must be 'compact' or 'expanded', got %q", cfg.Name, mode)
	}

	if len(cfg.Lanes) == 0 {
		return nil, fmt.Errorf("plugin %q: kind %q requires 'lanes'", cfg.Name, cfg.Kind)
	}
	if len(cfg.Lanes) > 10 {
		return nil, fmt.Errorf("plugin %q: too many lanes (%d), max is 10", cfg.Name, len(cfg.Lanes))
	}

	parser := ruki.NewParser(schema)
	lanes, err := parseLanes(cfg.Name, cfg.Lanes, parser)
	if err != nil {
		return nil, err
	}

	widthSum := 0
	for _, lane := range lanes {
		widthSum += lane.Width
	}
	if widthSum > 100 {
		slog.Warn("lane widths sum exceeds 100%", "plugin", cfg.Name, "sum", widthSum)
	}

	actions, err := parsePluginActions(cfg.Actions, parser, viewNames)
	if err != nil {
		return nil, fmt.Errorf("plugin %q (%s): %w", cfg.Name, base.FilePath, err)
	}

	return &TikiPlugin{
		BasePlugin: base,
		Lanes:      lanes,
		Mode:       mode,
		Actions:    actions,
	}, nil
}

// parseLanes validates and parses the lanes section of a board/list view.
func parseLanes(pluginName string, configs []PluginLaneConfig, parser *ruki.Parser) ([]TikiLane, error) {
	lanes := make([]TikiLane, 0, len(configs))
	for i, lane := range configs {
		if lane.Name == "" {
			return nil, fmt.Errorf("plugin %q: lane %d missing name", pluginName, i)
		}
		columns := lane.Columns
		if columns == 0 {
			columns = 1
		}
		if columns < 0 {
			return nil, fmt.Errorf("plugin %q: lane %q has invalid columns %d", pluginName, lane.Name, columns)
		}
		if lane.Width < 0 || lane.Width > 100 {
			return nil, fmt.Errorf("plugin %q: lane %q has invalid width %d (must be 0-100)", pluginName, lane.Name, lane.Width)
		}

		filterStmt, err := parseLaneFilter(pluginName, lane, parser)
		if err != nil {
			return nil, err
		}
		actionStmt, err := parseLaneAction(pluginName, lane, parser)
		if err != nil {
			return nil, err
		}

		lanes = append(lanes, TikiLane{
			Name:    lane.Name,
			Columns: columns,
			Width:   lane.Width,
			Filter:  filterStmt,
			Action:  actionStmt,
		})
	}
	return lanes, nil
}

func parseLaneFilter(pluginName string, lane PluginLaneConfig, parser *ruki.Parser) (*ruki.ValidatedStatement, error) {
	if lane.Filter == "" {
		return nil, nil
	}
	stmt, err := parser.ParseAndValidateStatement(lane.Filter, ruki.ExecutorRuntimePlugin)
	if err != nil {
		return nil, fmt.Errorf("plugin %q: parsing filter for lane %q: %w", pluginName, lane.Name, err)
	}
	if !stmt.IsSelect() {
		return nil, fmt.Errorf("plugin %q: lane %q filter must be a SELECT statement", pluginName, lane.Name)
	}
	if stmt.HasAnyInteractive() {
		return nil, fmt.Errorf("plugin %q: lane %q filter cannot use interactive builtins (input/choose)", pluginName, lane.Name)
	}
	if stmt.UsesTargetQualifier() {
		return nil, fmt.Errorf("plugin %q: lane %q filter cannot use target. — no selection context at render time", pluginName, lane.Name)
	}
	if stmt.UsesTargetsQualifier() {
		return nil, fmt.Errorf("plugin %q: lane %q filter cannot use targets. — no selection context at render time", pluginName, lane.Name)
	}
	return stmt, nil
}

func parseLaneAction(pluginName string, lane PluginLaneConfig, parser *ruki.Parser) (*ruki.ValidatedStatement, error) {
	if lane.Action == "" {
		return nil, nil
	}
	stmt, err := parser.ParseAndValidateStatement(lane.Action, ruki.ExecutorRuntimePlugin)
	if err != nil {
		return nil, fmt.Errorf("plugin %q: parsing action for lane %q: %w", pluginName, lane.Name, err)
	}
	if !stmt.IsUpdate() {
		return nil, fmt.Errorf("plugin %q: lane %q action must be an UPDATE statement", pluginName, lane.Name)
	}
	if stmt.HasAnyInteractive() {
		return nil, fmt.Errorf("plugin %q: lane %q action cannot use interactive builtins (input/choose)", pluginName, lane.Name)
	}
	return stmt, nil
}

// parseWikiPlugin handles kind: wiki — a markdown view bound to a specific document.
// As of Phase 6A only `path:` is accepted. `document:` (ID-based resolution)
// lands in Phase 6B together with the document-store wikilink resolver; until
// then, accepting it would ship a placeholder instead of real content, so the
// parser rejects it with a clear deferral message.
func parseWikiPlugin(cfg pluginFileConfig, base BasePlugin) (Plugin, error) {
	if err := rejectBoardOnlyFields(cfg, "wiki"); err != nil {
		return nil, err
	}
	if len(cfg.Actions) > 0 {
		return nil, fmt.Errorf("plugin %q: kind: wiki cannot have per-view `actions:` — use top-level actions", cfg.Name)
	}
	if cfg.Document != "" {
		return nil, fmt.Errorf(
			"plugin %q: `document:` (ID-based resolution) is not yet implemented — "+
				"use `path:` with a relative filepath. ID-based binding is a future enhancement.",
			cfg.Name)
	}
	if cfg.Path == "" {
		return nil, fmt.Errorf("plugin %q: kind: wiki requires `path:` (relative path to a markdown document)", cfg.Name)
	}
	return &DokiPlugin{
		BasePlugin:   base,
		DocumentPath: cfg.Path,
	}, nil
}

// parseDetailPlugin handles kind: detail — a markdown view of the current selection.
func parseDetailPlugin(cfg pluginFileConfig, base BasePlugin) (Plugin, error) {
	if err := rejectBoardOnlyFields(cfg, "detail"); err != nil {
		return nil, err
	}
	if cfg.Document != "" {
		return nil, fmt.Errorf("plugin %q: `document:` only valid on kind: wiki", cfg.Name)
	}
	if cfg.Path != "" {
		return nil, fmt.Errorf("plugin %q: `path:` only valid on kind: wiki", cfg.Name)
	}
	if len(cfg.Actions) > 0 {
		return nil, fmt.Errorf("plugin %q: kind: detail cannot have per-view `actions:` — use top-level actions", cfg.Name)
	}
	return &DokiPlugin{BasePlugin: base}, nil
}

// rejectBoardOnlyFields catches lanes/mode set on a non-board/list view.
func rejectBoardOnlyFields(cfg pluginFileConfig, kind string) error {
	if len(cfg.Lanes) > 0 {
		return fmt.Errorf("plugin %q: `lanes:` only valid on board or list views (got kind: %s)", cfg.Name, kind)
	}
	if cfg.Mode != "" {
		return fmt.Errorf("plugin %q: `mode:` only valid on board or list views (got kind: %s)", cfg.Name, kind)
	}
	return nil
}

// parsePluginActions parses and validates plugin action configs into PluginAction slice.
// viewNames (if non-nil) is used to validate `kind: view` action targets.
func parsePluginActions(configs []PluginActionConfig, parser *ruki.Parser, viewNames map[string]struct{}) ([]PluginAction, error) {
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

		actionKind, err := resolveActionKind(cfg, i)
		if err != nil {
			return nil, err
		}

		var parsed PluginAction
		switch actionKind {
		case ActionKindRuki:
			parsed, err = parseRukiAction(cfg, i, parser, key, r, mod, keyStr)
		case ActionKindView:
			parsed, err = parseViewAction(cfg, i, viewNames, key, r, mod, keyStr)
		default:
			err = fmt.Errorf("action %d (key %q): unknown action kind %q", i, cfg.Key, cfg.Kind)
		}
		if err != nil {
			return nil, err
		}
		actions = append(actions, parsed)
	}

	return actions, nil
}

// resolveActionKind determines whether an action is a ruki or view action.
// Explicit `kind:` wins. Otherwise: `action:` set → ruki; `view:` set → view.
// Both set or neither set is an error.
func resolveActionKind(cfg PluginActionConfig, idx int) (ActionKind, error) {
	switch strings.ToLower(cfg.Kind) {
	case string(ActionKindRuki):
		return ActionKindRuki, nil
	case string(ActionKindView):
		return ActionKindView, nil
	case "":
		// inference path
	default:
		return "", fmt.Errorf("action %d (key %q): unknown kind %q — expected `ruki` or `view`", idx, cfg.Key, cfg.Kind)
	}

	hasAction := cfg.Action != ""
	hasView := cfg.View != ""
	switch {
	case hasAction && hasView:
		return "", fmt.Errorf("action %d (key %q): cannot set both `action:` and `view:` — use `kind:` to disambiguate", idx, cfg.Key)
	case hasAction:
		return ActionKindRuki, nil
	case hasView:
		return ActionKindView, nil
	default:
		return "", fmt.Errorf("action %d (key %q): must set either `action:` (ruki) or `view:` (view navigation)", idx, cfg.Key)
	}
}

// parseRukiAction builds a ruki-kind PluginAction.
func parseRukiAction(cfg PluginActionConfig, idx int, parser *ruki.Parser, key tcell.Key, r rune, mod tcell.ModMask, keyStr string) (PluginAction, error) {
	if cfg.Action == "" {
		return PluginAction{}, fmt.Errorf("action %d (key %q): kind: ruki requires `action:`", idx, cfg.Key)
	}
	if cfg.View != "" {
		return PluginAction{}, fmt.Errorf("action %d (key %q): kind: ruki must not set `view:`", idx, cfg.Key)
	}

	var (
		stmt         *ruki.ValidatedStatement
		inputType    ruki.ValueType
		hasInput     bool
		hasChoose    bool
		chooseFilter *ruki.SubQuery
		err          error
	)

	if cfg.Input != "" {
		typ, parseErr := ruki.ParseScalarTypeName(cfg.Input)
		if parseErr != nil {
			return PluginAction{}, fmt.Errorf("action %d (key %q) input: %w", idx, cfg.Key, parseErr)
		}
		stmt, err = parser.ParseAndValidateStatementWithInput(cfg.Action, ruki.ExecutorRuntimePlugin, typ)
		if err != nil {
			return PluginAction{}, fmt.Errorf("parsing action %d (key %q): %w", idx, cfg.Key, err)
		}
		if !stmt.UsesInputBuiltin() {
			return PluginAction{}, fmt.Errorf("action %d (key %q) declares 'input: %s' but does not use input()", idx, cfg.Key, cfg.Input)
		}
		inputType = typ
		hasInput = true
	} else {
		stmt, err = parser.ParseAndValidateStatement(cfg.Action, ruki.ExecutorRuntimePlugin)
		if err != nil {
			return PluginAction{}, fmt.Errorf("parsing action %d (key %q): %w", idx, cfg.Key, err)
		}
	}

	if stmt.IsExpr() {
		return PluginAction{}, fmt.Errorf(
			"action %d (key %q): action must be create, update, delete, or select (got expression statement)",
			idx, cfg.Key,
		)
	}

	if stmt.UsesChooseBuiltin() {
		hasChoose = true
		chooseFilter = stmt.ChooseFilter()
	}

	require, err := inferRequirements(cfg.Require, stmt, idx, cfg.Key)
	if err != nil {
		return PluginAction{}, err
	}

	showInHeader := true
	if cfg.Hot != nil {
		showInHeader = *cfg.Hot
	}
	return PluginAction{
		Key:          key,
		Rune:         r,
		Modifier:     mod,
		KeyStr:       keyStr,
		Label:        cfg.Label,
		Kind:         ActionKindRuki,
		Action:       stmt,
		ShowInHeader: showInHeader,
		InputType:    inputType,
		HasInput:     hasInput,
		HasChoose:    hasChoose,
		ChooseFilter: chooseFilter,
		Require:      require,
	}, nil
}

// parseViewAction builds a view-navigation PluginAction.
func parseViewAction(cfg PluginActionConfig, idx int, viewNames map[string]struct{}, key tcell.Key, r rune, mod tcell.ModMask, keyStr string) (PluginAction, error) {
	if cfg.View == "" {
		return PluginAction{}, fmt.Errorf("action %d (key %q): kind: view requires `view:` (target view name)", idx, cfg.Key)
	}
	if cfg.Action != "" {
		return PluginAction{}, fmt.Errorf("action %d (key %q): kind: view must not set `action:`", idx, cfg.Key)
	}
	if cfg.Input != "" {
		return PluginAction{}, fmt.Errorf("action %d (key %q): kind: view does not support `input:`", idx, cfg.Key)
	}
	if viewNames != nil {
		if _, ok := viewNames[cfg.View]; !ok {
			return PluginAction{}, fmt.Errorf("action %d (key %q): references unknown view %q", idx, cfg.Key, cfg.View)
		}
	}

	require := make([]string, 0, len(cfg.Require))
	for _, r := range cfg.Require {
		if err := validateRequirement(r); err != nil {
			return PluginAction{}, fmt.Errorf("action %d (key %q) require: %w", idx, cfg.Key, err)
		}
		require = append(require, r)
	}

	showInHeader := true
	if cfg.Hot != nil {
		showInHeader = *cfg.Hot
	}
	return PluginAction{
		Key:          key,
		Rune:         r,
		Modifier:     mod,
		KeyStr:       keyStr,
		Label:        cfg.Label,
		Kind:         ActionKindView,
		TargetView:   cfg.View,
		ShowInHeader: showInHeader,
		Require:      dedup(require),
	}, nil
}

// validateRequireList validates view-level `require:` tokens.
func validateRequireList(reqs []string) ([]string, error) {
	if len(reqs) == 0 {
		return nil, nil
	}
	for _, r := range reqs {
		if err := validateRequirement(r); err != nil {
			return nil, err
		}
	}
	return dedup(reqs), nil
}

// inferRequirements validates explicit requirements and auto-infers selection
// requirements from builtin and qualifier usage:
//   - id() or target.<field> → "id" (legacy alias for selection:one)
//   - ids() or targets.<field> → "selection:any" (at least one selection)
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

	needsSingle := stmt.UsesIDBuiltin() || stmt.UsesTargetQualifier()
	needsAny := stmt.UsesIDsBuiltin() || stmt.UsesTargetsQualifier()

	if needsSingle && !containsRequirement(reqs, "id") {
		reqs = append(reqs, "id")
	}
	if needsAny && !hasAnySelectionRequirement(reqs) {
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

// parsePluginYAML parses plugin YAML data into a Plugin. Exposed for tests;
// passes nil viewNames so `kind: view` targets are not cross-validated.
func parsePluginYAML(data []byte, source string, schema ruki.Schema) (Plugin, error) {
	var cfg pluginFileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	return parsePluginConfig(cfg, source, schema, nil)
}
