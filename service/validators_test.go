package service

import (
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

func TestWrapTikiFieldValidator_DeleteCase(t *testing.T) {
	// when new is nil (delete), the validator should inspect old
	validator := wrapTikiFieldValidator(func(tk *tikipkg.Tiki) string {
		if tk.Title() == "" {
			return "title required"
		}
		return ""
	})

	old := func() *tikipkg.Tiki { t := tikipkg.New(); t.SetID("DEL001"); t.SetTitle("has title"); return t }()
	// new is nil → delete case, validator should use old
	rejection := validator(old, nil, nil)
	if rejection != nil {
		t.Errorf("expected no rejection for valid old tiki, got: %s", rejection.Reason)
	}

	// old with empty title → validator should reject
	badOld := func() *tikipkg.Tiki { t := tikipkg.New(); t.SetID("DEL002"); return t }()
	rejection = validator(badOld, nil, nil)
	if rejection == nil {
		t.Fatal("expected rejection for old tiki with empty title")
		return
	}
	if rejection.Reason != "title required" {
		t.Errorf("expected 'title required', got %q", rejection.Reason)
	}
}
