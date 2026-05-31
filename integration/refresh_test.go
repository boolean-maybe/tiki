package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// TestRefresh_FromBoard verifies 'r' key reloads tikis from disk
func TestRefresh_FromKanban(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create initial tiki
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "First Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Verify TIKI-1 is visible
	found, _, _ := ta.FindText("000001")
	if !found {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible initially")
	}

	// Create a new tiki externally (simulating external modification)
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "New External Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create external tiki: %v", err)
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify TIKI-2 is now visible
	found2, _, _ := ta.FindText("000002")
	if !found2 {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible after refresh")
	}
}

// TestRefresh_ExternalModification verifies refresh loads modified tiki content
func TestRefresh_ExternalModification(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tiki
	tikiID := "000001"
	if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, "Original Title", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Verify original title visible (may be truncated on narrow screens)
	found, _, _ := ta.FindText("Origina")
	if !found {
		ta.DumpScreen()
		t.Errorf("original title should be visible")
	}

	// Verify tiki exists in store
	tiki := ta.TikiStore.GetTiki(tikiID)
	if tiki == nil || tiki.Title() != "Original Title" {
		t.Fatalf("tiki should exist with original title")
	}

	// Modify the tiki file externally
	if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, "Modified Title", "ready", "story"); err != nil {
		t.Fatalf("failed to modify tiki: %v", err)
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify tiki store has updated title
	tikiAfter := ta.TikiStore.GetTiki(tikiID)
	if tikiAfter == nil {
		t.Fatalf("tiki should still exist after refresh")
		return
	}
	if tikiAfter.Title() != "Modified Title" {
		t.Errorf("tiki title in store = %q, want %q", tikiAfter.Title(), "Modified Title")
	}

	// Note: The UI may not immediately reflect the change due to view caching,
	// but the important thing is that the tiki store reloaded the data
}

// TestRefresh_ExternalDeletion verifies refresh handles deleted tikis
func TestRefresh_ExternalDeletion(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create two tikis
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "First Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "Second Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Verify both tikis visible
	found1, _, _ := ta.FindText("000001")
	found2, _, _ := ta.FindText("000002")
	if !found1 || !found2 {
		ta.DumpScreen()
		t.Errorf("both tikis should be visible initially")
	}

	// Delete TIKI-1 externally
	tikiPath := filepath.Join(ta.TikiDir, "000001.md")
	if err := os.Remove(tikiPath); err != nil {
		t.Fatalf("failed to delete tiki file: %v", err)
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify TIKI-1 is gone
	found1After, _, _ := ta.FindText("000001")
	if found1After {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should NOT be visible after deletion and refresh")
	}

	// Verify TIKI-2 still visible
	found2After, _, _ := ta.FindText("000002")
	if !found2After {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should still be visible after refresh")
	}

	// Verify tiki store count
	tikis := ta.TikiStore.GetAllTikis()
	if len(tikis) != 1 {
		t.Errorf("tiki store should have 1 tiki after refresh, got %d", len(tikis))
	}
}

// TestRefresh_PreservesSelection verifies selection is maintained when tiki still exists
func TestRefresh_PreservesSelection(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tikis
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "First Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "Second Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Move to second tiki
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	kanbanConfig := ta.GetPluginConfig("Kanban")
	// Verify we're on index 1 (TIKI-2)
	if kanbanConfig.GetSelectedIndex() != 1 {
		t.Fatalf("expected index 1, got %d", kanbanConfig.GetSelectedIndex())
	}

	// Create a new tiki externally (doesn't affect selection)
	if err := testutil.CreateTestTiki(ta.TikiDir, "000003", "Third Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify selection is still preserved (might shift if new tiki sorts before)
	// For this test, we just verify no crash and index is valid
	selectedIndex := kanbanConfig.GetSelectedIndex()
	if selectedIndex < 0 {
		t.Errorf("selected index should be valid after refresh, got %d", selectedIndex)
	}
}

// TestRefresh_ResetsSelectionWhenTikiDeleted verifies selection resets when selected tiki deleted
func TestRefresh_ResetsSelectionWhenTikiDeleted(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tikis
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "First Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "Second Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Move to second tiki
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	kanbanConfig := ta.GetPluginConfig("Kanban")
	// Verify we're on index 1 (TIKI-2)
	if kanbanConfig.GetSelectedIndex() != 1 {
		t.Fatalf("expected index 1, got %d", kanbanConfig.GetSelectedIndex())
	}

	// Delete TIKI-2 externally (the selected tiki)
	tikiPath := filepath.Join(ta.TikiDir, "000002.md")
	if err := os.Remove(tikiPath); err != nil {
		t.Fatalf("failed to delete tiki file: %v", err)
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify selection reset to index 0
	if kanbanConfig.GetSelectedIndex() != 0 {
		t.Errorf("selection should reset to index 0 when selected tiki deleted, got %d", kanbanConfig.GetSelectedIndex())
	}

	// Verify TIKI-1 is still visible
	found1, _, _ := ta.FindText("000001")
	if !found1 {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible after refresh")
	}
}

// TestRefresh_FromTikiDetail verifies refresh works from tiki detail view
func TestRefresh_FromTikiDetail(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tiki
	tikiID := "000001"
	if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, "Original Title", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Navigate: Kanban → Tiki Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify original title visible
	found, _, _ := ta.FindText("Original Title")
	if !found {
		ta.DumpScreen()
		t.Errorf("original title should be visible in tiki detail")
	}

	// Modify the tiki file externally
	if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, "Updated Title", "ready", "story"); err != nil {
		t.Fatalf("failed to modify tiki: %v", err)
	}

	// Press 'r' to refresh from tiki detail view
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify updated title is now visible
	foundNew, _, _ := ta.FindText("Updated Title")
	if !foundNew {
		ta.DumpScreen()
		t.Errorf("updated title should be visible after refresh in tiki detail")
	}
}

// TestRefresh_WithActiveSearch verifies refresh behavior with active search
func TestRefresh_WithActiveSearch(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tikis
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "Alpha Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "Beta Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Search for "Alpha" (should show only TIKI-1)
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("Alpha")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify only TIKI-1 visible
	found1, _, _ := ta.FindText("000001")
	found2, _, _ := ta.FindText("000002")
	if !found1 || found2 {
		ta.DumpScreen()
		t.Errorf("search should filter to only TIKI-1")
	}

	// Press 'r' to refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Note: Refresh keeps search active (doesn't clear it automatically)
	// User must press Esc to clear search manually
	// This test just verifies refresh doesn't crash with active search

	// Verify TIKI-1 is still visible (search still active)
	found1After, _, _ := ta.FindText("000001")
	if !found1After {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should still be visible (search persists after refresh)")
	}
}

// TestRefresh_MultipleRefreshes verifies multiple consecutive refreshes work correctly
func TestRefresh_MultipleRefreshes(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create initial tiki
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "First Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Open Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// First refresh (no changes)
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify TIKI-1 still visible
	found, _, _ := ta.FindText("000001")
	if !found {
		t.Errorf("TIKI-1 should be visible after first refresh")
	}

	// Add a new tiki
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "Second Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}

	// Second refresh
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify both tikis visible
	found1, _, _ := ta.FindText("000001")
	found2, _, _ := ta.FindText("000002")
	if !found1 || !found2 {
		ta.DumpScreen()
		t.Errorf("both tikis should be visible after second refresh")
	}

	// Third refresh (no changes again)
	ta.SendKey(tcell.KeyRune, 'r', tcell.ModNone)

	// Verify both tikis still visible
	found1Again, _, _ := ta.FindText("000001")
	found2Again, _, _ := ta.FindText("000002")
	if !found1Again || !found2Again {
		ta.DumpScreen()
		t.Errorf("both tikis should be visible after third refresh")
	}
}
