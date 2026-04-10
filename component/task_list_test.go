package component

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/task"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func makeTasks(ids ...string) []*task.Task {
	tasks := make([]*task.Task, len(ids))
	for i, id := range ids {
		tasks[i] = &task.Task{
			ID:     id,
			Title:  "Task " + id,
			Status: task.StatusBacklog,
		}
	}
	return tasks
}

func TestNewTaskList(t *testing.T) {
	tl := NewTaskList(5)

	if tl == nil {
		t.Fatal("NewTaskList returned nil")
		return
	}
	if tl.maxVisibleRows != 5 {
		t.Errorf("Expected maxVisibleRows=5, got %d", tl.maxVisibleRows)
	}
	if tl.selectionIndex != 0 {
		t.Errorf("Expected initial selectionIndex=0, got %d", tl.selectionIndex)
	}

	colors := config.GetColors()
	if tl.idGradient != colors.TaskBoxIDColor {
		t.Error("Expected ID gradient from config")
	}
	if tl.idFallback != config.GetColors().FallbackTaskIDColor {
		t.Error("Expected ID fallback from config")
	}
	if tl.titleColor != colors.TaskBoxTitleColor {
		t.Error("Expected title color from config")
	}
}

func TestSetTasks_RecomputesIDColumnWidth(t *testing.T) {
	tl := NewTaskList(10)

	tl.SetTasks(makeTasks("AB", "ABCDE", "XY"))
	if tl.idColumnWidth != 5 {
		t.Errorf("Expected idColumnWidth=5, got %d", tl.idColumnWidth)
	}

	tl.SetTasks(makeTasks("A"))
	if tl.idColumnWidth != 1 {
		t.Errorf("Expected idColumnWidth=1, got %d", tl.idColumnWidth)
	}
}

func TestSetTasks_EmptyList(t *testing.T) {
	tl := NewTaskList(10)
	tl.SetTasks(nil)

	if tl.idColumnWidth != 0 {
		t.Errorf("Expected idColumnWidth=0, got %d", tl.idColumnWidth)
	}
	if tl.selectionIndex != 0 {
		t.Errorf("Expected selectionIndex=0, got %d", tl.selectionIndex)
	}
}

func TestSelection_ClampsBounds(t *testing.T) {
	tl := NewTaskList(10)
	tl.SetTasks(makeTasks("A", "B", "C"))

	tl.SetSelection(-5)
	if tl.selectionIndex != 0 {
		t.Errorf("Expected clamped to 0, got %d", tl.selectionIndex)
	}

	tl.SetSelection(100)
	if tl.selectionIndex != 2 {
		t.Errorf("Expected clamped to 2, got %d", tl.selectionIndex)
	}

	tl.SetSelection(1)
	if tl.selectionIndex != 1 {
		t.Errorf("Expected 1, got %d", tl.selectionIndex)
	}
}

func TestGetSelectedTask(t *testing.T) {
	tl := NewTaskList(10)
	tasks := makeTasks("A", "B", "C")
	tl.SetTasks(tasks)

	tl.SetSelection(1)
	selected := tl.GetSelectedTask()
	if selected == nil || selected.ID != "B" {
		t.Errorf("Expected task B, got %v", selected)
	}
}

func TestGetSelectedTask_EmptyList(t *testing.T) {
	tl := NewTaskList(10)
	if tl.GetSelectedTask() != nil {
		t.Error("Expected nil for empty task list")
	}
}

func TestScrollDown(t *testing.T) {
	tl := NewTaskList(10)
	tl.SetTasks(makeTasks("A", "B", "C"))

	tl.ScrollDown()
	if tl.selectionIndex != 1 {
		t.Errorf("Expected 1 after ScrollDown, got %d", tl.selectionIndex)
	}

	tl.ScrollDown()
	if tl.selectionIndex != 2 {
		t.Errorf("Expected 2 after second ScrollDown, got %d", tl.selectionIndex)
	}

	// Should not go past last item
	tl.ScrollDown()
	if tl.selectionIndex != 2 {
		t.Errorf("Expected 2 (clamped), got %d", tl.selectionIndex)
	}
}

func TestScrollUp(t *testing.T) {
	tl := NewTaskList(10)
	tl.SetTasks(makeTasks("A", "B", "C"))
	tl.SetSelection(2)

	tl.ScrollUp()
	if tl.selectionIndex != 1 {
		t.Errorf("Expected 1 after ScrollUp, got %d", tl.selectionIndex)
	}

	tl.ScrollUp()
	if tl.selectionIndex != 0 {
		t.Errorf("Expected 0 after second ScrollUp, got %d", tl.selectionIndex)
	}

	// Should not go below 0
	tl.ScrollUp()
	if tl.selectionIndex != 0 {
		t.Errorf("Expected 0 (clamped), got %d", tl.selectionIndex)
	}
}

func TestScrollDown_EmptyList(t *testing.T) {
	tl := NewTaskList(10)
	// Should not panic
	tl.ScrollDown()
	tl.ScrollUp()
}

func TestFewerItemsThanViewport(t *testing.T) {
	tl := NewTaskList(10)
	tl.SetTasks(makeTasks("A", "B"))

	// scrollOffset should stay at 0 since all items fit
	if tl.scrollOffset != 0 {
		t.Errorf("Expected scrollOffset=0, got %d", tl.scrollOffset)
	}

	tl.ScrollDown()
	if tl.scrollOffset != 0 {
		t.Errorf("Expected scrollOffset=0 with fewer items, got %d", tl.scrollOffset)
	}
}

func TestSetIDColors(t *testing.T) {
	tl := NewTaskList(10)
	g := config.Gradient{Start: [3]int{255, 0, 0}, End: [3]int{0, 255, 0}}
	fb := config.NewColor(tcell.ColorRed)

	result := tl.SetIDColors(g, fb)
	if result != tl {
		t.Error("SetIDColors should return self for chaining")
	}
	if tl.idGradient != g {
		t.Error("Expected custom gradient")
	}
	if tl.idFallback != fb {
		t.Error("Expected custom fallback")
	}
}

func TestSetTitleColor(t *testing.T) {
	tl := NewTaskList(10)
	c := config.NewColor(tcell.ColorRed)
	result := tl.SetTitleColor(c)
	if result != tl {
		t.Error("SetTitleColor should return self for chaining")
	}
	if tl.titleColor != c {
		t.Errorf("Expected color red, got %v", tl.titleColor)
	}
}

func TestSetTasks_ClampsSelectionOnShrink(t *testing.T) {
	tl := NewTaskList(10)
	tl.SetTasks(makeTasks("A", "B", "C", "D", "E"))
	tl.SetSelection(4)

	// Shrink list — selection should clamp
	tl.SetTasks(makeTasks("A", "B"))
	if tl.selectionIndex != 1 {
		t.Errorf("Expected selectionIndex clamped to 1, got %d", tl.selectionIndex)
	}
}

func TestGetSelectedIndex(t *testing.T) {
	tl := NewTaskList(10)
	tl.SetTasks(makeTasks("A", "B", "C"))
	tl.SetSelection(2)

	if tl.GetSelectedIndex() != 2 {
		t.Errorf("Expected 2, got %d", tl.GetSelectedIndex())
	}
}

func TestBuildRow(t *testing.T) {
	tl := NewTaskList(10)

	pendingTask := &task.Task{ID: "TIKI-ABC001", Title: "My pending task", Status: task.StatusBacklog}
	doneTask := &task.Task{ID: "TIKI-ABC002", Title: "My done task", Status: task.StatusDone}

	// set tasks so idColumnWidth is computed
	tl.SetTasks([]*task.Task{pendingTask, doneTask})

	width := 80

	t.Run("pending task shows circle", func(t *testing.T) {
		row := tl.buildRow(pendingTask, false, width)
		if !strings.Contains(row, "\u25CB") {
			t.Error("pending task row should contain circle (○)")
		}
	})

	t.Run("done task shows checkmark", func(t *testing.T) {
		row := tl.buildRow(doneTask, false, width)
		if !strings.Contains(row, "\u2713") {
			t.Error("done task row should contain checkmark (✓)")
		}
	})

	t.Run("contains task ID", func(t *testing.T) {
		row := tl.buildRow(pendingTask, false, width)
		if !strings.Contains(row, pendingTask.ID) {
			t.Errorf("row should contain task ID %q", pendingTask.ID)
		}
	})

	t.Run("contains title", func(t *testing.T) {
		row := tl.buildRow(pendingTask, false, width)
		escaped := tview.Escape(pendingTask.Title)
		if !strings.Contains(row, escaped) {
			t.Errorf("row should contain escaped title %q", escaped)
		}
	})

	t.Run("selected row has selection color prefix", func(t *testing.T) {
		row := tl.buildRow(pendingTask, true, width)
		selTag := tl.selectionColor.Tag().WithBg(tl.selectionBgColor).String()
		if !strings.HasPrefix(row, selTag) {
			t.Errorf("selected row should start with selection color %q", selTag)
		}
	})

	t.Run("unselected row has no selection prefix", func(t *testing.T) {
		row := tl.buildRow(pendingTask, false, width)
		selTag := tl.selectionColor.Tag().WithBg(tl.selectionBgColor).String()
		if strings.HasPrefix(row, selTag) {
			t.Error("unselected row should not start with selection color")
		}
	})
}
