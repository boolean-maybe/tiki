package taskdetail

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/view/renderer"
)

// TestBuildMetadataColumns_Structure verifies that buildMetadataColumns returns 2 flex containers
// and RenderMetadataColumn3 returns the third column
func TestBuildMetadataColumns_Structure(t *testing.T) {
	// Setup
	s := store.NewInMemoryStore()
	renderer, err := renderer.NewGlamourRenderer()
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}
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

	view := NewTaskDetailView(s, task.ID, renderer)
	view.SetFallbackTask(task)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	// Execute
	col1, col2 := view.buildMetadataColumns(task, ctx)
	col3 := RenderMetadataColumn3(task, colors)

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

// TestBuildMetadataColumns_Column1Fields verifies column 1 contains Status, Type, Priority
func TestBuildMetadataColumns_Column1Fields(t *testing.T) {
	// Setup
	s := store.NewInMemoryStore()
	renderer, err := renderer.NewGlamourRenderer()
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}
	task := &task.Task{
		ID:       "TIKI-1",
		Title:    "Test Task",
		Status:   task.StatusReady,
		Type:     task.TypeStory,
		Priority: 3,
	}

	view := NewTaskDetailView(s, task.ID, renderer)
	view.SetFallbackTask(task)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	// Execute
	col1, _ := view.buildMetadataColumns(task, ctx)

	// Verify column 1 has 3 items (Status, Type, Priority)
	if col1.GetItemCount() != 3 {
		t.Errorf("Expected col1 to have 3 items, got %d", col1.GetItemCount())
	}
}

// TestBuildMetadataColumns_Column2Fields verifies column 2 contains Assignee, Points, and spacer
func TestBuildMetadataColumns_Column2Fields(t *testing.T) {
	// Setup
	s := store.NewInMemoryStore()
	renderer, err := renderer.NewGlamourRenderer()
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}
	task := &task.Task{
		ID:       "TIKI-1",
		Title:    "Test Task",
		Assignee: "user@example.com",
		Points:   5,
	}

	view := NewTaskDetailView(s, task.ID, renderer)
	view.SetFallbackTask(task)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	// Execute
	_, col2 := view.buildMetadataColumns(task, ctx)

	// Verify column 2 has 3 items (Assignee, Points, Spacer)
	if col2.GetItemCount() != 3 {
		t.Errorf("Expected col2 to have 3 items, got %d", col2.GetItemCount())
	}
}

// TestBuildMetadataColumns_Column3Fields verifies column 3 contains Author, Created, Updated
func TestBuildMetadataColumns_Column3Fields(t *testing.T) {
	// Setup
	task := &task.Task{
		ID:        "TIKI-1",
		Title:     "Test Task",
		CreatedBy: "creator@example.com",
		CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2024, 1, 2, 14, 30, 0, 0, time.UTC),
	}

	colors := config.GetColors()

	// Execute
	col3 := RenderMetadataColumn3(task, colors)

	// Verify column 3 has 3 items (Author, Created, Updated)
	if col3.GetItemCount() != 3 {
		t.Errorf("Expected col3 to have 3 items, got %d", col3.GetItemCount())
	}
}
