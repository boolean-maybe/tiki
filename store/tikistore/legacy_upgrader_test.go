package tikistore

import (
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestLegacyUpgrader_UpgradeTiki pins the live load-side normalization:
// snake_case statuses written by old versions of tiki are rewritten to
// camelCase in the in-memory Fields map. Already-canonical values pass
// through unchanged. UpgradeTiki is the only upgrader path — UpgradeTask
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
		{"single word backlog", "backlog", "backlog"},
		{"single word ready", "ready", "ready"},
		{"single word review", "review", "review"},
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
