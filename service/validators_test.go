package service

import (
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

func TestWrapTikiFieldValidator_DeleteCase(t *testing.T) {
	// when new is nil (delete), the validator should inspect old
	validator := wrapTikiFieldValidator(func(tk *tikipkg.Tiki) string {
		if tk.Title == "" {
			return "title required"
		}
		return ""
	})

	old := &tikipkg.Tiki{ID: "DEL001", Title: "has title"}
	// new is nil → delete case, validator should use old
	rejection := validator(old, nil, nil)
	if rejection != nil {
		t.Errorf("expected no rejection for valid old tiki, got: %s", rejection.Reason)
	}

	// old with empty title → validator should reject
	badOld := &tikipkg.Tiki{ID: "DEL002"}
	rejection = validator(badOld, nil, nil)
	if rejection == nil {
		t.Fatal("expected rejection for old tiki with empty title")
		return
	}
	if rejection.Reason != "title required" {
		t.Errorf("expected 'title required', got %q", rejection.Reason)
	}
}
