package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// setupPluginViewTest creates a test app with plugins loaded and test data
func setupPluginViewTest(t *testing.T) *testutil.TestApp {
	ta := testutil.NewTestApp(t)
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("Failed to load plugins: %v", err)
	}

	// Create tikis for Backlog plugin (status = backlog)
	tikis := []struct {
		id     string
		title  string
		status string
		typ    string
	}{
		{"000001", "First Backlog Tiki", "backlog", "story"},
		{"000002", "Second Backlog Tiki", "backlog", "bug"},
		{"000003", "Third Backlog Tiki", "backlog", "story"},
		{"000004", "Fourth Backlog Tiki", "backlog", "bug"},
		{"000005", "Todo Tiki (not in backlog)", "ready", "story"},
	}

	for _, tiki := range tikis {
		if err := testutil.CreateTestTiki(ta.TikiDir, tiki.id, tiki.title, tiki.status, tiki.typ); err != nil {
			t.Fatalf("Failed to create tiki: %v", err)
		}
	}

	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("Failed to reload tikis: %v", err)
	}

	return ta
}

// TestPluginView_GridNavigation verifies arrow key navigation in 4-column grid
func TestPluginView_GridNavigation(t *testing.T) {
	t.Skip("SimulationScreen test framework issue - navigation works correctly in actual app")
	ta := setupPluginViewTest(t)
	defer ta.Cleanup()

	// Navigate: Board → Backlog Plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone) // F3 = Backlog plugin
	ta.Draw()                                 // Redraw after view change

	// Verify we're on plugin view
	currentView := ta.NavController.CurrentView()
	if !model.IsPluginViewID(currentView.ViewID) {
		t.Fatalf("expected plugin view, got %v", currentView.ViewID)
	}

	// Get plugin config
	pluginConfig := ta.GetPluginConfig("Backlog")
	if pluginConfig == nil {
		t.Fatalf("Backlog plugin config not found")
	}

	// Initial selection should be 0
	if pluginConfig.GetSelectedIndex() != 0 {
		t.Errorf("initial selection = %d, want 0", pluginConfig.GetSelectedIndex())
	}

	// With 5 tikis in 4-column grid:
	// [0, 1, 2, 3]
	// [4]
	// Only index 0 can move down to index 4

	// Press Right arrow (move to next column in same row)
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Selection should move to index 1 (same row, next column)
	if pluginConfig.GetSelectedIndex() != 1 {
		t.Errorf("after Right, selection = %d, want 1", pluginConfig.GetSelectedIndex())
	}

	// Press Down arrow - should NOT move (no tiki in column 1 of row 2)
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Selection should stay at index 1 (can't move down to non-existent index 5)
	if pluginConfig.GetSelectedIndex() != 1 {
		t.Errorf("after Down from index 1, selection = %d, want 1 (no tiki below)", pluginConfig.GetSelectedIndex())
	}

	// Go back to index 0
	ta.SendKey(tcell.KeyLeft, 0, tcell.ModNone)
	if pluginConfig.GetSelectedIndex() != 0 {
		t.Errorf("after Left, selection = %d, want 0", pluginConfig.GetSelectedIndex())
	}

	// Press Down arrow from index 0 - should move to index 4
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Selection should move to index 4 (only valid down move)
	if pluginConfig.GetSelectedIndex() != 4 {
		t.Errorf("after Down from index 0, selection = %d, want 4", pluginConfig.GetSelectedIndex())
	}

	// Press Up arrow - should move back to index 0
	ta.SendKey(tcell.KeyUp, 0, tcell.ModNone)

	// Selection should move back to index 0
	if pluginConfig.GetSelectedIndex() != 0 {
		t.Errorf("after Up, selection = %d, want 0", pluginConfig.GetSelectedIndex())
	}
}

// TestPluginView_FilterByStatus verifies plugin filters tikis by status
func TestPluginView_FilterByStatus(t *testing.T) {
	ta := setupPluginViewTest(t)
	defer ta.Cleanup()

	// Navigate: Board → Backlog Plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone) // F3 = Backlog plugin

	// Verify backlog tikis are visible
	found1, _, _ := ta.FindText("000001")
	found2, _, _ := ta.FindText("000002")
	if !found1 || !found2 {
		ta.DumpScreen()
		t.Errorf("backlog tikis should be visible in backlog plugin")
	}

	// Verify non-backlog tiki is NOT visible
	found5, _, _ := ta.FindText("000005")
	if found5 {
		ta.DumpScreen()
		t.Errorf("todo tiki TIKI-5 should NOT be visible in backlog plugin")
	}
}

// TestPluginView_OpenTiki verifies Enter opens tiki detail from plugin
func TestPluginView_OpenTiki(t *testing.T) {
	ta := setupPluginViewTest(t)
	defer ta.Cleanup()

	// Navigate: Board → Backlog Plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone) // F3 = Backlog plugin

	// Press Enter to open first tiki
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Phase 3: configurable detail view (workflow `kind: view` action),
	// not the legacy TikiDetailViewID.
	currentView := ta.NavController.CurrentView()
	wantDetail := model.DetailPluginViewID()
	if currentView.ViewID != wantDetail {
		t.Errorf("expected detail view %s, got %v", wantDetail, currentView.ViewID)
	}

	// Verify correct tiki is displayed
	found, _, _ := ta.FindText("000001")
	if !found {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be displayed in tiki detail")
	}
}

// TestPluginView_CreateTiki verifies 'n' creates new tiki
// TestPluginView_DeleteTiki verifies 'd' deletes selected tiki
func TestPluginView_DeleteTiki(t *testing.T) {
	ta := setupPluginViewTest(t)
	defer ta.Cleanup()

	// Navigate: Board → Backlog Plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)

	// Verify TIKI-1 is visible
	found, _, _ := ta.FindText("000001")
	if !found {
		ta.DumpScreen()
		t.Fatalf("TIKI-1 should be visible before delete")
	}

	// Press 'd' to delete first tiki (TIKI-1)
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify tiki is deleted
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	deletedTiki := ta.TikiStore.GetTiki("000001")
	if deletedTiki != nil {
		t.Errorf("TIKI-1 should be deleted from store")
	}

	// Verify file is removed
	tikiPath := filepath.Join(ta.TikiDir, "000001.md")
	if _, err := os.Stat(tikiPath); !os.IsNotExist(err) {
		t.Errorf("TIKI-1 file should be deleted")
	}
}

// TestPluginView_Search verifies '/' opens search in plugin
func TestPluginView_Search(t *testing.T) {
	ta := setupPluginViewTest(t)
	defer ta.Cleanup()

	// Navigate: Board → Backlog Plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)

	// Verify multiple tikis visible initially
	found1, _, _ := ta.FindText("000001")
	found2, _, _ := ta.FindText("000002")
	if !found1 || !found2 {
		ta.DumpScreen()
		t.Fatalf("both tikis should be visible initially")
	}

	// Press '/' to open search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)

	// Verify search box is visible
	foundPrompt, _, _ := ta.FindText(">")
	if !foundPrompt {
		ta.DumpScreen()
		t.Errorf("search box prompt should be visible")
	}

	// Type search query
	ta.SendText("First")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify only TIKI-1 is visible
	found1After, _, _ := ta.FindText("000001")
	found2After, _, _ := ta.FindText("000002")
	if !found1After {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible after search")
	}
	if found2After {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should NOT be visible after search")
	}
}

// TestPluginView_SearchNoResults verifies search with no matches shows empty results
func TestPluginView_SearchNoResults(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("Failed to load plugins: %v", err)
	}

	// Create a single tiki
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "First Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to plugin view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Verify tiki is visible before search
	foundBefore, _, _ := ta.FindText("000001")
	if !foundBefore {
		ta.DumpScreen()
		t.Fatalf("TIKI-1 should be visible before search")
	}

	// Start search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)

	// Type non-matching search query
	ta.SendText("xyznonexistent")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify tiki is NOT visible after no-match search
	foundAfter, _, _ := ta.FindText("000001")
	if foundAfter {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should NOT be visible after no-match search")
	}

	// Press Escape to clear search
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify tiki reappears
	foundCleared, _, _ := ta.FindText("000001")
	if !foundCleared {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should reappear after clearing search")
	}
}

// TestPluginView_EmptyPlugin verifies plugin view with no matching tikis
func TestPluginView_EmptyPlugin(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("Failed to load plugins: %v", err)
	}

	// Create only todo tikis (no backlog tikis)
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "Todo Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Backlog Plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone) // F3 = Backlog plugin

	// Verify no tikis are visible (empty plugin)
	found, _, _ := ta.FindText("000001")
	if found {
		ta.DumpScreen()
		t.Errorf("todo tiki should NOT be visible in backlog plugin")
	}

	// Verify we can still navigate (no crash)
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyUp, 0, tcell.ModNone)
	// Should not crash
}

// TestPluginView_NavigateBetweenColumns verifies horizontal navigation wraps
func TestPluginView_NavigateBetweenColumns(t *testing.T) {
	ta := setupPluginViewTest(t)
	defer ta.Cleanup()

	// Navigate: Board → Backlog Plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)

	pluginConfig := ta.GetPluginConfig("Backlog")
	if pluginConfig == nil {
		t.Fatalf("Backlog plugin config not found")
	}

	// Start at index 0
	if pluginConfig.GetSelectedIndex() != 0 {
		t.Fatalf("initial selection = %d, want 0", pluginConfig.GetSelectedIndex())
	}

	// Press Right 3 times to reach column 3 (index 3)
	for i := 0; i < 3; i++ {
		ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)
	}

	if pluginConfig.GetSelectedIndex() != 3 {
		t.Errorf("after 3x Right, selection = %d, want 3", pluginConfig.GetSelectedIndex())
	}

	// Press Right again - should wrap or stay at boundary
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Verify no crash and selection is valid
	selectedIndex := pluginConfig.GetSelectedIndex()
	if selectedIndex < 0 {
		t.Errorf("selection should be valid, got %d", selectedIndex)
	}
}

// TestPluginView_EscAtRootDoesNothing verifies Esc at root plugin does nothing
func TestPluginView_EscAtRootDoesNothing(t *testing.T) {
	ta := setupPluginViewTest(t)
	defer ta.Cleanup()

	// Start at Kanban, switch to Backlog (uses ReplaceView, so still at depth 1)
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)

	// Verify we're on Backlog plugin view at depth 1
	currentView := ta.NavController.CurrentView()
	if currentView.ViewID != model.MakePluginViewID("Backlog") {
		t.Fatalf("expected Backlog plugin view, got %v", currentView.ViewID)
	}
	if ta.NavController.Depth() != 1 {
		t.Fatalf("expected depth 1, got %d", ta.NavController.Depth())
	}

	// Press Esc - should do nothing since we're at root
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify we're still on Backlog (Esc does nothing at root)
	currentView = ta.NavController.CurrentView()
	if currentView.ViewID != model.MakePluginViewID("Backlog") {
		t.Errorf("expected to stay on Backlog after Esc at root, got %v", currentView.ViewID)
	}
}

// TestPluginView_MultiplePlugins verifies switching between different plugins
func TestPluginView_MultiplePlugins(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("Failed to load plugins: %v", err)
	}

	// Create tikis for multiple plugins
	// Backlog: status = backlog (also recent since just created)
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "Backlog Tiki", "backlog", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}

	// Recent: status = todo (also recent since just created)
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "Recent Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}

	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Board → Backlog Plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone) // F3 = Backlog

	// Verify only backlog tiki visible in Backlog plugin
	found1, _, _ := ta.FindText("000001")
	if !found1 {
		ta.DumpScreen()
		t.Errorf("backlog tiki should be visible in backlog plugin")
	}

	found2InBacklog, _, _ := ta.FindText("000002")
	if found2InBacklog {
		ta.DumpScreen()
		t.Errorf("todo tiki should NOT be visible in backlog plugin (filtered by status)")
	}

	// Switch to Recent plugin (Ctrl-R)
	ta.SendKey(tcell.KeyRune, 'R', tcell.ModCtrl)

	// Verify BOTH tikis visible in Recent plugin (both were just created)
	// Recent shows all recently modified/created tikis regardless of status
	found1InRecent, _, _ := ta.FindText("000001")
	if !found1InRecent {
		ta.DumpScreen()
		t.Errorf("backlog tiki should be visible in recent plugin (recently created)")
	}

	found2InRecent, _, _ := ta.FindText("000002")
	if !found2InRecent {
		ta.DumpScreen()
		t.Errorf("todo tiki should be visible in recent plugin (recently created)")
	}
}

// TestPluginView_ViKeysNavigation verifies vi-style keys (h/j/k/l) work in plugin
func TestPluginView_ViKeysNavigation(t *testing.T) {
	t.Skip("SimulationScreen test framework issue - navigation works correctly in actual app")
	ta := setupPluginViewTest(t)
	defer ta.Cleanup()

	// Navigate: Board → Backlog Plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)

	pluginConfig := ta.GetPluginConfig("Backlog")
	if pluginConfig == nil {
		t.Fatalf("Backlog plugin config not found")
	}

	// Start at index 0
	if pluginConfig.GetSelectedIndex() != 0 {
		t.Fatalf("initial selection = %d, want 0", pluginConfig.GetSelectedIndex())
	}

	// With 5 tikis in 4-column grid: [0, 1, 2, 3] / [4]

	// Press 'l' (vi Right)
	ta.SendKey(tcell.KeyRune, 'l', tcell.ModNone)
	ta.Draw() // Redraw after navigation

	if pluginConfig.GetSelectedIndex() != 1 {
		t.Errorf("after 'l', selection = %d, want 1", pluginConfig.GetSelectedIndex())
	}

	// Press 'j' (vi Down) - should NOT move (no tiki at index 5)
	ta.SendKey(tcell.KeyRune, 'j', tcell.ModNone)

	if pluginConfig.GetSelectedIndex() != 1 {
		t.Errorf("after 'j' from index 1, selection = %d, want 1 (no tiki below)", pluginConfig.GetSelectedIndex())
	}

	// Go back to index 0
	ta.SendKey(tcell.KeyRune, 'h', tcell.ModNone)
	if pluginConfig.GetSelectedIndex() != 0 {
		t.Errorf("after 'h', selection = %d, want 0", pluginConfig.GetSelectedIndex())
	}

	// Press 'j' (vi Down) from index 0 - should move to index 4
	ta.SendKey(tcell.KeyRune, 'j', tcell.ModNone)

	if pluginConfig.GetSelectedIndex() != 4 {
		t.Errorf("after 'j' from index 0, selection = %d, want 4", pluginConfig.GetSelectedIndex())
	}

	// Press 'k' (vi Up) - should move back to index 0
	ta.SendKey(tcell.KeyRune, 'k', tcell.ModNone)

	if pluginConfig.GetSelectedIndex() != 0 {
		t.Errorf("after 'k', selection = %d, want 0", pluginConfig.GetSelectedIndex())
	}
}

// TestPluginView_SelectionPersistsAcrossViews verifies selection is maintained
func TestPluginView_SelectionPersistsAcrossViews(t *testing.T) {
	ta := setupPluginViewTest(t)
	defer ta.Cleanup()

	// Navigate: Board → Backlog Plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyF3, 0, tcell.ModNone)

	pluginConfig := ta.GetPluginConfig("Backlog")
	if pluginConfig == nil {
		t.Fatalf("Backlog plugin config not found")
	}

	// Move to index 2
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	if pluginConfig.GetSelectedIndex() != 2 {
		t.Fatalf("selection = %d, want 2", pluginConfig.GetSelectedIndex())
	}

	// Open tiki detail
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Go back to plugin
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify selection is still at index 2
	if pluginConfig.GetSelectedIndex() != 2 {
		t.Errorf("selection after return = %d, want 2 (should be preserved)", pluginConfig.GetSelectedIndex())
	}
}
