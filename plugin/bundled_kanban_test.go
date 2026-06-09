package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/ruki"
	"github.com/boolean-maybe/tiki/gridlayout"
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
		"due": true, "recurrence": true, "tags": true, "dependsOn": true,
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

			factory := ruki.DocumentFactory(func() ruki.Document { return tikipkg.WrapDoc(tikipkg.New()) })
			executor := ruki.NewExecutor(testSchema(), factory, nil,
				ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimePlugin})
			input := ruki.NewSingleSelectionInput(project.ID())
			candidateDocs, err := executor.EvalSubQueryFilter(action.ChooseFilter, tikipkg.WrapDocs(all), input)
			if err != nil {
				t.Fatalf("EvalSubQueryFilter: %v", err)
			}
			candidates := tikipkg.UnwrapDocs(candidateDocs)

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

// TestBundledKanban_DetailMakeRecurringSetsRecurrenceAndDue pins the Detail
// view's "Make recurring" action (key "R"). It must set recurrence to the
// daily cron and, in the same statement, set due to the next occurrence — the
// same recurrence/due coupling the edit path (SaveRecurrence) and the
// recurring-completion trigger produce. The action is authored with ruki's
// recurrence constructor daily() (added in ruki v0.1.1); a string literal would
// fail validation because recurrence is a derived-only type.
func TestBundledKanban_DetailMakeRecurringSetsRecurrenceAndDue(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	src := filepath.Join(filepath.Dir(wd), "config", "workflows", "kanban.yaml")
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
		t.Fatal("bundled kanban does not contain a Detail view")
	}

	var action *PluginAction
	for i := range detail.Actions {
		if detail.Actions[i].KeyStr == "R" {
			action = &detail.Actions[i]
			break
		}
	}
	if action == nil {
		t.Fatal("Detail view has no \"R\" action")
	}
	if action.Kind != ActionKindRuki || action.Action == nil {
		t.Fatalf("Detail \"R\" action is not a ruki action: kind=%v", action.Kind)
	}
	// Must surface in the footer — a detail action with hot:false (ShowInHeader
	// false) registers and fires but is invisible to the user.
	if !action.ShowInHeader {
		t.Error("Detail \"R\" action has ShowInHeader=false; it would be hidden from the footer")
	}

	tk := tikipkg.New()
	tk.SetID("TASK01")
	tk.SetTitle("Real Tiki")
	tk.Set("type", "story")
	tk.Set("status", "ready")

	factory := ruki.DocumentFactory(func() ruki.Document { return tikipkg.WrapDoc(tikipkg.New()) })
	executor := ruki.NewExecutor(testSchema(), factory, nil,
		ruki.ExecutorRuntime{Mode: ruki.ExecutorRuntimePlugin})
	result, err := executor.Execute(action.Action, tikipkg.WrapDocs([]*tikipkg.Tiki{tk}),
		ruki.NewSingleSelectionInput(tk.ID()))
	if err != nil {
		t.Fatalf("execute \"r\" action: %v", err)
	}
	if result.Update == nil || len(result.Update.Updated) != 1 {
		t.Fatalf("expected one updated tiki, got %+v", result.Update)
	}

	updated := tikipkg.UnwrapDoc(result.Update.Updated[0])
	gotRecurrence, _, _ := updated.StringField(tikipkg.FieldRecurrence)
	if gotRecurrence != "0 0 * * *" {
		t.Errorf("recurrence = %q, want daily cron %q", gotRecurrence, "0 0 * * *")
	}
	gotDue, hasDue, _ := updated.TimeField(tikipkg.FieldDue)
	if !hasDue || gotDue.IsZero() {
		t.Errorf("due not set to next occurrence: hasDue=%v due=%v", hasDue, gotDue)
	}
}

// TestBundledKanban_DetailActionsAvoidBuiltinGlobals guards against a
// workflow-declared Detail action shadowing a built-in app global. The action
// registry is last-write-wins by rune (controller.ActionRegistry.Register), so
// a Detail action keyed "r" silently overrides the global Refresh once the
// workflow loads — the kind of collision that's invisible until a user can no
// longer refresh from the detail view. These runes are registered in Go for
// every detail surface (controller/actions.go + detail_controller.go) and must
// not be reused by the bundled Detail/Project views.
func TestBundledKanban_DetailActionsAvoidBuiltinGlobals(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	src := filepath.Join(filepath.Dir(wd), "config", "workflows", "kanban.yaml")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("bundled kanban not at expected path %s: %v", src, err)
	}
	plugins, _, errs := loadPluginsFromFile(src, testSchema())
	if len(errs) != 0 {
		t.Fatalf("bundled kanban did not load cleanly: %v", errs)
	}

	// Built-in detail-surface global runes: r=Refresh, q=Quit, f=Full screen,
	// s=Edit source, c=Chat, e=Edit. A workflow detail action must not reuse
	// these.
	reserved := map[string]string{
		"r": "Refresh", "q": "Quit", "f": "Full screen",
		"s": "Edit source", "c": "Chat", "e": "Edit",
	}
	for _, p := range plugins {
		dp, ok := p.(*DetailPlugin)
		if !ok {
			continue
		}
		for i := range dp.Actions {
			key := dp.Actions[i].KeyStr
			if builtin, clash := reserved[key]; clash {
				t.Errorf("Detail view %q action key=%q (label=%q) collides with built-in global %q",
					dp.Name, key, dp.Actions[i].Label, builtin)
			}
		}
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

// TestBundledKanban_DetailUsesFieldCaptions asserts the Detail view's layout
// references field-owned captions: the priority field appears as exactly one
// caption anchor (DisplayCaption) and one value anchor. (priority has a plain
// value cell; status's value lives inside a composite, so it isn't a standalone
// AnchorField — priority is the clean case to assert on.)
func TestBundledKanban_DetailUsesFieldCaptions(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	src := filepath.Join(filepath.Dir(wd), "config", "workflows", "kanban.yaml")
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
		t.Fatal("bundled kanban does not contain a Detail view")
	}

	var captionAnchors, valueAnchors int
	for _, a := range detail.Layout.Anchors {
		if a.Kind == gridlayout.AnchorField && a.Name == "priority" {
			if a.Display == gridlayout.DisplayCaption {
				captionAnchors++
			} else {
				valueAnchors++
			}
		}
	}
	if captionAnchors != 1 || valueAnchors != 1 {
		t.Errorf("priority: caption anchors=%d value anchors=%d, want 1 and 1", captionAnchors, valueAnchors)
	}
}

// TestBundledKanban_SizingGrammarMigration guards the column-sizing-grammar
// migration: the retired "<->" stretcher token must not appear anywhere in the
// bundled workflow, and the Detail view's list fields (tags, dependsOn) must be
// marked hide-when-empty via the `?` suffix (replacing the old list-type
// auto-hide).
func TestBundledKanban_SizingGrammarMigration(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	src := filepath.Join(filepath.Dir(wd), "config", "workflows", "kanban.yaml")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("bundled kanban not at expected path %s: %v", src, err)
	}
	if strings.Contains(string(raw), "<->") {
		t.Error("bundled kanban still contains retired <-> token; migrate to :fr")
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
		t.Fatal("bundled kanban does not contain a Detail view")
	}
	wantHide := map[string]bool{"tags": false, "dependsOn": false}
	for _, a := range detail.Layout.Anchors {
		if a.Kind == gridlayout.AnchorField && a.HideWhenEmpty {
			if _, ok := wantHide[a.Name]; ok {
				wantHide[a.Name] = true
			}
		}
	}
	for name, ok := range wantHide {
		if !ok {
			t.Errorf("Detail layout field %q is not marked hide-when-empty (`?`)", name)
		}
	}
}

// TestBundledKanban_DetailCaptionAnchorsAlignWithNames pins the precondition
// of the doubled-Tab bug: the bundled Detail layout carries a display-only
// `.caption` anchor for every field (so each field name appears more than
// once in the flat anchor list), and AnchorDisplays must stay positionally
// aligned with AnchorNames so edit-mode traversal can tell a caption anchor
// from its value anchor and skip the former. If this layout ever loses its
// caption anchors the bug can't recur, but the alignment contract is what the
// view's Tab traversal relies on, so guard it here against the real workflow.
func TestBundledKanban_DetailCaptionAnchorsAlignWithNames(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	src := filepath.Join(filepath.Dir(wd), "config", "workflows", "kanban.yaml")
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
		t.Fatal("bundled kanban does not contain a Detail view")
	}

	names := detail.Layout.AnchorNames()
	displays := detail.Layout.AnchorDisplays()
	if len(names) != len(displays) {
		t.Fatalf("AnchorNames (%d) and AnchorDisplays (%d) lengths differ — not positionally aligned",
			len(names), len(displays))
	}

	captionFields := map[string]bool{}
	valueFields := map[string]bool{}
	for i, name := range names {
		if displays[i] == gridlayout.DisplayCaption {
			captionFields[name] = true
		} else {
			valueFields[name] = true
		}
	}
	if len(captionFields) == 0 {
		t.Fatal("bundled Detail layout has no caption anchors — bug precondition gone; revisit this guard")
	}
	// Every field that has a caption anchor must also have a value anchor, and
	// thus appear at least twice in the flat list — exactly the shape that made
	// the caption a stray Tab stop before the fix.
	for name := range captionFields {
		if !valueFields[name] && name != "title" {
			t.Errorf("field %q has a caption anchor but no value anchor in the bundled Detail layout", name)
		}
	}
}
