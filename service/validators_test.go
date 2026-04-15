package service

import (
	"testing"

	"github.com/boolean-maybe/tiki/task"
)

func TestWrapFieldValidator_DeleteCase(t *testing.T) {
	// when new is nil (delete), the validator should inspect old
	validator := wrapFieldValidator(func(tk *task.Task) string {
		if tk.Title == "" {
			return "title required"
		}
		return ""
	})

	old := &task.Task{ID: "TIKI-DEL001", Title: "has title", Priority: 3}
	// new is nil → delete case, validator should use old
	rejection := validator(old, nil, nil)
	if rejection != nil {
		t.Errorf("expected no rejection for valid old task, got: %s", rejection.Reason)
	}

	// old with empty title → validator should reject
	badOld := &task.Task{ID: "TIKI-DEL002", Title: "", Priority: 3}
	rejection = validator(badOld, nil, nil)
	if rejection == nil {
		t.Fatal("expected rejection for old task with empty title")
		return
	}
	if rejection.Reason != "title required" {
		t.Errorf("expected 'title required', got %q", rejection.Reason)
	}
}
