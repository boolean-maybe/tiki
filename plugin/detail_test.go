package plugin

import (
	"strings"
	"testing"
)

// TestDetailPlugin_ParsesLayout asserts kind: detail accepts layout: and
// builds a DetailPlugin with the parsed layout grid.
func TestDetailPlugin_ParsesLayout(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Detail",
		Kind:   "detail",
		Layout: "status | type | priority",
	}
	p, err := parsePluginConfig(cfg, "test", schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dp, ok := p.(*DetailPlugin)
	if !ok {
		t.Fatalf("expected *DetailPlugin, got %T", p)
	}
	got := strings.Join(dp.Layout.AnchorNames(), ",")
	if got != "status,type,priority" {
		t.Errorf("layout anchor names = %q, want %q", got, "status,type,priority")
	}
}

// TestDetailPlugin_RejectsUnknownLayoutField asserts that referencing a field
// the schema doesn't know fails the workflow load instead of silently skipping.
func TestDetailPlugin_RejectsUnknownLayoutField(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Detail",
		Kind:   "detail",
		Layout: "status | no_such_field",
	}
	_, err := parsePluginConfig(cfg, "test", schema, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no_such_field") {
		t.Errorf("expected error mentioning the unknown field, got %q", err.Error())
	}
}

// TestDetailPlugin_AcceptsAnyTypedOrGenericWorkflowField asserts that the
// detail loader accepts any workflow-declared field that has a renderable
// value path. Fields with typed editors render via the typed registry;
// fields without typed editors render via a generic catalog-driven row.
func TestDetailPlugin_AcceptsAnyTypedOrGenericWorkflowField(t *testing.T) {
	schema := testSchema()
	for _, name := range []string{"status", "type", "priority", "createdBy", "createdAt", "updatedAt"} {
		t.Run(name, func(t *testing.T) {
			cfg := pluginFileConfig{
				Name:   "Detail",
				Kind:   "detail",
				Layout: name,
			}
			if _, err := parsePluginConfig(cfg, "test", schema, nil); err != nil {
				t.Errorf("%q should be accepted in layout, got: %v", name, err)
			}
		})
	}
}

// TestDetailPlugin_RejectsFilepathInLayout pins that filepath — a system
// field whose value lives on tk.Path rather than in tk.Fields — is rejected
// at workflow load. Letting it through would render as "filepath: —"
// because the generic catalog renderer reads from Fields, which is
// misleading. The remedy is for the renderer to add a typed Get; until
// then, filepath is rejected.
func TestDetailPlugin_RejectsFilepathInLayout(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Detail",
		Kind:   "detail",
		Layout: "filepath",
	}
	_, err := parsePluginConfig(cfg, "test", schema, nil)
	if err == nil {
		t.Fatal("expected filepath to be rejected from layout")
	}
	if !strings.Contains(err.Error(), "filepath") {
		t.Errorf("expected error to mention filepath, got: %v", err)
	}
}

// TestDetailPlugin_AcceptsValidCaptionMarkup asserts that a literal caption
// carrying valid `<role>` color markup loads without error. Roles are
// drawn from workflow.ValidRoles; the loader parses but does not resolve
// the role to a concrete color (that happens at render time).
func TestDetailPlugin_AcceptsValidCaptionMarkup(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Detail",
		Kind:   "detail",
		Layout: `<danger>!!! Status: | status`,
	}
	if _, err := parsePluginConfig(cfg, "test", schema, nil); err != nil {
		t.Errorf("valid <role> caption should be accepted, got: %v", err)
	}
}

// TestDetailPlugin_RejectsUnknownCaptionRole pins that an unknown role name
// in a literal caption surfaces as a workflow-load error rather than
// rendering as broken text at first paint.
func TestDetailPlugin_RejectsUnknownCaptionRole(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Detail",
		Kind:   "detail",
		Layout: `<nope>Status: | status`,
	}
	_, err := parsePluginConfig(cfg, "test", schema, nil)
	if err == nil {
		t.Fatal("expected error for unknown caption role, got nil")
	}
	if !strings.Contains(err.Error(), "layout caption") {
		t.Errorf("expected error to mention 'layout caption', got: %v", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("expected error to mention the bad role name, got: %v", err)
	}
}

// TestDetailPlugin_AllowsTitleInLayout asserts title is accepted in the
// layout grid as a layout reservation — the title primitive renders
// outside the grid; the cell occupies space only.
func TestDetailPlugin_AllowsTitleInLayout(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Detail",
		Kind:   "detail",
		Layout: "title",
	}
	if _, err := parsePluginConfig(cfg, "test", schema, nil); err != nil {
		t.Errorf("title should be accepted as a layout reservation, got: %v", err)
	}
}

// TestDetailPlugin_RejectsIdentityFieldsInLayout asserts that description,
// id, and body cannot be configured in the layout grid — they are always
// rendered by the detail view chrome.
func TestDetailPlugin_RejectsIdentityFieldsInLayout(t *testing.T) {
	schema := testSchema()
	for _, name := range []string{"description", "id", "body"} {
		t.Run(name, func(t *testing.T) {
			cfg := pluginFileConfig{
				Name:   "Detail",
				Kind:   "detail",
				Layout: name,
			}
			_, err := parsePluginConfig(cfg, "test", schema, nil)
			if err == nil {
				t.Fatalf("expected error for identity field %q, got nil", name)
			}
			if !strings.Contains(err.Error(), "always rendered") {
				t.Errorf("expected error to mention 'always rendered', got %q", err.Error())
			}
		})
	}
}

// TestDetailPlugin_RejectsInvalidConfigKeys asserts that detail-only-invalid
// fields produce errors. path:/document:/lanes: belong to other kinds.
// mode: is rejected globally by rejectLegacyTopLevel (see TestLegacyFieldRejection).
func TestDetailPlugin_RejectsInvalidConfigKeys(t *testing.T) {
	schema := testSchema()
	validLayout := "status"
	cases := []struct {
		name      string
		cfg       pluginFileConfig
		wantError string
	}{
		{"path rejected", pluginFileConfig{Name: "D", Kind: "detail", Path: "x.md", Layout: validLayout}, "path:"},
		{"document rejected", pluginFileConfig{Name: "D", Kind: "detail", Document: "ABC", Layout: validLayout}, "document:"},
		{"lanes rejected", pluginFileConfig{Name: "D", Kind: "detail", Lanes: []PluginLaneConfig{{Name: "x"}}, Layout: validLayout}, "lanes:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePluginConfig(tc.cfg, "test", schema, nil)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("expected error mentioning %q, got %q", tc.wantError, err.Error())
			}
		})
	}
}

// TestDetailPlugin_AllowsPerViewActions asserts that kind: detail accepts
// per-view actions:, parsed via the same path used by board/list views.
func TestDetailPlugin_AllowsPerViewActions(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Detail",
		Kind:   "detail",
		Layout: "status",
		Actions: []PluginActionConfig{
			{
				Key:    "a",
				Label:  "Assign me",
				Action: `update where id = id() set assignee=user()`,
			},
		},
	}
	p, err := parsePluginConfig(cfg, "test", schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dp, ok := p.(*DetailPlugin)
	if !ok {
		t.Fatalf("expected *DetailPlugin, got %T", p)
	}
	if len(dp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(dp.Actions))
	}
	if dp.Actions[0].Label != "Assign me" {
		t.Errorf("action label = %q, want %q", dp.Actions[0].Label, "Assign me")
	}
}

// TestDetailPlugin_AllowsViewKindActions asserts that detail views can declare
// kind: view actions to navigate to other views (the same passthrough used by
// board views).
func TestDetailPlugin_AllowsViewKindActions(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:   "Detail",
		Kind:   "detail",
		Layout: "status",
		Actions: []PluginActionConfig{
			{Key: "F4", Label: "Backlog", Kind: "view", View: "Backlog"},
		},
	}
	viewNames := map[string]ViewKind{"Detail": KindDetail, "Backlog": KindBoard}
	p, err := parsePluginConfig(cfg, "test", schema, viewNames)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dp, ok := p.(*DetailPlugin)
	if !ok {
		t.Fatalf("expected *DetailPlugin, got %T", p)
	}
	if len(dp.Actions) != 1 || dp.Actions[0].Kind != ActionKindView || dp.Actions[0].TargetView != "Backlog" {
		t.Errorf("expected single kind:view action targeting Backlog, got %+v", dp.Actions)
	}
}

// TestKeyParser_AcceptsEnter asserts the named-key parser recognizes Enter
// (added so workflow.yaml can declare `key: Enter` for kind:view actions).
func TestKeyParser_AcceptsEnter(t *testing.T) {
	key, _, _, keyStr, err := parseCanonicalKey("Enter")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if keyStr != "Enter" {
		t.Errorf("keyStr = %q, want %q", keyStr, "Enter")
	}
	if key == 0 {
		t.Error("expected non-zero key for Enter")
	}
}

func TestDetailPlugin_RejectsUnknownFieldRole(t *testing.T) {
	schema := testSchema()
	raw := "<nosuchrole>title | --\nstatus | type"
	_, err := validateLayout("Test", "detail", raw, schema)
	if err == nil {
		t.Fatal("expected error for unknown field role")
	}
	if !strings.Contains(err.Error(), "nosuchrole") {
		t.Errorf("error should mention the bad role name, got: %v", err)
	}
}

func TestDetailPlugin_AcceptsKnownFieldRole(t *testing.T) {
	schema := testSchema()
	raw := "<highlight>title | --\nstatus | type"
	_, err := validateLayout("Test", "detail", raw, schema)
	if err != nil {
		t.Fatalf("unexpected error for valid field role: %v", err)
	}
}

func TestDetailPlugin_AcceptsFieldRoleWithModifier(t *testing.T) {
	schema := testSchema()
	raw := "<text.muted.accent>title | --\nstatus | type"
	_, err := validateLayout("Test", "detail", raw, schema)
	if err != nil {
		t.Fatalf("unexpected error for role with known modifier: %v", err)
	}
}

func TestDetailPlugin_RejectsUnknownFieldModifier(t *testing.T) {
	schema := testSchema()
	// Token "<text.muted.bogus>" — "bogus" is not a known modifier, so the
	// splitter treats the whole thing as the role name, which is then
	// rejected as an unknown role. Either way the validator must error.
	raw := "<text.muted.bogus>title | --\nstatus | type"
	_, err := validateLayout("Test", "detail", raw, schema)
	if err == nil {
		t.Fatal("expected error for unknown modifier")
	}
}

// TestWikiKind_BuildsWikiPlugin asserts that kind: wiki parses to a WikiPlugin
// (markdown-view path), distinct from the DetailPlugin used by kind: detail.
func TestWikiKind_BuildsWikiPlugin(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name: "Docs",
		Kind: "wiki",
		Path: "index.md",
	}
	p, err := parsePluginConfig(cfg, "test", schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*WikiPlugin); !ok {
		t.Errorf("expected *WikiPlugin for kind: wiki, got %T", p)
	}
}
