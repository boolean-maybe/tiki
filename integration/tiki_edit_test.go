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
		if tk.Title() == title {
			return tk
		}
	}
	return nil
}

// =============================================================================
// NEW TASK CREATION (Draft Mode) Tests
// =============================================================================

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

func TestEditSource_DuplicateCaseIDs_Repro(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create a tiki with lowercase suffix ID directly in the store.
	tikiID := "6EQDUE"
	tk := tikipkg.New()
	tk.SetID(tikiID)
	tk.SetTitle("Edit Source Duplicate")
	tk.Set("type", "story")
	tk.Set("status", "inbox")
	tk.Set("priority", "medium")
	tk.Set("points", "1")
	if err := ta.TikiStore.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki failed: %v", err)
	}
	if tk.ID() != "6EQDUE" {
		t.Fatalf("expected tiki ID to be normalized, got %q", tk.ID())
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
		switch tsk.ID() {
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
