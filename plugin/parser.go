package plugin

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"gopkg.in/yaml.v3"

	"github.com/boolean-maybe/ruki"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/theme"
	"github.com/boolean-maybe/tiki/workflow"
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
			"`view:` as a display mode is no longer supported — declare a `layout:` field on the board/list view instead")
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
	if cfg.Mode != "" {
		return legacyFieldError("mode",
			"`mode:` is no longer supported — use `layout:` to declare the tiki-box layout on a board/list view")
	}
	if len(cfg.Metadata) > 0 {
		return legacyFieldError("metadata",
			"`metadata:` has been renamed to `layout:` — update your workflow.yaml")
	}
	return nil
}

// parsePluginConfig parses a pluginFileConfig into a Plugin.
// viewNames is the set of names declared in the enclosing workflow file, used
// to validate `kind: view` action targets. Pass nil for one-off parsing (e.g.
// tests); in that case view-target validation is skipped.
func parsePluginConfig(cfg pluginFileConfig, source string, schema ruki.Schema, viewNames map[string]ViewKind) (Plugin, error) {
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
		return parseDetailPlugin(cfg, base, schema, viewNames)
	default:
		// unreachable: IsValidKind already gated this
		return nil, fmt.Errorf("plugin %q (%s): unhandled kind %q", cfg.Name, source, kind)
	}
}

// parseBoardOrListPlugin handles board and list kinds.
func parseBoardOrListPlugin(cfg pluginFileConfig, base BasePlugin, schema ruki.Schema, viewNames map[string]ViewKind) (Plugin, error) {
	if cfg.Document != "" {
		return nil, fmt.Errorf("plugin %q: `document:` only valid on kind: wiki", cfg.Name)
	}
	if cfg.Path != "" {
		return nil, fmt.Errorf("plugin %q: `path:` only valid on kind: wiki", cfg.Name)
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

	layout, err := validateLayout(cfg.Name, cfg.Kind, cfg.Layout, schema)
	if err != nil {
		return nil, err
	}
	warnMultiRowFieldsInTikiBox(cfg.Name, layout)

	widthSum := 0
	for _, lane := range lanes {
		widthSum += lane.Width
	}
	if widthSum > 100 {
		slog.Warn("lane widths sum exceeds 100%", "plugin", cfg.Name, "sum", widthSum)
	}

	actions, err := parsePluginActions(cfg.Actions, parser, viewNames, false)
	if err != nil {
		return nil, fmt.Errorf("plugin %q (%s): %w", cfg.Name, base.FilePath, err)
	}

	return &WorkflowPlugin{
		BasePlugin: base,
		Lanes:      lanes,
		Layout:     layout,
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
	if strings.TrimSpace(cfg.Layout) != "" {
		return nil, fmt.Errorf("plugin %q: `layout:` only valid on kind: board, list, or detail", cfg.Name)
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
	return &WikiPlugin{
		BasePlugin:   base,
		DocumentPath: cfg.Path,
	}, nil
}

// parseDetailPlugin handles kind: detail — a configurable view of a single
// selected tiki. Renders title, the configured layout grid, and description.
// Per-view actions are allowed and will be surfaced alongside the built-in
// detail actions and global actions.
func parseDetailPlugin(cfg pluginFileConfig, base BasePlugin, schema ruki.Schema, viewNames map[string]ViewKind) (Plugin, error) {
	if err := rejectBoardOnlyFields(cfg, "detail"); err != nil {
		return nil, err
	}
	if cfg.Document != "" {
		return nil, fmt.Errorf("plugin %q: `document:` only valid on kind: wiki", cfg.Name)
	}
	if cfg.Path != "" {
		return nil, fmt.Errorf("plugin %q: `path:` only valid on kind: wiki", cfg.Name)
	}

	spec, err := validateLayout(cfg.Name, "detail", cfg.Layout, schema)
	if err != nil {
		return nil, err
	}

	parser := ruki.NewParser(schema)
	actions, err := parsePluginActions(cfg.Actions, parser, viewNames, true)
	if err != nil {
		return nil, fmt.Errorf("plugin %q (%s): %w", cfg.Name, base.FilePath, err)
	}

	return &DetailPlugin{
		BasePlugin: base,
		Layout:     spec,
		Actions:    actions,
	}, nil
}

// validateLayout parses the 2D layout grid and validates each anchor's
// field name against the schema. Literal cells may carry `<role>` color
// markup drawn from workflow.ValidRoles (escape literal `<` as `<<`);
// markup is parsed and role names checked at load time so typos surface
// at startup rather than at first render. Two categories of names may
// appear:
//
//   - Workflow-declared fields from `workflow.yaml fields:` (the common case).
//   - Supported audit fields: `createdBy`, `createdAt`, `updatedAt`. These
//     are system fields wired into the typed detail-view registry so they
//     render as read-only rows.
//   - `title` is allowed — it renders in-grid via the field registry as a
//     regular text field. Accepts optional `<role>` annotation.
//
// Detail-view-specific rejections (only applied when viewKind == "detail"):
//
//   - Identity/body fields other than `title` (`id`, `description`, `body`).
//     Detail-view chrome already renders these.
//   - Path fields (`filepath`, `path`) — values live on the tiki struct
//     rather than in Fields and have no typed detail renderer yet.
//
// Always rejected:
//
//   - Anything not in the schema (catches typos).
//   - Grid shape errors (ragged rows, orphan spans, mixed stretchers)
//     surface with row/col coordinates.
//
// Workflow-declared fields without a typed editor fall back to a generic
// read-only `Label: value` row at render time.
func validateLayout(pluginName, viewKind string, raw string, schema ruki.Schema) (gridlayout.GridSpec, error) {
	if strings.TrimSpace(raw) == "" {
		return gridlayout.GridSpec{}, fmt.Errorf(
			"plugin %q: view kind %q requires a non-empty `layout:` field", pluginName, viewKind)
	}
	spec, err := gridlayout.ParseLayout(raw)
	if err != nil {
		return gridlayout.GridSpec{}, fmt.Errorf("plugin %q: layout: %w", pluginName, err)
	}
	for _, a := range spec.Anchors {
		if a.Kind == gridlayout.AnchorLiteral {
			// Literal anchors carry static text and an optional `<role>`
			// prefix promoted to Anchor.Role/Modifier. Validate the role
			// name against the closed vocabulary; the text itself is
			// rendered verbatim (no inline markup parsing).
			if a.Role != "" {
				if _, ok := workflow.ValidRoles[a.Role]; !ok {
					return gridlayout.GridSpec{}, fmt.Errorf(
						"plugin %q: layout caption: unknown color role %q", pluginName, a.Role)
				}
				if a.Modifier != "" && !theme.IsKnownModifier(a.Modifier) {
					return gridlayout.GridSpec{}, fmt.Errorf(
						"plugin %q: layout caption: unknown color modifier %q", pluginName, a.Modifier)
				}
			}
			continue
		}
		if a.Kind == gridlayout.AnchorComposite {
			for _, seg := range a.Segments {
				if seg.Role != "" {
					if _, ok := workflow.ValidRoles[seg.Role]; !ok {
						label := seg.Name
						if seg.Kind == gridlayout.SegmentLiteral {
							label = "(literal)"
						}
						return gridlayout.GridSpec{}, fmt.Errorf(
							"plugin %q: layout composite segment %q: unknown color role %q", pluginName, label, seg.Role)
					}
					if seg.Modifier != "" && !theme.IsKnownModifier(seg.Modifier) {
						label := seg.Name
						if seg.Kind == gridlayout.SegmentLiteral {
							label = "(literal)"
						}
						return gridlayout.GridSpec{}, fmt.Errorf(
							"plugin %q: layout composite segment %q: unknown color modifier %q", pluginName, label, seg.Modifier)
					}
				}
				if seg.Kind == gridlayout.SegmentLiteral {
					continue
				}
				if err := validateLayoutFieldName(pluginName, viewKind, seg.Name, schema); err != nil {
					return gridlayout.GridSpec{}, err
				}
				if err := validateCountDisplay(pluginName, seg.Name, seg.Display, schema); err != nil {
					return gridlayout.GridSpec{}, err
				}
			}
			continue
		}
		if a.Role != "" {
			if _, ok := workflow.ValidRoles[a.Role]; !ok {
				return gridlayout.GridSpec{}, fmt.Errorf(
					"plugin %q: layout field %q: unknown color role %q", pluginName, a.Name, a.Role)
			}
			if a.Modifier != "" && !theme.IsKnownModifier(a.Modifier) {
				return gridlayout.GridSpec{}, fmt.Errorf(
					"plugin %q: layout field %q: unknown color modifier %q", pluginName, a.Name, a.Modifier)
			}
		}
		if err := validateLayoutFieldName(pluginName, viewKind, a.Name, schema); err != nil {
			return gridlayout.GridSpec{}, err
		}
		if err := validateCountDisplay(pluginName, a.Name, a.Display, schema); err != nil {
			return gridlayout.GridSpec{}, err
		}
	}
	return spec, nil
}

// validateCountDisplay rejects a `.count` display suffix on a non-list field.
// The item count is only meaningful for list-typed fields (stringList /
// tikiIdList); on any other type it would coerce to "0"/"1" misleadingly, so
// it is a hard load-time error. List-ness is read from the ruki schema, which
// is the type source threaded into layout validation; a nil schema (no
// validation pass) is a no-op.
func validateCountDisplay(pluginName, name string, display gridlayout.DisplayMode, schema ruki.Schema) error {
	if display != gridlayout.DisplayCount {
		return nil
	}
	if schema == nil {
		return nil
	}
	spec, ok := schema.Field(name)
	if !ok {
		// field existence is already enforced by validateLayoutFieldName.
		return nil
	}
	if spec.Type == ruki.ValueListString || spec.Type == ruki.ValueListRef {
		return nil
	}
	return fmt.Errorf(
		"plugin %q: layout field %q: .count is only valid on list fields (stringList/tikiIdList)",
		pluginName, name)
}

// warnMultiRowFieldsInTikiBox emits a load-time warning when a board/list
// layout references a list-typed workflow field (TypeListString or
// TypeListRef). Tiki cards on board/list views are fixed-height (one
// cell per layout row), so a multi-row field renders only its first row
// of content — the rest is clipped silently. The user-facing remedy is
// either to drop the field from the tiki box layout or to surface it on
// a kind: detail view, which honors per-field height.
func warnMultiRowFieldsInTikiBox(pluginName string, spec gridlayout.GridSpec) {
	check := func(name string) {
		fd, ok := workflow.Field(name)
		if !ok {
			return
		}
		if fd.Type.IsList() {
			slog.Warn(
				"layout references multi-row field on a fixed-height tiki box — only the first row will render",
				"plugin", pluginName, "field", name, "type", fd.Type)
		}
	}
	for _, a := range spec.Anchors {
		switch a.Kind {
		case gridlayout.AnchorComposite:
			// composites always render a single line on a card (the list segment
			// comma-joins, a `.count` segment is an integer), so a list field used
			// inside one is never clipped — nothing to warn about.
		case gridlayout.AnchorLiteral:
			// literals have no field name to check
		default:
			// a `.count` anchor renders a single integer line, never the multi-row
			// list value, so it is not subject to the clipping this warns about.
			if a.Display == gridlayout.DisplayCount {
				continue
			}
			check(a.Name)
		}
	}
}

func validateLayoutFieldName(pluginName, viewKind, name string, schema ruki.Schema) error {
	if name == "title" {
		return nil
	}
	if viewKind == "detail" {
		if isDetailIdentityField(name) {
			return fmt.Errorf(
				"plugin %q: layout cannot include %q — description and id are always rendered by the detail view chrome",
				pluginName, name)
		}
		if isDetailNonRenderableSystemField(name) {
			return fmt.Errorf(
				"plugin %q: layout cannot include %q — it lives on the tiki struct (not in Fields) and has no detail-view renderer",
				pluginName, name)
		}
	}
	if schema != nil {
		if _, ok := schema.Field(name); !ok {
			return fmt.Errorf(
				"plugin %q: layout field %q is not a workflow-declared field",
				pluginName, name)
		}
	}
	return nil
}

// isDetailIdentityField reports whether the field is one of the identity/body
// fields that are always rendered by a detail view (and therefore disallowed
// in `metadata:`). Title is NOT in this set — it renders in-grid.
func isDetailIdentityField(name string) bool {
	switch name {
	case "description", "body", "id":
		return true
	}
	return false
}

// isDetailNonRenderableSystemField reports whether the field is a system
// field whose value lives on the Tiki struct rather than in Fields, and
// for which the detail view has no typed renderer. Such names would render
// as the absent placeholder via the generic fall-back, which is misleading.
// createdBy/createdAt/updatedAt are also struct-bound but are wired into
// the typed registry, so they're allowed.
func isDetailNonRenderableSystemField(name string) bool {
	return name == "filepath" || name == "path"
}

// rejectBoardOnlyFields catches lanes set on a non-board/list view.
func rejectBoardOnlyFields(cfg pluginFileConfig, kind string) error {
	if len(cfg.Lanes) > 0 {
		return fmt.Errorf("plugin %q: `lanes:` only valid on board or list views (got kind: %s)", cfg.Name, kind)
	}
	return nil
}

// parsePluginActions parses and validates plugin action configs into PluginAction slice.
// viewNames (if non-nil) is used to validate `kind: view` action targets.
// sourceIsDetailView indicates whether these actions belong to a detail view's own actions: block.
func parsePluginActions(configs []PluginActionConfig, parser *ruki.Parser, viewNames map[string]ViewKind, sourceIsDetailView bool) ([]PluginAction, error) {
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
			parsed, err = parseViewAction(cfg, i, parser, viewNames, sourceIsDetailView, key, r, mod, keyStr)
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
	if cfg.Mode != "" {
		return PluginAction{}, fmt.Errorf("action %d (key %q): mode: only valid on kind: view actions", idx, cfg.Key)
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

// parseViewAction builds a view-navigation PluginAction from a `kind: view`
// action config. The optional `mode:` field selects a detail-view entry point
// when the target view is `kind: detail`; accepted values are `view` (read-only,
// the default when `mode:` is omitted), `edit` (edit mode), `new` (create a new
// tiki), `edit-desc` (edit description), and `edit-tags` (edit tags). Any other
// value is rejected.
//
// The optional `choose:` field carries a bare ruki "select [where <cond>]"
// that produces the candidate set for a QuickSelect picker; the chosen tiki
// id is then carried into the target view's navigation params instead of the
// source view's cursor selection. Pipes, `order by`, `limit`, and explicit
// field lists are rejected — `choose:` is strictly a candidate filter.
//
// Parse errors:
//   - "mode must be one of view, edit, new, edit-desc, edit-tags (got %q)" when
//     the value is not in the closed vocabulary.
//   - "mode: only valid when targeting a kind: detail view" when `mode:` is set
//     but the target view is not `kind: detail`.
//   - "mode: new not valid on a detail view's own actions" when `mode: new` is
//     declared on a detail view's own `actions:` block (a detail view is already
//     viewing a tiki, so creating a new one is not a self-action).
//   - "`choose:` does not support pipes; remove the `| …` suffix" when the
//     parsed select carries a pipe action.
//   - "`choose:` does not support `order by`" / "`limit`" / "explicit field
//     list" when the parsed select carries those clauses.
//   - "`choose:` cannot be combined with `require: [\"selection:one\"]`" — a
//     choose-bearing action produces its own selection via QuickSelect, so
//     pre-requiring a cursor selection on the source view is contradictory.
func parseViewAction(cfg PluginActionConfig, idx int, parser *ruki.Parser, viewNames map[string]ViewKind, sourceIsDetailView bool, key tcell.Key, r rune, mod tcell.ModMask, keyStr string) (PluginAction, error) {
	if cfg.View == "" {
		return PluginAction{}, fmt.Errorf("action %d (key %q): kind: view requires `view:` (target view name)", idx, cfg.Key)
	}
	if cfg.Input != "" {
		return PluginAction{}, fmt.Errorf("action %d (key %q): kind: view does not support `input:`", idx, cfg.Key)
	}

	var targetKind ViewKind
	if viewNames != nil {
		kind, ok := viewNames[cfg.View]
		if !ok {
			return PluginAction{}, fmt.Errorf("action %d (key %q): references unknown view %q", idx, cfg.Key, cfg.View)
		}
		targetKind = kind
	}

	modeStr := strings.TrimSpace(cfg.Mode)
	var mode DetailMode
	if modeStr == "" {
		mode = DetailModeView
	} else {
		switch DetailMode(modeStr) {
		case DetailModeView, DetailModeEdit, DetailModeNew,
			DetailModeEditDesc, DetailModeEditTags:
			mode = DetailMode(modeStr)
		default:
			return PluginAction{}, fmt.Errorf("action %d (key %q): mode must be one of view, edit, new, edit-desc, edit-tags (got %q)",
				idx, cfg.Key, modeStr)
		}
		if targetKind != KindDetail {
			return PluginAction{}, fmt.Errorf("action %d (key %q): mode: only valid when targeting a kind: detail view",
				idx, cfg.Key)
		}
		if mode == DetailModeNew && sourceIsDetailView {
			return PluginAction{}, fmt.Errorf("action %d (key %q): mode: new not valid on a detail view's own actions",
				idx, cfg.Key)
		}
	}

	// an optional `action:` on a kind: view action is the seed for mode: new —
	// a single ruki create statement that pre-fills the synthesized draft. it
	// is parsed only after mode is resolved so the guard can reject it on any
	// non-new mode.
	var createSeed *ruki.ValidatedStatement
	if strings.TrimSpace(cfg.Action) != "" {
		if mode != DetailModeNew {
			return PluginAction{}, fmt.Errorf(
				"action %d (key %q): kind: view must not set 'action:' unless mode: new", idx, cfg.Key)
		}
		stmt, err := parser.ParseAndValidateStatement(cfg.Action, ruki.ExecutorRuntimePlugin)
		if err != nil {
			return PluginAction{}, fmt.Errorf("parsing action %d (key %q) seed: %w", idx, cfg.Key, err)
		}
		if !stmt.IsCreate() {
			return PluginAction{}, fmt.Errorf(
				"action %d (key %q): mode: new 'action:' must be a create statement", idx, cfg.Key)
		}
		if strings.TrimSpace(cfg.Choose) != "" {
			return PluginAction{}, fmt.Errorf(
				"action %d (key %q): cannot combine `action:` create seed with `choose:`", idx, cfg.Key)
		}
		createSeed = stmt
	}

	require := make([]string, 0, len(cfg.Require))
	for _, r := range cfg.Require {
		if err := validateRequirement(r); err != nil {
			return PluginAction{}, fmt.Errorf("action %d (key %q) require: %w", idx, cfg.Key, err)
		}
		require = append(require, r)
	}

	var (
		hasChoose    bool
		chooseFilter *ruki.SubQuery
	)
	if strings.TrimSpace(cfg.Choose) != "" {
		filter, err := parseChooseField(cfg.Choose, idx, cfg.Key, parser)
		if err != nil {
			return PluginAction{}, err
		}
		for _, req := range require {
			if req == "selection:one" {
				return PluginAction{}, fmt.Errorf(
					"action %d (key %q): `choose:` cannot be combined with `require: [\"selection:one\"]`",
					idx, cfg.Key)
			}
		}
		hasChoose = true
		chooseFilter = filter
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
		Mode:         mode,
		CreateSeed:   createSeed,
		ShowInHeader: showInHeader,
		HasChoose:    hasChoose,
		ChooseFilter: chooseFilter,
		Require:      dedup(require),
	}, nil
}

// parseChooseField parses the value of a `choose:` YAML field on a kind: view
// action. The value must be a bare ruki "select [where <cond>]" — pipes,
// `order by`, `limit`, and explicit field lists are rejected so the candidate
// filter stays unambiguous (downstream sorts uniformly via
// sortTikisByPriorityTitle).
func parseChooseField(src string, idx int, cfgKey string, parser *ruki.Parser) (*ruki.SubQuery, error) {
	stmt, err := parser.ParseAndValidateStatement(src, ruki.ExecutorRuntimePlugin)
	if err != nil {
		return nil, fmt.Errorf("action %d (key %q) choose: %w", idx, cfgKey, err)
	}
	if !stmt.IsSelect() {
		return nil, fmt.Errorf(
			"action %d (key %q): `choose:` must be a `select` statement",
			idx, cfgKey)
	}
	if stmt.IsPipe() {
		return nil, fmt.Errorf(
			"action %d (key %q): `choose:` does not support pipes; remove the `| …` suffix",
			idx, cfgKey)
	}
	if stmt.HasOrderBy() {
		return nil, fmt.Errorf(
			"action %d (key %q): `choose:` does not support `order by`",
			idx, cfgKey)
	}
	if stmt.HasLimit() {
		return nil, fmt.Errorf(
			"action %d (key %q): `choose:` does not support `limit`",
			idx, cfgKey)
	}
	if stmt.HasFields() {
		return nil, fmt.Errorf(
			"action %d (key %q): `choose:` does not support an explicit field list",
			idx, cfgKey)
	}
	return &ruki.SubQuery{Where: stmt.SelectWhere()}, nil
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
//   - id() / filepath() / target.<field> → "id" (legacy alias for selection:one)
//   - ids() / filepaths() / targets.<field> → "selection:any" (at least one selection)
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

	needsSingle := stmt.UsesIDBuiltin() || stmt.UsesFilepathBuiltin() || stmt.UsesTargetQualifier()
	needsAny := stmt.UsesIDsBuiltin() || stmt.UsesFilepathsBuiltin() || stmt.UsesTargetsQualifier()

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
