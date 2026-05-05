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

	// Defaults declared by the plan: status, type, priority — in order.
	want := []string{"status", "type", "priority"}
	got := strings.Join(detail.Metadata, ",")
	if got != strings.Join(want, ",") {
		t.Errorf("Detail.Metadata = %v, want %v", detail.Metadata, want)
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
