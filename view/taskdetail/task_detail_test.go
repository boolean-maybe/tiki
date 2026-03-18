package taskdetail

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
)

// TestBuildMetadataColumns_Structure verifies that buildMetadataColumns returns 3 flex containers
func TestBuildMetadataColumns_Structure(t *testing.T) {
	// Setup
	s := store.NewInMemoryStore()
	task := &task.Task{
		ID:          "TIKI-1",
		Title:       "Test Task",
		Description: "Test description",
		Status:      task.StatusReady,
		Type:        task.TypeStory,
		Priority:    3,
		Points:      5,
		Assignee:    "user@example.com",
		CreatedBy:   "creator@example.com",
		CreatedAt:   time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2024, 1, 2, 14, 30, 0, 0, time.UTC),
	}

	view := NewTaskDetailView(s, task.ID, nil, nil)
	view.SetFallbackTask(task)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	// Execute
	col1, col2, col3 := view.buildMetadataColumns(task, ctx, colors)

	// Verify all three columns are returned and non-nil
	if col1 == nil {
		t.Error("Expected col1 to be non-nil")
	}
	if col2 == nil {
		t.Error("Expected col2 to be non-nil")
	}
	if col3 == nil {
		t.Error("Expected col3 to be non-nil")
	}
}

// TestBuildMetadataColumns_Column1Fields verifies column 1 contains Status, Type, Priority, Points
func TestBuildMetadataColumns_Column1Fields(t *testing.T) {
	// Setup
	s := store.NewInMemoryStore()
	task := &task.Task{
		ID:       "TIKI-1",
		Title:    "Test Task",
		Status:   task.StatusReady,
		Type:     task.TypeStory,
		Priority: 3,
		Points:   5,
	}

	view := NewTaskDetailView(s, task.ID, nil, nil)
	view.SetFallbackTask(task)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	// Execute
	col1, _, _ := view.buildMetadataColumns(task, ctx, colors)

	// Verify column 1 has 4 items (Status, Type, Priority, Points)
	if col1.GetItemCount() != 4 {
		t.Errorf("Expected col1 to have 4 items, got %d", col1.GetItemCount())
	}
}

// TestBuildMetadataColumns_Column2Fields verifies column 2 contains Assignee, Author, Created, Updated
func TestBuildMetadataColumns_Column2Fields(t *testing.T) {
	// Setup
	s := store.NewInMemoryStore()
	task := &task.Task{
		ID:        "TIKI-1",
		Title:     "Test Task",
		Assignee:  "user@example.com",
		CreatedBy: "creator@example.com",
		CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2024, 1, 2, 14, 30, 0, 0, time.UTC),
	}

	view := NewTaskDetailView(s, task.ID, nil, nil)
	view.SetFallbackTask(task)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	// Execute
	_, col2, _ := view.buildMetadataColumns(task, ctx, colors)

	// Verify column 2 has 4 items (Assignee, Author, Created, Updated)
	if col2.GetItemCount() != 4 {
		t.Errorf("Expected col2 to have 4 items, got %d", col2.GetItemCount())
	}
}

// TestBuildMetadataColumns_Column3Fields verifies column 3 contains Due, Recurrence
func TestBuildMetadataColumns_Column3Fields(t *testing.T) {
	// Setup
	s := store.NewInMemoryStore()
	task := &task.Task{
		ID:    "TIKI-1",
		Title: "Test Task",
	}

	view := NewTaskDetailView(s, task.ID, nil, nil)
	view.SetFallbackTask(task)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	// Execute
	_, _, col3 := view.buildMetadataColumns(task, ctx, colors)

	// Verify column 3 has 2 items (Due, Recurrence)
	if col3.GetItemCount() != 2 {
		t.Errorf("Expected col3 to have 2 items, got %d", col3.GetItemCount())
	}
}
