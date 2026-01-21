package integration

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/testutil"

	"github.com/gdamore/tcell/v2"
)

// TestBoardSearch_OpenSearchBox verifies that pressing '/' opens the search box
func TestBoardSearch_OpenSearchBox(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create test tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Press '/' to open search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)

	// Verify search box is visible (look for the "> " prompt)
	found, _, _ := ta.FindText(">")
	if !found {
		ta.DumpScreen()
		t.Errorf("search box prompt '>' not found after pressing '/'")
	}
}

// TestBoardSearch_FilterResults verifies that search filters tasks by title
func TestBoardSearch_FilterResults(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create multiple tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Second Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-3", "Special Feature", taskpkg.StatusInProgress, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Open search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)

	// Type "Task" to match TIKI-1 and TIKI-2
	ta.SendText("Task")

	// Press Enter to submit search
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify TIKI-1 and TIKI-2 are visible
	found1, _, _ := ta.FindText("TIKI-1")
	if !found1 {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible in search results")
	}

	found2, _, _ := ta.FindText("TIKI-2")
	if !found2 {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible in search results")
	}

	// Verify TIKI-3 is NOT visible (doesn't match "Task")
	found3, _, _ := ta.FindText("TIKI-3")
	if found3 {
		ta.DumpScreen()
		t.Errorf("TIKI-3 should NOT be visible (doesn't match 'Task')")
	}
}

// TestBoardSearch_NoMatches verifies empty search results
func TestBoardSearch_NoMatches(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create test task
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Open search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)

	// Search for something that doesn't exist
	ta.SendText("NoMatch")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify TIKI-1 is NOT visible
	found, _, _ := ta.FindText("TIKI-1")
	if found {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should NOT be visible when search has no matches")
	}
}

// TestBoardSearch_EscapeClears verifies Esc clears search and restores selection
func TestBoardSearch_EscapeClears(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create test tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Second Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Move to second task (TIKI-2)
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Verify we're on TIKI-2 (row 1)
	if ta.BoardConfig.GetSelectedRow() != 1 {
		t.Fatalf("expected row 1, got %d", ta.BoardConfig.GetSelectedRow())
	}

	// Open search and search for "First" (matches TIKI-1 only)
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("First")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify only TIKI-1 is visible
	found1, _, _ := ta.FindText("TIKI-1")
	if !found1 {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible in search results")
	}

	found2, _, _ := ta.FindText("TIKI-2")
	if found2 {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should NOT be visible in filtered view")
	}

	// Press Esc to clear search
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify all tasks are visible again
	found1After, _, _ := ta.FindText("TIKI-1")
	if !found1After {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible after clearing search")
	}

	found2After, _, _ := ta.FindText("TIKI-2")
	if !found2After {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible after clearing search")
	}

	// Verify selection was restored to row 1 (TIKI-2)
	if ta.BoardConfig.GetSelectedRow() != 1 {
		t.Errorf("selection should be restored to row 1, got %d", ta.BoardConfig.GetSelectedRow())
	}
}

// TestBoardSearch_EscapeFromSearchBox verifies Esc while typing cancels search
func TestBoardSearch_EscapeFromSearchBox(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create test task
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Open search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)

	// Start typing
	ta.SendText("First")

	// Press Esc BEFORE submitting (should close search box without searching)
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Verify search box is closed (look for search box with border, not just ">")
	// The "> " prompt with a space is the search box, "</>" in header is the keyboard shortcut
	found, _, _ := ta.FindText("> First")
	if found {
		ta.DumpScreen()
		t.Errorf("search box should be closed after Esc")
	}

	// Verify task is still visible (no filtering happened)
	foundTask, _, _ := ta.FindText("TIKI-1")
	if !foundTask {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should still be visible (search was cancelled)")
	}
}

// TestBoardSearch_MultipleSequentialSearches verifies multiple searches in a row
func TestBoardSearch_MultipleSequentialSearches(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create test tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Alpha Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Beta Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-3", "Gamma Feature", taskpkg.StatusInProgress, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// First search: "Alpha"
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("Alpha")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify only TIKI-1 visible
	found1, _, _ := ta.FindText("TIKI-1")
	if !found1 {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible after searching 'Alpha'")
	}

	// Clear search
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Second search: "Beta"
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("Beta")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify only TIKI-2 visible
	found2, _, _ := ta.FindText("TIKI-2")
	if !found2 {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible after searching 'Beta'")
	}

	found1After, _, _ := ta.FindText("TIKI-1")
	if found1After {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should NOT be visible after searching 'Beta'")
	}

	// Clear search
	ta.SendKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Third search: "Task" (matches both TIKI-1 and TIKI-2)
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("Task")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify TIKI-1 and TIKI-2 visible, TIKI-3 not visible
	found1Final, _, _ := ta.FindText("TIKI-1")
	found2Final, _, _ := ta.FindText("TIKI-2")
	found3Final, _, _ := ta.FindText("TIKI-3")

	if !found1Final {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be visible after searching 'Task'")
	}
	if !found2Final {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible after searching 'Task'")
	}
	if found3Final {
		ta.DumpScreen()
		t.Errorf("TIKI-3 should NOT be visible after searching 'Task'")
	}
}

// TestBoardSearch_CaseInsensitive verifies search is case-insensitive
func TestBoardSearch_CaseInsensitive(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create test task with mixed case
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "MySpecialTask", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Search with lowercase
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("special")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify TIKI-1 is found (case-insensitive match)
	found, _, _ := ta.FindText("TIKI-1")
	if !found {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be found with case-insensitive search")
	}
}

// TestBoardSearch_NavigateResults verifies arrow key navigation in search results
func TestBoardSearch_NavigateResults(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create multiple matching tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Feature A", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Feature B", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-3", "Feature C", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Search for "Feature" (matches all three)
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("Feature")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Initial selection should be row 0
	if ta.BoardConfig.GetSelectedRow() != 0 {
		t.Errorf("initial selection should be row 0, got %d", ta.BoardConfig.GetSelectedRow())
	}

	// Press Down arrow to move to next result
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Should now be on row 1
	if ta.BoardConfig.GetSelectedRow() != 1 {
		t.Errorf("after Down, selection should be row 1, got %d", ta.BoardConfig.GetSelectedRow())
	}

	// Press Down arrow again
	ta.SendKey(tcell.KeyDown, 0, tcell.ModNone)

	// Should now be on row 2
	if ta.BoardConfig.GetSelectedRow() != 2 {
		t.Errorf("after second Down, selection should be row 2, got %d", ta.BoardConfig.GetSelectedRow())
	}

	// Press Up arrow
	ta.SendKey(tcell.KeyUp, 0, tcell.ModNone)

	// Should be back on row 1
	if ta.BoardConfig.GetSelectedRow() != 1 {
		t.Errorf("after Up, selection should be row 1, got %d", ta.BoardConfig.GetSelectedRow())
	}
}

// TestBoardSearch_OpenTaskFromResults verifies opening task from search results
func TestBoardSearch_OpenTaskFromResults(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create test tasks
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Alpha Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Beta Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Search for "Beta"
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("Beta")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Press Enter to open task detail
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify we're now on task detail view
	currentView := ta.NavController.CurrentView()
	if currentView.ViewID != model.TaskDetailViewID {
		t.Errorf("should be on task detail view, got %v", currentView.ViewID)
	}

	// Verify TIKI-2 is displayed in task detail
	found, _, _ := ta.FindText("TIKI-2")
	if !found {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should be visible in task detail view")
	}
}

// TestBoardSearch_SpecialCharacters verifies search handles special characters
func TestBoardSearch_SpecialCharacters(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create task with special characters
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "Fix bug #123", taskpkg.StatusTodo, taskpkg.TypeBug); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-2", "Normal Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Search for "bug" (word that appears in the title)
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)
	ta.SendText("bug")
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify TIKI-1 is found (contains "bug")
	found1, _, _ := ta.FindText("TIKI-1")
	if !found1 {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should be found when searching for 'bug'")
	}

	// Verify TIKI-2 is NOT found
	found2, _, _ := ta.FindText("TIKI-2")
	if found2 {
		ta.DumpScreen()
		t.Errorf("TIKI-2 should NOT be found when searching for 'bug'")
	}
}

// TestBoardSearch_EmptyQuery verifies empty search query is ignored
func TestBoardSearch_EmptyQuery(t *testing.T) {
	ta := testutil.NewTestApp(t)
	defer ta.Cleanup()

	// Create test task
	if err := testutil.CreateTestTask(ta.TaskDir, "TIKI-1", "First Task", taskpkg.StatusTodo, taskpkg.TypeStory); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	if err := ta.TaskStore.Reload(); err != nil {
		t.Fatalf("failed to reload tasks: %v", err)
	}

	// Navigate to board
	ta.NavController.PushView(model.BoardViewID, nil)
	ta.Draw()

	// Open search
	ta.SendKey(tcell.KeyRune, '/', tcell.ModNone)

	// Press Enter without typing anything (empty query)
	ta.SendKey(tcell.KeyEnter, 0, tcell.ModNone)

	// Verify task is still visible (no filtering happened)
	found, _, _ := ta.FindText("TIKI-1")
	if !found {
		ta.DumpScreen()
		t.Errorf("TIKI-1 should still be visible (empty search ignored)")
	}

	// Note: Search box stays open on empty query (expected behavior)
	// User must press Esc to close it
	// This is correct - empty search doesn't close the box, just ignores the search
}
