package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadPluginsFromFile_DetailWithEnterAction asserts the Phase 1 wire-up:
// a workflow declaring Detail + Enter action loads cleanly, parses the action
// as a kind: view, and produces a DetailPlugin with the configured metadata.
func TestLoadPluginsFromFile_DetailWithEnterAction(t *testing.T) {
	tmpDir := t.TempDir()
	content := `actions:
  - key: Enter
    label: Open
    kind: view
    view: Detail
    require: ["selection:one"]
views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Ready
        filter: select where status = "ready"
  - name: Detail
    kind: detail
    require: ["selection:one"]
    metadata: [status, type, priority]
`
	path := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	plugins, globals, errs := loadPluginsFromFile(path, testSchema())
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}

	var detail *DetailPlugin
	for _, p := range plugins {
		if dp, ok := p.(*DetailPlugin); ok {
			detail = dp
			break
		}
	}
	if detail == nil {
		t.Fatal("expected a DetailPlugin in loaded plugins")
	}
	wantMeta := []string{"status", "type", "priority"}
	if strings.Join(detail.Metadata, ",") != strings.Join(wantMeta, ",") {
		t.Errorf("metadata = %v, want %v", detail.Metadata, wantMeta)
	}

	// global Enter action should have parsed as kind: view targeting Detail.
	var enterAction *PluginAction
	for i := range globals {
		if globals[i].KeyStr == "Enter" {
			enterAction = &globals[i]
			break
		}
	}
	if enterAction == nil {
		t.Fatal("expected an Enter global action")
	}
	if enterAction.Kind != ActionKindView {
		t.Errorf("Enter kind = %v, want %v", enterAction.Kind, ActionKindView)
	}
	if enterAction.TargetView != "Detail" {
		t.Errorf("Enter target = %q, want %q", enterAction.TargetView, "Detail")
	}
}

// TestLoadPluginsFromFile_DropsSelfTargetingViewGlobalFromTarget asserts the
// merge-time filter for self-targeting view actions: a global declaring
// `Enter → Detail` should be merged into Board/Backlog/etc. but NOT appended
// to Detail's own Actions slice. Without this, pressing Enter on the active
// Detail view would push another identical Detail copy onto the stack
// (and could race with the markdown viewer's own Enter handler).
func TestLoadPluginsFromFile_DropsSelfTargetingViewGlobalFromTarget(t *testing.T) {
	tmpDir := t.TempDir()
	content := `actions:
  - key: Enter
    label: Open
    kind: view
    view: Detail
    require: ["selection:one"]
views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Ready
        filter: select where status = "ready"
  - name: Detail
    kind: detail
    require: ["selection:one"]
    metadata: [status]
`
	path := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	plugins, err := LoadPluginsFromFile(path, testSchema())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	var detail *DetailPlugin
	var board *TikiPlugin
	for _, p := range plugins {
		switch v := p.(type) {
		case *DetailPlugin:
			detail = v
		case *TikiPlugin:
			board = v
		}
	}
	if detail == nil || board == nil {
		t.Fatalf("expected both DetailPlugin and TikiPlugin; got detail=%v board=%v", detail, board)
	}

	// Detail must NOT carry a self-target Enter action.
	for _, a := range detail.Actions {
		if a.KeyStr == "Enter" && a.Kind == ActionKindView && a.TargetView == "Detail" {
			t.Errorf("Detail.Actions still contains self-targeting Enter → Detail")
		}
	}

	// Board MUST carry it (this is the dispatch path users rely on).
	var boardEnter *PluginAction
	for i := range board.Actions {
		if board.Actions[i].KeyStr == "Enter" {
			boardEnter = &board.Actions[i]
			break
		}
	}
	if boardEnter == nil {
		t.Fatal("Board.Actions missing the merged Enter global")
	}
	if boardEnter.TargetView != "Detail" {
		t.Errorf("Board Enter target = %q, want %q", boardEnter.TargetView, "Detail")
	}
}

// TestLoadPluginsFromFile_RejectsEnterActionTargetingMissingView guards the
// existing cross-view validation: Enter → unknown view should fail load.
func TestLoadPluginsFromFile_RejectsEnterActionTargetingMissingView(t *testing.T) {
	tmpDir := t.TempDir()
	content := `actions:
  - key: Enter
    label: Open
    kind: view
    view: NoSuchView
views:
  - name: Board
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Ready
        filter: select where status = "ready"
`
	path := filepath.Join(tmpDir, "workflow.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	_, _, errs := loadPluginsFromFile(path, testSchema())
	if len(errs) == 0 {
		t.Fatal("expected error for unknown target view, got none")
	}
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "NoSuchView") {
		t.Errorf("expected error to mention unknown view, got: %s", joined)
	}
}
