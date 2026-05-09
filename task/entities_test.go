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
		ID:                  "TIKI-ABC123",
		Title:               "Test task",
		Description:         "desc",
		Comments:            []Comment{{ID: "c1", Author: "bob", Text: "hello"}},
		CreatedBy:           "alice",
		CreatedAt:           time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:           time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		LoadedMtime:         time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		CustomFields:        map[string]interface{}{"flag": true, "labels": []string{"a", "b"}},
		UnknownFields:       map[string]interface{}{"random": 42},
		WorkflowFrontmatter: map[string]interface{}{"status": "ready"},
	}

	clone := original.Clone()

	if clone.ID != original.ID {
		t.Errorf("ID = %q, want %q", clone.ID, original.ID)
	}
	if clone.Title != original.Title {
		t.Errorf("Title = %q, want %q", clone.Title, original.Title)
	}
	if clone.Description != original.Description {
		t.Errorf("Description = %q, want %q", clone.Description, original.Description)
	}
	if clone.CreatedBy != original.CreatedBy {
		t.Errorf("CreatedBy = %q, want %q", clone.CreatedBy, original.CreatedBy)
	}

	// verify deep copy: mutate clone, original must be unaffected
	clone.Comments[0].Text = "MUTATED"
	if original.Comments[0].Text == "MUTATED" {
		t.Fatal("mutating clone Comments affected original — not a deep copy")
	}

	clone.CustomFields["flag"] = false
	if original.CustomFields["flag"] == false {
		t.Fatal("mutating clone CustomFields affected original — not a deep copy")
	}

	if labels, ok := clone.CustomFields["labels"].([]string); ok {
		labels[0] = "MUTATED"
		origLabels, _ := original.CustomFields["labels"].([]string)
		if origLabels[0] == "MUTATED" {
			t.Fatal("mutating clone CustomFields slice affected original — not a deep copy")
		}
	}

	clone.WorkflowFrontmatter["status"] = "done"
	if original.WorkflowFrontmatter["status"] == "done" {
		t.Fatal("mutating clone WorkflowFrontmatter affected original — not a deep copy")
	}
}

func TestClone_NilSlices(t *testing.T) {
	original := &Task{
		ID:    "TIKI-ABC123",
		Title: "bare task",
		// Comments, CustomFields, UnknownFields, WorkflowFrontmatter all nil
	}

	clone := original.Clone()

	if clone.Comments != nil {
		t.Error("nil Comments should remain nil in clone")
	}
	if clone.CustomFields != nil {
		t.Error("nil CustomFields should remain nil in clone")
	}
	if clone.UnknownFields != nil {
		t.Error("nil UnknownFields should remain nil in clone")
	}
	if clone.WorkflowFrontmatter != nil {
		t.Error("nil WorkflowFrontmatter should remain nil in clone")
	}
}
