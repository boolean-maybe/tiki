package tikistore

import (
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestLegacyUpgrader_UpgradeTiki pins the live load-side normalization:
// snake_case statuses written by old versions of tiki are rewritten to
// camelCase in the in-memory Fields map. Already-canonical values pass
// through unchanged. UpgradeTiki is the only upgrader path — UpgradeTiki
// was removed in Phase 7 cleanup.
func TestLegacyUpgrader_UpgradeTiki(t *testing.T) {
	upgrader := &LegacyUpgrader{}

	tests := []struct {
		name       string
		status     string
		wantStatus string
	}{
		{"snake_case in_progress → inProgress", "in_progress", "inProgress"},
		{"already camelCase inProgress", "inProgress", "inProgress"},
		{"single word done", "done", "done"},
		{"single word inbox", "inbox", "inbox"},
		{"single word ready", "ready", "ready"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := tikipkg.New()
			tk.ID = "TEST01"
			tk.Title = "test"
			tk.Set("status", tt.status)

			upgrader.UpgradeTiki(tk)

			got, _, _ := tk.StringField("status")
			if got != tt.wantStatus {
				t.Errorf("UpgradeTiki status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

// TestLegacyUpgrader_UpgradeTiki_LeavesAbsentStatusAlone verifies the no-op
// path: a tiki without a status field must not gain one through the upgrader.
// Exact-presence is preserved.
func TestLegacyUpgrader_UpgradeTiki_LeavesAbsentStatusAlone(t *testing.T) {
	upgrader := &LegacyUpgrader{}

	tk := tikipkg.New()
	tk.ID = "TEST02"
	tk.Title = "no status"

	upgrader.UpgradeTiki(tk)

	if tk.Has("status") {
		t.Error("UpgradeTiki added a status field to a tiki that didn't have one")
	}
}

// TestLegacyUpgrader_UpgradeTiki_NilSafe verifies the upgrader is safe to
// call with a nil *Tiki — the persistence layer should never pass one,
// but defensive handling keeps a load-time crash from corrupting the
// in-memory store.
func TestLegacyUpgrader_UpgradeTiki_NilSafe(t *testing.T) {
	upgrader := &LegacyUpgrader{}
	upgrader.UpgradeTiki(nil) // must not panic
}

// TestLegacyUpgrader_PriorityIntToEnum pins the migration: pre-Phase-3 docs
// stored priority as 1..5; the upgrader rewrites those into the canonical
// enum keys declared in workflow.yaml, ranked by position. Without this,
// any existing user document with `priority: 2` would load with priority
// demoted to "stale unknown" and lose its sort/filter behavior.
func TestLegacyUpgrader_PriorityIntToEnum(t *testing.T) {
	upgrader := &LegacyUpgrader{}

	tests := []struct {
		name string
		raw  interface{}
		want string
	}{
		{"int 1 → high", 1, "high"},
		{"int 2 → medium-high", 2, "medium-high"},
		{"int 3 → medium", 3, "medium"},
		{"int 4 → medium-low", 4, "medium-low"},
		{"int 5 → low", 5, "low"},
		{"float64 3.0 → medium (YAML can decode as float)", float64(3), "medium"},
		{"int64 2 → medium-high", int64(2), "medium-high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := tikipkg.New()
			tk.ID = "TEST"
			tk.Set("priority", tt.raw)

			upgrader.UpgradeTiki(tk)

			got, _, _ := tk.StringField("priority")
			if got != tt.want {
				t.Errorf("priority = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestLegacyUpgrader_PriorityOutOfRangeFallsBackToDefault pins the
// safety-net for ranks outside the configured enum: a stale `priority: 7`
// (out of range for a 5-level enum) lands on the workflow's declared
// default rather than staying stale-unknown.
func TestLegacyUpgrader_PriorityOutOfRangeFallsBackToDefault(t *testing.T) {
	upgrader := &LegacyUpgrader{}

	tk := tikipkg.New()
	tk.ID = "TEST"
	tk.Set("priority", 7) // out of range for the canonical 5-level enum

	upgrader.UpgradeTiki(tk)

	got, _, _ := tk.StringField("priority")
	if got != "medium" {
		t.Errorf("priority = %q, want %q (default fallback)", got, "medium")
	}
}

// TestLegacyUpgrader_PriorityFractionalFloatStaysStale pins the
// truncation safety net: a non-integer float (e.g. priority: 2.9) was
// never a legitimate legacy value, so the upgrader must leave it alone
// rather than silently truncating to rank 2 and writing a valid enum
// key. The on-disk value should round-trip as stale-unknown for manual fixup.
func TestLegacyUpgrader_PriorityFractionalFloatStaysStale(t *testing.T) {
	upgrader := &LegacyUpgrader{}

	tk := tikipkg.New()
	tk.ID = "TEST"
	tk.Set("priority", float64(2.9))

	upgrader.UpgradeTiki(tk)

	got := tk.Fields["priority"]
	// Must remain a non-string (the upgrader didn't touch it).
	if _, isString := got.(string); isString {
		t.Errorf("priority was migrated to a string %v; fractional floats must stay stale for repair", got)
	}
	if got != float64(2.9) {
		t.Errorf("priority = %v, want 2.9 (untouched)", got)
	}
}

// TestLegacyUpgrader_PriorityWholeFloatStillMigrates pins the converse:
// YAML decoders sometimes deliver whole numbers as float64 ("2" vs "2.0")
// — the upgrader must continue to accept those as legitimate legacy ranks.
func TestLegacyUpgrader_PriorityWholeFloatStillMigrates(t *testing.T) {
	upgrader := &LegacyUpgrader{}

	tk := tikipkg.New()
	tk.ID = "TEST"
	tk.Set("priority", float64(2)) // whole-number float

	upgrader.UpgradeTiki(tk)

	got, _, _ := tk.StringField("priority")
	if got != "medium-high" {
		t.Errorf("priority = %q, want %q", got, "medium-high")
	}
}

// TestLegacyUpgrader_UpgradeTiki_UnknownStatusFallsBackToDefault pins the
// regression behavior of the legacy MapStatus contract: a status key that
// is neither already canonical nor reaches a recognized key after camel-
// case normalization (e.g. retired aliases like "closed", "todo",
// "completed", "open") must be migrated to the workflow's declared
// default, not left in place. Leaving it stale would keep the document
// loading with an out-of-domain status — silently breaking lane filters
// and any "where status = X" ruki queries.
func TestLegacyUpgrader_UpgradeTiki_UnknownStatusFallsBackToDefault(t *testing.T) {
	upgrader := &LegacyUpgrader{}

	tests := []struct {
		name       string
		status     string
		wantStatus string
	}{
		{"retired alias closed", "closed", "inbox"},
		{"retired alias todo", "todo", "inbox"},
		{"retired alias completed", "completed", "inbox"},
		{"retired alias open", "open", "inbox"},
		{"retired alias backlog", "backlog", "inbox"},
		{"retired alias review", "review", "inbox"},
		{"random text", "foobar", "inbox"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := tikipkg.New()
			tk.ID = "TEST01"
			tk.Title = "test"
			tk.Set("status", tt.status)

			upgrader.UpgradeTiki(tk)

			got, _, _ := tk.StringField("status")
			if got != tt.wantStatus {
				t.Errorf("UpgradeTiki status = %q, want %q (default fallback)", got, tt.wantStatus)
			}
		})
	}
}

// TestLegacyUpgrader_PriorityStringPasses verifies the no-op path: a
// priority that is already a canonical key passes through unchanged.
func TestLegacyUpgrader_PriorityStringPasses(t *testing.T) {
	upgrader := &LegacyUpgrader{}

	tk := tikipkg.New()
	tk.ID = "TEST"
	tk.Set("priority", "medium-high")

	upgrader.UpgradeTiki(tk)

	got, _, _ := tk.StringField("priority")
	if got != "medium-high" {
		t.Errorf("priority changed unexpectedly: got %q, want medium-high", got)
	}
}
