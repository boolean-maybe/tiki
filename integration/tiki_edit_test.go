package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/testutil"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/gdamore/tcell/v2"
)

// findTikiByTitle finds a tiki by its title in a slice of tikis
func findTikiByTitle(tikis []*tikipkg.Tiki, title string) *tikipkg.Tiki {
	for _, tk := range tikis {
		if tk.Title == title {
			return tk
		}
	}
	return nil
}

// =============================================================================
// NEW TASK CREATION (Draft Mode) Tests
// =============================================================================

func TestNewTiki_Enter_SavesAndCreatesFile(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new tiki (opens edit view with title focused)
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Type title
	ta.SendText("My New Tiki")

	// Press Enter to save
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify: file should be created
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Find the new tiki by title (IDs are now random)
	tiki := findTikiByTitle(ta.TikiStore.GetAllTikis(), "My New Tiki")
	if tiki == nil {
		t.Fatalf("new tiki not found in store")
		return
	}
	if tiki.Title != "My New Tiki" {
		t.Errorf("title = %q, want %q", tiki.Title, "My New Tiki")
	}

	// Verify file exists on disk (filename uses lowercase ID)
	tikiPath := filepath.Join(ta.TikiDir, strings.ToLower(tiki.ID)+".md")
	if _, err := os.Stat(tikiPath); os.IsNotExist(err) {
		t.Errorf("tiki file was not created at %s", tikiPath)
	}
}

func TestNewTiki_Escape_DiscardsWithoutCreatingFile(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new tiki
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Type title
	ta.SendText("Tiki To Discard")

	// Press Escape to cancel
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify: no file should be created
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Should have no tikis (find by title since IDs are random)
	tiki := findTikiByTitle(ta.TikiStore.GetAllTikis(), "Tiki To Discard")
	if tiki != nil {
		t.Errorf("tiki should not exist after escape, but found: %+v", tiki)
	}

	// Verify no tiki files on disk
	files, _ := filepath.Glob(filepath.Join(ta.TikiDir, "tiki-*.md"))
	if len(files) > 0 {
		t.Errorf("tiki files should not exist, but found: %v", files)
	}
}

func TestNewTiki_CtrlS_SavesAndCreatesFile(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new tiki
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Type title
	ta.SendText("Tiki Saved With CtrlS")

	// Tab to another field (Points): Title → Status → Type → Priority → Points (4 tabs)
	for i := 0; i < 4; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Press Ctrl+S to save
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Verify: file should be created
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	tiki := findTikiByTitle(ta.TikiStore.GetAllTikis(), "Tiki Saved With CtrlS")
	if tiki == nil {
		t.Fatalf("new tiki not found in store")
		return
	}
	if tiki.Title != "Tiki Saved With CtrlS" {
		t.Errorf("title = %q, want %q", tiki.Title, "Tiki Saved With CtrlS")
	}
}

func TestEditSource_DuplicateCaseIDs_Repro(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a tiki with lowercase suffix ID directly in the store.
	tikiID := "6EQDUE"
	tk := tikipkg.New()
	tk.ID = tikiID
	tk.Title = "Edit Source Duplicate"
	tk.Set("type", "story")
	tk.Set("status", "backlog")
	tk.Set("priority", "medium")
	tk.Set("points", "1")
	if err := ta.TikiStore.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki failed: %v", err)
	}
	if tk.ID != "6EQDUE" {
		t.Fatalf("expected tiki ID to be normalized, got %q", tk.ID)
	}

	// Mock editor to modify the tiki file and return immediately.
	ta.NavController.SetEditorOpener(func(path string) error {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content = append(content, '\n')
		return os.WriteFile(path, content, 0644) //nolint:gosec // G703: path is a controlled temp file provided by the app
	})

	// Phase 3: open the configurable detail view directly. The Edit
	// source ('s') keybinding now lives on the configurable detail
	// view's registry; the legacy TikiDetailViewID is no longer the
	// host for the edit-source flow.
	ta.NavController.PushView(
		model.DetailPluginViewID(),
		model.EncodePluginViewParams(model.PluginViewParams{TikiID: tikiID}),
	)
	ta.Draw()

	// Trigger "Edit source" (key 's') which reloads the tiki after editor returns.
	ta.SendKey(tcell.KeyRune, 's', tcell.ModNone)

	// Expect a single tiki in store (no case-duplicate).
	tikis := ta.TikiStore.GetAllTikis()
	if len(tikis) != 1 {
		t.Fatalf("expected 1 tiki after edit source, got %d", len(tikis))
	}

	foundUpper := false
	for _, tsk := range tikis {
		switch tsk.ID {
		case "6EQDUE":
			foundUpper = true
		}
	}
	if !foundUpper {
		t.Fatalf("expected uppercase ID variant, foundUpper=%v", foundUpper)
	}

	// Ensure file path is the lowercased ID (edit source uses this file).
	tikiFilePath := filepath.Join(ta.TikiDir, strings.ToLower(tikiID)+".md")
	if _, err := os.Stat(tikiFilePath); os.IsNotExist(err) {
		t.Fatalf("expected tiki file to exist at %s", tikiFilePath)
	}
}

func TestNewTiki_EmptyTitle_DoesNotSave(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new tiki
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Don't type anything - leave title empty
	// Press Enter to try to save
	ta.SendKeyToFocused(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify: no file should be created (empty title validation)
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	// Should have no tikis
	tikis := ta.TikiStore.GetAllTikis()
	if len(tikis) > 0 {
		t.Errorf("tiki with empty title should not be saved, but found: %+v", tikis)
	}

	// Verify no tiki files on disk
	files, _ := filepath.Glob(filepath.Join(ta.TikiDir, "tiki-*.md"))
	if len(files) > 0 {
		t.Errorf("tiki files should not exist, but found: %v", files)
	}
}

// =============================================================================
// EXISTING TASK EDITING Tests
// =============================================================================
//
// Phase 3 cleanup: TestTikiEdit_EnterInPointsFieldDoesNotSave and
// TestTikiEdit_TitleChangesSaved were removed. Both opened the legacy
// tiki edit view via Enter → 'e' and asserted "Enter saves title" /
// "Enter in Points field is a no-op". After Phase 2 the configurable
// detail view's in-place edit mode owns those keystrokes; coverage
// lives in view/tikidetail/configurable_detail_edit_test.go.

// =============================================================================
// PHASE 2: EXISTING TASK SAVE/CANCEL Tests
// =============================================================================
//
// Previously held save/cancel tests (Ctrl+S from points,
// Escape from title/points, edit-session state clearing) that
// asserted on a deleted rigid 6-field edit view reached via
// Enter → 'e'. Edit mode is now in-place on the configurable detail
// view; equivalent save/cancel coverage lives in
// view/tikidetail/configurable_detail_edit_test.go.

// =============================================================================
// PHASE 3: FIELD NAVIGATION Tests
// =============================================================================
//
// Previously held Tab-traversal tests that asserted on a deleted
// rigid 6-field edit view (Title, Status, Type, Priority, Points,
// Assignee). The configurable detail view's edit mode traverses
// only the workflow-declared metadata (default
// [status, type, priority]); coverage of the new traversal
// lives in view/tikidetail/configurable_detail_edit_test.go.

// =============================================================================
// PHASE 4: MULTI-FIELD OPERATIONS Tests
// =============================================================================
//
// Previously held multi-field save/discard tests that asserted on a
// deleted rigid 5-field edit view (title/priority/points). The
// configurable detail view's edit mode operates on the workflow's
// declared metadata only (default [status, type, priority]); see
// view/tikidetail/configurable_detail_edit_test.go for save/discard
// coverage of the new path.

func TestNewTiki_MultipleFields_AllSaved(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new tiki
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Type title
	ta.SendText("New Tiki With Multiple Fields")

	// Tab to Priority field (7 tabs: status → assignee → due → tags → type → recurrence → priority)
	for i := 0; i < 7; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	// set priority to medium-low (declaration is high→low, so Down moves
	// to next/less-urgent value)
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Tab to Points field (Priority → Points), then Up to cycle from
	// the default "3" toward the previous (higher) value "7".
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	ta.SendKey(tcell.KeyUp, 0, tcell.ModNone)

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Verify: file should be created with all fields
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}
	tk := findTikiByTitle(ta.TikiStore.GetAllTikis(), "New Tiki With Multiple Fields")
	if tk == nil {
		t.Fatalf("new tiki not found in store")
		return
	}
	if tk.Title != "New Tiki With Multiple Fields" {
		t.Errorf("title = %q, want %q", tk.Title, "New Tiki With Multiple Fields")
	}
	priority, _, _ := tk.StringField("priority")
	if priority != "medium-low" {
		t.Errorf("priority = %q, want %q", priority, "medium-low")
	}
	points, _, _ := tk.StringField("points")
	if points != "7" {
		t.Errorf("points = %q, want %q", points, "7")
	}
}

// =============================================================================
// REGRESSION TESTS
// =============================================================================

// TestNewTiki_AfterEditingExistingTiki_StatusAndTypeNotCorrupted
// Phase 3 cleanup: the original test mixed the legacy "Enter → 'e' →
// Ctrl+S" edit-existing flow (now in-place on the configurable detail
// view) with the surviving 'n' new-tiki draft flow. The regression it
// guarded against — TikiEditSession state from a prior edit session
// leaking into the next draft — is now covered at the unit level by
// the TikiEditSession edit-session tests in controller/tiki_edit_session_test.go,
// which directly exercise the StartEditSession/CommitEditSession/
// ClearDraft state machine without relying on TUI keystroke routing.

func TestNewTiki_WithStatusAndType_Saves(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Start on board view
	ta.NavController.PushView(model.MakePluginViewID("Kanban"), nil)
	ta.Draw()

	// Press 'n' to create new tiki
	ta.SendKey(tcell.KeyRune, 'n', tcell.ModNone)

	// Set title
	ta.SendText("Hey")

	// Tab to Status field (1 tab)
	ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)

	// Cycle status to Review (press down arrow several times)
	// Status order: Backlog -> Ready -> In Progress -> Review -> Done
	for i := 0; i < 3; i++ {
		ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)
	}

	// Tab to Type field (4 tabs: status → assignee → due → tags → type)
	for i := 0; i < 4; i++ {
		ta.SendKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// Cycle type to Bug (press down arrow once)
	// Type order: Story -> Bug
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Save with Ctrl+S
	ta.SendKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	// Verify: file should be created
	if err := ta.TikiStore.Reload(); err != nil {
		t.Fatalf("failed to reload tikis: %v", err)
	}

	tk := findTikiByTitle(ta.TikiStore.GetAllTikis(), "Hey")
	if tk == nil {
		t.Fatalf("new tiki not found in store")
		return
	}

	heyStatus, _, _ := tk.StringField("status")
	heyType, _, _ := tk.StringField("type")
	t.Logf("Tiki found: Title=%q, Status=%v, Type=%v", tk.Title, heyStatus, heyType)

	if tk.Title != "Hey" {
		t.Errorf("title = %q, want %q", tk.Title, "Hey")
	}
	if heyStatus != "review" {
		t.Errorf("status = %v, want %v", heyStatus, "review")
	}
	if heyType != "bug" {
		t.Errorf("type = %v, want %v", heyType, "bug")
	}
}
