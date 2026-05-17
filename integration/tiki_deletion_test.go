package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// TestTikiDeletion_FromKanban verifies 'd' deletes tiki from kanban
func TestTikiDeletion_FromKanban(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tikis
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "First Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "Second Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Verify TIKI-1 visible
	found, _, _ := ta.FindText("000001")
	if !found {
		ta.DumpScreen()
		t.Fatalf("TIKI-1 should be visible before delete")
	}

	// Press 'd' to delete first tiki
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify tiki deleted
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	tiki := ta.TikiStore.GetTiki("000001")
	if tiki != nil {
		t.Errorf("TIKI-1 should be deleted from store")
	}

	// Verify file removed
	tikiPath := filepath.Join(ta.TikiDir, "000001.md")
	if _, err := os.Stat(tikiPath); !os.IsNotExist(err) {
		t.Errorf("TIKI-1 file should be deleted")
	}

	// Verify TIKI-2 still visible
	found2, _, _ := ta.FindText("000002")
	if !found2 {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should still be visible after deleting TIKI-1")
	}
}

// TestTikiDeletion_SelectionMoves verifies selection moves to next tiki after delete
func TestTikiDeletion_SelectionMoves(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create three tikis
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "First Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "Second Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000003", "Third Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Move to second tiki (index 1)
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	kanbanConfig := ta.GetPluginConfig("Kanban")
	// Verify we're on index 1
	if kanbanConfig.GetSelectedIndex() != 1 {
		t.Fatalf("expected index 1, got %d", kanbanConfig.GetSelectedIndex())
	}

	// Delete TIKI-2
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Selection should move to next tiki (TIKI-3, which is now at index 1)
	selectedIndex := kanbanConfig.GetSelectedIndex()
	if selectedIndex != 1 {
		t.Errorf("selection after delete = index %d, want index 1", selectedIndex)
	}

	// Verify TIKI-3 is visible
	found3, _, _ := ta.FindText("000003")
	if !found3 {
		ta.DumpScreen()
		t.Errorf("TIKI-3 should be visible after deleting TIKI-2")
	}
}

// TestTikiDeletion_LastTikiInLane verifies deleting last tiki resets selection
func TestTikiDeletion_LastTikiInLane(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create only one tiki in todo lane
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "Only Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Verify TIKI-1 visible
	found, _, _ := ta.FindText("000001")
	if !found {
		ta.DumpScreen()
		t.Fatalf("TIKI-1 should be visible")
	}

	// Delete the only tiki
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Verify tiki deleted
	tiki := ta.TikiStore.GetTiki("000001")
	if tiki != nil {
		t.Errorf("TIKI-1 should be deleted")
	}

	kanbanConfig := ta.GetPluginConfig("Kanban")
	// Verify selection reset to 0
	if kanbanConfig.GetSelectedIndex() != 0 {
		t.Errorf("selection should reset to 0 after deleting last tiki, got %d", kanbanConfig.GetSelectedIndex())
	}

	// Verify no crash occurred (lane is empty)
	// This is implicit - if we got here without panic, test passes
}

// TestTikiDeletion_MultipleSequential verifies deleting multiple tikis in sequence
func TestTikiDeletion_MultipleSequential(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create five tikis
	tikiIDs := []string{"SEQ001", "SEQ002", "SEQ003", "SEQ004", "SEQ005"}
	for i, tikiID := range tikiIDs {
		title := fmt.Sprintf("Tiki %d", i+1)
		if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, title, "ready", "story"); err != nil {
			t.Fatalf("failed to create tiki: %v", err)
		}
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Delete first tiki
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Delete first tiki again (was TIKI-2, now at top)
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Delete first tiki again (was TIKI-3, now at top)
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify only 2 tikis remain
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	allTikis := ta.TikiStore.GetAllTikis()
	if len(allTikis) != 2 {
		t.Errorf("expected 2 tikis remaining, got %d", len(allTikis))
	}

	// Verify SEQ004 and SEQ005 still exist
	tiki4 := ta.TikiStore.GetTiki("SEQ004")
	tiki5 := ta.TikiStore.GetTiki("SEQ005")
	if tiki4 == nil || tiki5 == nil {
		t.Errorf("SEQ004 and SEQ005 should still exist")
	}
}

// TestTikiDeletion_FromDifferentLane verifies deleting from non-todo lane
func TestTikiDeletion_FromDifferentLane(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tiki in in_progress lane
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "In Progress Tiki", "inProgress", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Move to in_progress lane (Right arrow)
	ta.SendKey(tcell.KeyRight, 0, tcell.ModNone)

	// Verify TIKI-1 visible
	found, _, _ := ta.FindText("000001")
	if !found {
		ta.DumpScreen()
		t.Fatalf("TIKI-1 should be visible in in_progress lane")
	}

	// Delete tiki
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify deleted
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	tiki := ta.TikiStore.GetTiki("000001")
	if tiki != nil {
		t.Errorf("TIKI-1 should be deleted")
	}
}

// TestTikiDeletion_CannotDeleteFromTikiDetail verifies 'd' doesn't work in tiki detail
func TestTikiDeletion_CannotDeleteFromTikiDetail(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tiki
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "Tiki to Not Delete", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate: Kanban → Tiki Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Phase 3: configurable detail view, not the legacy TikiDetailViewID.
	currentView := ta.NavController.CurrentView()
	wantDetail := model.DetailPluginViewID()
	if currentView.ViewID != wantDetail {
		t.Fatalf("expected detail view %s, got %v", wantDetail, currentView.ViewID)
	}

	// Press 'd' (should not delete from tiki detail view)
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload and verify tiki still exists
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	tiki := ta.TikiStore.GetTiki("000001")
	if tiki == nil {
		t.Errorf("TIKI-1 should NOT be deleted from tiki detail view")
	}

	// Verify we're still on tiki detail (or moved somewhere else, but tiki exists)
	if tiki == nil {
		t.Errorf("tiki should still exist after pressing 'd' in tiki detail")
	}
}

// TestTikiDeletion_WithMultipleLanes verifies deletion doesn't affect other lanes
func TestTikiDeletion_WithMultipleLanes(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create tikis in different lanes
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "Todo Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "In Progress Tiki", "inProgress", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000003", "Done Tiki", "done", "story"); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Navigate to Kanban plugin
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Delete TIKI-1 from todo lane
	ta.SendKey(tcell.KeyRune, 'd', tcell.ModNone)

	// Reload
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Verify TIKI-1 deleted
	if ta.TikiStore.GetTiki("000001") != nil {
		t.Errorf("TIKI-1 should be deleted")
	}

	// Verify TIKI-2 and TIKI-3 still exist (in other lanes)
	if ta.TikiStore.GetTiki("000002") == nil {
		t.Errorf("TIKI-2 (in different lane) should still exist")
	}
	if ta.TikiStore.GetTiki("000003") == nil {
		t.Errorf("TIKI-3 (in different lane) should still exist")
	}
}
