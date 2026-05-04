package taskdetail

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// newTestViewTiki creates a *tikipkg.Tiki test fixture for view-layer tests.
func newTestViewTiki(id string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = id
	tk.Title = "Test Task"
	tk.Set(tikipkg.FieldStatus, string(task.StatusReady))
	tk.Set(tikipkg.FieldType, string(task.TypeStory))
	tk.Set(tikipkg.FieldPriority, 3)
	tk.Set(tikipkg.FieldPoints, 5)
	tk.Set(tikipkg.FieldAssignee, "user@example.com")
	tk.Set("createdBy", "creator@example.com")
	tk.CreatedAt = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tk.UpdatedAt = time.Date(2024, 1, 2, 14, 30, 0, 0, time.UTC)
	return tk
}

// TestBuildMetadataColumns_Structure verifies that buildMetadataColumns returns 3 flex containers
func TestBuildMetadataColumns_Structure(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI01")

	view := NewTaskDetailView(s, tk.ID, false, nil, nil)
	view.SetFallbackTiki(tk)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	col1, col2, col3 := view.buildMetadataColumns(tk, ctx, colors)

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
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI01")

	view := NewTaskDetailView(s, tk.ID, false, nil, nil)
	view.SetFallbackTiki(tk)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	col1, _, _ := view.buildMetadataColumns(tk, ctx, colors)

	// Verify column 1 has 4 items (Status, Type, Priority, Points)
	if col1.GetItemCount() != 4 {
		t.Errorf("Expected col1 to have 4 items, got %d", col1.GetItemCount())
	}
}

// TestBuildMetadataColumns_Column2Fields verifies column 2 contains Assignee, Author, Created, Updated
func TestBuildMetadataColumns_Column2Fields(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI01")

	view := NewTaskDetailView(s, tk.ID, false, nil, nil)
	view.SetFallbackTiki(tk)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	_, col2, _ := view.buildMetadataColumns(tk, ctx, colors)

	// Verify column 2 has 4 items (Assignee, Author, Created, Updated)
	if col2.GetItemCount() != 4 {
		t.Errorf("Expected col2 to have 4 items, got %d", col2.GetItemCount())
	}
}

// TestBuildMetadataColumns_Column3Fields verifies column 3 contains Due, Recurrence
func TestBuildMetadataColumns_Column3Fields(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := tikipkg.New()
	tk.ID = "TIKI01"
	tk.Title = "Test Task"

	view := NewTaskDetailView(s, tk.ID, false, nil, nil)
	view.SetFallbackTiki(tk)

	colors := config.GetColors()
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: colors}

	_, _, col3 := view.buildMetadataColumns(tk, ctx, colors)

	// Verify column 3 has 2 items (Due, Recurrence)
	if col3.GetItemCount() != 2 {
		t.Errorf("Expected col3 to have 2 items, got %d", col3.GetItemCount())
	}
}
