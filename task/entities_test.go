package task

import (
	"testing"
	"time"
)

func TestClone_NilTask(t *testing.T) {
	var nilTask *Task
	if nilTask.Clone() != nil {
		t.Fatal("Clone of nil task should return nil")
	}
}

func TestClone_FullTask(t *testing.T) {
	original := &Task{
		ID:          "TIKI-ABC123",
		Title:       "Test task",
		Description: "desc",
		Type:        TypeStory,
		Status:      StatusReady,
		Tags:        []string{"a", "b"},
		DependsOn:   []string{"TIKI-DEP001"},
		Due:         time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		Recurrence:  RecurrenceDaily,
		Assignee:    "alice",
		Priority:    2,
		Points:      5,
		Comments:    []Comment{{ID: "c1", Author: "bob", Text: "hello"}},
		CreatedBy:   "alice",
		CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		LoadedMtime: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	clone := original.Clone()

	// verify all scalar fields copied
	if clone.ID != original.ID {
		t.Errorf("ID = %q, want %q", clone.ID, original.ID)
	}
	if clone.Title != original.Title {
		t.Errorf("Title = %q, want %q", clone.Title, original.Title)
	}
	if clone.Priority != original.Priority {
		t.Errorf("Priority = %d, want %d", clone.Priority, original.Priority)
	}
	if clone.Points != original.Points {
		t.Errorf("Points = %d, want %d", clone.Points, original.Points)
	}
	if clone.Recurrence != original.Recurrence {
		t.Errorf("Recurrence = %v, want %v", clone.Recurrence, original.Recurrence)
	}

	// verify deep copy: mutate clone slices, original must be unaffected
	clone.Tags[0] = "MUTATED"
	if original.Tags[0] == "MUTATED" {
		t.Fatal("mutating clone Tags affected original — not a deep copy")
	}

	clone.DependsOn[0] = "MUTATED"
	if original.DependsOn[0] == "MUTATED" {
		t.Fatal("mutating clone DependsOn affected original — not a deep copy")
	}

	clone.Comments[0].Text = "MUTATED"
	if original.Comments[0].Text == "MUTATED" {
		t.Fatal("mutating clone Comments affected original — not a deep copy")
	}
}

func TestClone_NilSlices(t *testing.T) {
	original := &Task{
		ID:    "TIKI-ABC123",
		Title: "bare task",
		// Tags, DependsOn, Comments are all nil
	}

	clone := original.Clone()

	if clone.Tags != nil {
		t.Error("nil Tags should remain nil in clone")
	}
	if clone.DependsOn != nil {
		t.Error("nil DependsOn should remain nil in clone")
	}
	if clone.Comments != nil {
		t.Error("nil Comments should remain nil in clone")
	}
}
