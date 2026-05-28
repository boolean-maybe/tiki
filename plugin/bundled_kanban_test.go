package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
