package plugin

import (
	"strings"
	"testing"
)

// TestDetailPlugin_ParsesMetadata asserts kind: detail accepts metadata: and
// builds a DetailPlugin (not a DokiPlugin, the Phase-1 split).
func TestDetailPlugin_ParsesMetadata(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:     "Detail",
		Kind:     "detail",
		Metadata: []string{"status", "type", "priority"},
	}
	p, err := parsePluginConfig(cfg, "test", schema, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dp, ok := p.(*DetailPlugin)
	if !ok {
		t.Fatalf("expected *DetailPlugin, got %T", p)
	}
	got := strings.Join(dp.Metadata, ",")
	if got != "status,type,priority" {
		t.Errorf("metadata = %q, want %q", got, "status,type,priority")
	}
}

// TestDetailPlugin_RejectsUnknownMetadataField asserts that referencing a field
// the schema doesn't know fails the workflow load instead of silently skipping.
func TestDetailPlugin_RejectsUnknownMetadataField(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:     "Detail",
		Kind:     "detail",
		Metadata: []string{"status", "no_such_field"},
	}
	_, err := parsePluginConfig(cfg, "test", schema, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no_such_field") {
		t.Errorf("expected error mentioning the unknown field, got %q", err.Error())
	}
}

// TestDetailPlugin_RejectsFieldsTheRegistryCannotRender asserts that fields
// the schema accepts but the detail-view registry can't render are rejected
// at workflow-load time (so users see a clear error instead of silent
// "(no renderer)" placeholders at runtime). createdAt/createdBy/filepath are
// schema-known but not in the renderable set today.
func TestDetailPlugin_RejectsFieldsTheRegistryCannotRender(t *testing.T) {
	schema := testSchema()
	for _, name := range []string{"createdAt", "updatedAt", "createdBy", "filepath"} {
		t.Run(name, func(t *testing.T) {
			cfg := pluginFileConfig{
				Name:     "Detail",
				Kind:     "detail",
				Metadata: []string{name},
			}
			_, err := parsePluginConfig(cfg, "test", schema, nil)
			if err == nil {
				t.Fatalf("expected error for non-renderable field %q, got nil", name)
			}
			if !strings.Contains(err.Error(), "not renderable") {
				t.Errorf("expected error to mention 'not renderable', got %q", err.Error())
			}
		})
	}
}

// TestDetailPlugin_RejectsIdentityFieldsInMetadata asserts that title, description,
// id, and body cannot be configured in metadata since they are always rendered
// (or never rendered, for id which is in the title row).
func TestDetailPlugin_RejectsIdentityFieldsInMetadata(t *testing.T) {
	schema := testSchema()
	for _, name := range []string{"title", "description", "id", "body"} {
		t.Run(name, func(t *testing.T) {
			cfg := pluginFileConfig{
				Name:     "Detail",
				Kind:     "detail",
				Metadata: []string{name},
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
// fields produce errors. path:/document:/lanes:/mode: belong to other kinds.
func TestDetailPlugin_RejectsInvalidConfigKeys(t *testing.T) {
	schema := testSchema()
	cases := []struct {
		name      string
		cfg       pluginFileConfig
		wantError string
	}{
		{"path rejected", pluginFileConfig{Name: "D", Kind: "detail", Path: "x.md"}, "path:"},
		{"document rejected", pluginFileConfig{Name: "D", Kind: "detail", Document: "ABC"}, "document:"},
		{"lanes rejected", pluginFileConfig{Name: "D", Kind: "detail", Lanes: []PluginLaneConfig{{Name: "x"}}}, "lanes:"},
		{"mode rejected", pluginFileConfig{Name: "D", Kind: "detail", Mode: "compact"}, "mode:"},
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
		Name:     "Detail",
		Kind:     "detail",
		Metadata: []string{"status"},
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
		Name:     "Detail",
		Kind:     "detail",
		Metadata: []string{"status"},
		Actions: []PluginActionConfig{
			{Key: "F4", Label: "Backlog", Kind: "view", View: "Backlog"},
		},
	}
	viewNames := map[string]struct{}{"Detail": {}, "Backlog": {}}
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

// TestWikiPlugin_RejectsMetadata asserts metadata: is detail-only.
func TestWikiPlugin_RejectsMetadata(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:     "Docs",
		Kind:     "wiki",
		Path:     "index.md",
		Metadata: []string{"status"},
	}
	_, err := parsePluginConfig(cfg, "test", schema, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "metadata:") {
		t.Errorf("expected error mentioning metadata, got %q", err.Error())
	}
}

// TestBoardPlugin_RejectsMetadata asserts metadata: is detail-only.
func TestBoardPlugin_RejectsMetadata(t *testing.T) {
	schema := testSchema()
	cfg := pluginFileConfig{
		Name:     "Board",
		Kind:     "board",
		Lanes:    []PluginLaneConfig{{Name: "Todo", Filter: "select"}},
		Metadata: []string{"status"},
	}
	_, err := parsePluginConfig(cfg, "test", schema, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "metadata:") {
		t.Errorf("expected error mentioning metadata, got %q", err.Error())
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

// TestWikiPlugin_StillBuildsDokiPlugin asserts the Phase 1 split — wiki
// continues to use the markdown-view path (DokiPlugin), only detail moves to
// DetailPlugin.
func TestWikiPlugin_StillBuildsDokiPlugin(t *testing.T) {
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
	if _, ok := p.(*DokiPlugin); !ok {
		t.Errorf("expected *DokiPlugin for kind: wiki, got %T", p)
	}
}
