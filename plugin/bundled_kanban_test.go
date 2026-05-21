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

	plugins, globals, errs := loadPluginsFromFile(src, testSchema())
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

	var enter *PluginAction
	for i := range globals {
		if globals[i].KeyStr == "Enter" {
			enter = &globals[i]
			break
		}
	}
	if enter == nil {
		t.Fatal("bundled kanban has no global Enter action")
	}
	if enter.Kind != ActionKindView {
		t.Errorf("Enter.Kind = %v, want %v", enter.Kind, ActionKindView)
	}
	if enter.TargetView != "Detail" {
		t.Errorf("Enter.TargetView = %q, want %q", enter.TargetView, "Detail")
	}
}

// TestBundledKanban_DeclaresAllDetailModeActions asserts the bundled kanban
// workflow exposes the five detail-view modes as top-level actions, replacing
// the previously hardcoded e / n / Ctrl-D / Ctrl-T shortcuts. Each action key
// must target the Detail view with the matching mode.
func TestBundledKanban_DeclaresAllDetailModeActions(t *testing.T) {
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

	wantByKey := map[string]DetailMode{
		"Enter":  DetailModeView,
		"e":      DetailModeEdit,
		"n":      DetailModeNew,
		"Ctrl-D": DetailModeEditDesc,
		"Ctrl-T": DetailModeEditTags,
	}
	for i := range globals {
		a := &globals[i]
		if a.Kind != ActionKindView || a.TargetView != "Detail" {
			continue
		}
		want, ok := wantByKey[a.KeyStr]
		if !ok {
			continue
		}
		// DetailModeView is the default and may be encoded as either "" or "view".
		got := a.Mode
		if want == DetailModeView && got == "" {
			got = DetailModeView
		}
		if got != want {
			t.Errorf("action key=%q: Mode = %q, want %q", a.KeyStr, a.Mode, want)
		}
		delete(wantByKey, a.KeyStr)
	}
	for key := range wantByKey {
		t.Errorf("missing detail-mode action for key %q", key)
	}
}
