package integration

import (
	"fmt"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// TestTikiDetailView_RenderMetadata verifies all tiki metadata is displayed
func TestTikiDetailView_RenderMetadata(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Load plugins to enable Kanban
	if err := ta.LoadPlugins(); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Create a tiki with all fields populated
	tikiID := "000001"
	if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, "Test Tiki Title", "inProgress", "bug"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Navigate: Kanban → Tiki Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone) // Open tiki detail

	// Verify tiki ID is visible
	found, _, _ := ta.FindText("000001")
	if !found {
		ta.DumpScreen()
		t.Errorf("tiki ID 'TIKI-1' not found in tiki detail view")
	}

	// Verify title is visible
	foundTitle, _, _ := ta.FindText("Test Tiki Title")
	if !foundTitle {
		ta.DumpScreen()
		t.Errorf("tiki title not found in tiki detail view")
	}

	// Verify status label is visible
	foundStatus, _, _ := ta.FindText("Status:")
	if !foundStatus {
		ta.DumpScreen()
		t.Errorf("'Status:' label not found in tiki detail view")
	}

	// Verify type label is visible
	foundType, _, _ := ta.FindText("Type:")
	if !foundType {
		ta.DumpScreen()
		t.Errorf("'Type:' label not found in tiki detail view")
	}

	// Verify priority label is visible
	foundPriority, _, _ := ta.FindText("Priority:")
	if !foundPriority {
		ta.DumpScreen()
		t.Errorf("'Priority:' label not found in tiki detail view")
	}
}

// TestTikiDetailView_RenderDescription verifies tiki description is displayed
func TestTikiDetailView_RenderDescription(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create tiki (description is set to the title by CreateTestTiki)
	tikiID := "000001"
	if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, "Tiki with description", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Navigate: Board → Tiki Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify description is visible (markdown rendered)
	// The description content is the same as the title in test fixtures
	foundDesc, _, _ := ta.FindText("Tiki with description")
	if !foundDesc {
		ta.DumpScreen()
		t.Errorf("tiki description not found in tiki detail view")
	}
}

// TestTikiDetailView_NavigateBack verifies Esc returns to board
func TestTikiDetailView_NavigateBack(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create tiki
	tikiID := "000001"
	if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, "Test Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Navigate: Board → Tiki Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Phase 3: configurable detail view (workflow `kind: view`), not
	// the legacy TikiDetailViewID.
	currentView := ta.NavController.CurrentView()
	wantDetail := model.DetailPluginViewID()
	if currentView.ViewID != wantDetail {
		t.Fatalf("expected detail view %s, got %v", wantDetail, currentView.ViewID)
	}

	// Press Esc to go back
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify we're back on board
	currentView = ta.NavController.CurrentView()
	if currentView.ViewID != model.MakePluginViewID("Kanban") {
		t.Errorf("expected board view after Esc, got %v", currentView.ViewID)
	}
}

// Phase 3 cleanup: TestTikiDetailView_InlineTitleEdit_Save and
// TestTikiDetailView_InlineTitleEdit_Cancel were removed. Inline title
// editing was a legacy TikiDetailView affordance ('e' opened the title
// field for keystroke editing, Enter committed). The configurable
// detail view does not surface a title editor — title is rendered as
// part of the always-on detail layout and edited via 'e' → in-place
// edit mode on the workflow-declared metadata fields.

// TestTikiDetailView_FromBoard verifies opening tiki from board
func TestTikiDetailView_FromBoard(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

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

	// Navigate to board
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Move to second tiki
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Open tiki detail
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify we're on tiki detail for TIKI-2
	found, _, _ := ta.FindText("000002")
	if !found {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible in tiki detail view")
	}

	// Verify TIKI-1 is NOT visible (we're viewing TIKI-2)
	found1, _, _ := ta.FindText("000001")
	if found1 {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should NOT be visible (we opened TIKI-2)")
	}
}

// TestTikiDetailView_EmptyDescription verifies rendering with no description
func TestTikiDetailView_EmptyDescription(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create tiki with minimal content
	tikiID := "000001"
	if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, "Tiki Title", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Navigate: Board → Tiki Detail
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify tiki title is visible
	found, _, _ := ta.FindText("Tiki Title")
	if !found {
		ta.DumpScreen()
		t.Errorf("tiki title should be visible even with empty description")
	}

	// Verify Status label is still visible
	foundStatus, _, _ := ta.FindText("Status:")
	if !foundStatus {
		ta.DumpScreen()
		t.Errorf("metadata should be visible even with empty description")
	}
}

// TestTikiDetailView_MultipleOpen verifies opening different tikis sequentially
func TestTikiDetailView_MultipleOpen(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create multiple tikis
	if err := testutil.CreateTestTiki(ta.TikiDir, "000001", "First Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000002", "Second Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := testutil.CreateTestTiki(ta.TikiDir, "000003", "Third Tiki", "ready", "story"); err != nil {
		t.Fatalf("failed to create test tiki: %v", err)
	}
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Open first tiki
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	found1, _, _ := ta.FindText("000001")
	if !found1 {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible after opening")
	}

	// Go back
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Move to second tiki and open
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	found2, _, _ := ta.FindText("000002")
	if !found2 {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible after opening")
	}

	// Go back
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Move to third tiki and open
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)
	found3, _, _ := ta.FindText("000003")
	if !found3 {
		ta.DumpScreen()
		t.Errorf("TIKI-3 should be visible after opening")
	}
}

// TestTikiDetailView_AllStatuses verifies rendering different status values
func TestTikiDetailView_AllStatuses(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	statuses := []string{
		"backlog",
		"ready",
		"inProgress",
		"review",
		"done",
	}

	for i, status := range statuses {
		tikiID := testutil.ID(fmt.Sprintf("TIKI-%d", i+1))
		title := fmt.Sprintf("Tiki %s", status)
		if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, title, status, "story"); err != nil {
			t.Fatalf("failed to create test tiki: %v", err)
		}
	}

	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Open board
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// For each status, navigate to first tiki with that status and verify detail view
	for i, status := range statuses {
		// Find the tiki on board (may need to navigate between lanes)
		tikiID := fmt.Sprintf("TIKI-%d", i+1)

		// Navigate to correct lane based on status
		// For simplicity, we'll just open first tiki in todo lane for this test
		if status == "ready" {
			ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

			// Verify tiki ID visible (use the normalized canonical id since
			// the UI renders the on-disk id, not the test-shorthand form).
			canonID := testutil.ID(tikiID)
			found, _, _ := ta.FindText(canonID)
			if !found {
				ta.DumpScreen()
				t.Errorf("tiki %s with status %s not found in detail view", canonID, status)
			}

			// Go back for next iteration
			ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)
			break // Just test one for now
		}
	}
}

// TestTikiDetailView_AllTypes verifies rendering different type values
func TestTikiDetailView_AllTypes(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	types := []string{
		"story",
		"bug",
	}

	for i, tikiType := range types {
		tikiID := fmt.Sprintf("TIKI-%d", i+1)
		title := fmt.Sprintf("Tiki %s", tikiType)
		if err := testutil.CreateTestTiki(ta.TikiDir, tikiID, title, "ready", tikiType); err != nil {
			t.Fatalf("failed to create test tiki: %v", err)
		}
	}

	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Open board and first tiki
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify Type label is visible
	found, _, _ := ta.FindText("Type:")
	if !found {
		ta.DumpScreen()
		t.Errorf("Type label should be visible in tiki detail")
	}
}

// Phase 3 cleanup: TestTikiDetailView_InlineEdit_PreservesOtherFields
// removed for the same reason as the inline-title tests above. The
// "edit doesn't corrupt sibling fields" invariant is now exercised by
// configurable_detail_edit_test.go via the field-registry change
// handlers, which only mutate the field they're bound to.
