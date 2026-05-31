package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/ruki"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestBundledKanban_HasDetailViewAndEnterAction asserts the Phase 1 update
// to the bundled kanban workflow: it must declare a Detail view (kind: detail)
// with the default metadata, and an Enter action targeting it. The test loads
// the YAML directly from the source path under config/workflows/.
//
// Run from the plugin package because we want to exercise the same loader path
// users hit at startup.
func TestBundledKanban_HasDetailViewAndEnterAction(t *testing.T) {
	// Locate config/workflows/kanban.yaml relative to the repo root. The
	// `plugin` package lives one level below the repo root, so step up.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	src := filepath.Join(repoRoot, "config", "workflows", "kanban.yaml")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("bundled kanban not at expected path %s: %v", src, err)
	}

	plugins, _, errs := loadPluginsFromFile(src, testSchema())
	if len(errs) != 0 {
		t.Fatalf("bundled kanban did not load cleanly: %v", errs)
	}

	var detail *DetailPlugin
	for _, p := range plugins {
		if dp, ok := p.(*DetailPlugin); ok && dp.Name == "Detail" {
			detail = dp
			break
		}
	}
	if detail == nil {
		t.Fatal("bundled kanban does not contain a Detail (kind: detail) view")
	}

	// Defaults declared by bundled kanban — the parsed grid's anchor
	// declaration order is the equivalent of the pre-grid flat list.
	wantAnchors := map[string]bool{
		"status": true, "type": true, "priority": true, "points": true,
		"assignee": true, "createdBy": true, "createdAt": true, "updatedAt": true,
		"due": true, "recurrence": true, "tags": true,
	}
	gotAnchors := detail.Layout.AnchorNames()
	for _, n := range gotAnchors {
		if n == "title" {
			continue // layout reservation, not a renderable field
		}
		if !wantAnchors[n] {
			t.Errorf("unexpected anchor in Detail.Layout: %q (full list: %s)", n, strings.Join(gotAnchors, ","))
		}
	}
	for n := range wantAnchors {
		found := false
		for _, g := range gotAnchors {
			if g == n {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing anchor in Detail.Layout: %q (full list: %s)", n, strings.Join(gotAnchors, ","))
		}
	}

	// Enter (Open) is a view-local action on board views, not a global.
	// Verify it lives on the Kanban board's action list.
	var kanban *WorkflowPlugin
	for _, p := range plugins {
		if wp, ok := p.(*WorkflowPlugin); ok && wp.Name == "Kanban" {
			kanban = wp
			break
		}
	}
	if kanban == nil {
		t.Fatal("bundled kanban does not contain a Kanban board view")
	}
	var enter *PluginAction
	for i := range kanban.Actions {
		if kanban.Actions[i].KeyStr == "Enter" {
			enter = &kanban.Actions[i]
			break
		}
	}
	if enter == nil {
		t.Fatal("Kanban board has no Enter action")
	}
	if enter.Kind != ActionKindView {
		t.Errorf("Enter.Kind = %v, want %v", enter.Kind, ActionKindView)
	}
	if enter.TargetView != "Detail" {
		t.Errorf("Enter.TargetView = %q, want %q", enter.TargetView, "Detail")
	}
}

// TestBundledKanban_GlobalsAreSelectionFree asserts the bundled kanban's
// top-level actions: block contains only actions that don't depend on a
// row-cursor selection. Selection-bound shortcuts (e/n/a/+/-, Enter, Ctrl-D,
// Ctrl-T) must live on individual board views, not as workflow globals —
// globals merge into every plugin (including wiki/Docs and detail views),
// where they surface uselessly.
func TestBundledKanban_GlobalsAreSelectionFree(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	src := filepath.Join(repoRoot, "config", "workflows", "kanban.yaml")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("bundled kanban not at expected path %s: %v", src, err)
	}

	_, globals, errs := loadPluginsFromFile(src, testSchema())
	if len(errs) != 0 {
		t.Fatalf("bundled kanban did not load cleanly: %v", errs)
	}

	// Keys that must NOT appear in the workflow-level actions: block.
	// Each is selection-bound (operates on the row cursor) and belongs on
	// individual board views.
	forbiddenAtGlobal := map[string]bool{
		"Enter":  true,
		"e":      true,
		"n":      true,
		"a":      true,
		"+":      true,
		"-":      true,
		"Ctrl-D": true,
		"Ctrl-T": true,
	}
	for i := range globals {
		a := &globals[i]
		if forbiddenAtGlobal[a.KeyStr] {
			t.Errorf("global action key=%q (label=%q) must live on a board view, not at workflow top level",
				a.KeyStr, a.Label)
		}
	}
}

// TestBundledKanban_AddToProjectExcludesPlainDocs pins that the bundled
// kanban's "Add to project" choose filters skip plain documents (e.g. doki
// section indexes that carry only `id:` and `title:` frontmatter, no workflow
// fields). The leak surfaces in the QuickSelect picker as bare-ID rows mixed
// with titled tikis — see picker render at view/palette/quick_select.go.
//
// Two callsites exercise this filter:
//   - Kanban board action key "l" (line 226): "Add tiki to project"
//   - Project detail action key "a" (line 284): "Add to project"
//
// Both predicates must include `has(type)`. Without it, ruki's missing-field
// rule (`absent != value` is true) lets plain docs through.
func TestBundledKanban_AddToProjectExcludesPlainDocs(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	src := filepath.Join(repoRoot, "config", "workflows", "kanban.yaml")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("bundled kanban not at expected path %s: %v", src, err)
	}

	plugins, _, errs := loadPluginsFromFile(src, testSchema())
	if len(errs) != 0 {
		t.Fatalf("bundled kanban did not load cleanly: %v", errs)
	}

	// The candidate set: a real workflow tiki, a project (must be excluded by
	// the predicate type != "project"), and a plain doc with no workflow
	// fields (must be excluded by the new has(type) guard).
	project := tikipkg.New()
	project.SetID("PROJ01")
	project.SetTitle("Sample Project")
	project.Set("type", "project")
	project.Set("status", "ready")

	tk := tikipkg.New()
	tk.SetID("TASK01")
	tk.SetTitle("Real Tiki")
	tk.Set("type", "story")
	tk.Set("status", "ready")

	plainDoc := tikipkg.New()
	plainDoc.SetID("DOKI01")
	plainDoc.SetTitle("Section Index")
	// no workflow fields — mirrors a doki index.md with only id+title

	all := []*tikipkg.Tiki{project, tk, plainDoc}

	cases := []struct {
		viewName string
		key      string
		label    string
	}{
		{viewName: "Roadmap", key: "l", label: "Add tiki to project"},
		{viewName: "Project", key: "a", label: "Add to project"},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			action := findChooseAction(t, plugins, c.viewName, c.key)
			if action.ChooseFilter == nil {
				t.Fatalf("action %q has no ChooseFilter", c.label)
			}

			executor := ruki.NewExecutor(testSchema(), nil,
				ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimePlugin})
			input := ruki.NewSingleSelectionInput(project.ID())
			candidates, err := executor.EvalSubQueryFilter(action.ChooseFilter, all, input)
			if err != nil {
				t.Fatalf("EvalSubQueryFilter: %v", err)
			}

			ids := map[string]bool{}
			for _, ct := range candidates {
				ids[ct.ID()] = true
			}
			if !ids["TASK01"] {
				t.Errorf("expected workflow tiki TASK01 in candidates, got %v", idList(candidates))
			}
			if ids["PROJ01"] {
				t.Errorf("project PROJ01 must not appear in candidates (filter excludes type=project): %v",
					idList(candidates))
			}
			if ids["DOKI01"] {
				t.Errorf("plain doc DOKI01 must not appear in candidates (filter must require has(type)): %v",
					idList(candidates))
			}
		})
	}
}

// findChooseAction locates a choose-bearing PluginAction by view name and key.
// Works for both board-style WorkflowPlugin views and DetailPlugin views.
func findChooseAction(t *testing.T, plugins []Plugin, viewName, key string) *PluginAction {
	t.Helper()
	matchOnView := func(name string, actions []PluginAction) *PluginAction {
		if name != viewName {
			return nil
		}
		for i := range actions {
			if actions[i].KeyStr == key && actions[i].HasChoose {
				return &actions[i]
			}
		}
		return nil
	}
	for _, p := range plugins {
		var match *PluginAction
		switch v := p.(type) {
		case *WorkflowPlugin:
			match = matchOnView(v.Name, v.Actions)
		case *DetailPlugin:
			match = matchOnView(v.Name, v.Actions)
		}
		if match != nil {
			return match
		}
	}
	t.Fatalf("no choose action found for view=%q key=%q", viewName, key)
	return nil
}

func idList(tikis []*tikipkg.Tiki) []string {
	out := make([]string, 0, len(tikis))
	for _, t := range tikis {
		out = append(out, t.ID())
	}
	return out
}
