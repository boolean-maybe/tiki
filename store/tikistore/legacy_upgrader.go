package tikistore

import (
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// LegacyUpgrader transforms documents loaded from disk to conform to current
// naming conventions. Every document loaded from a file passes through
// UpgradeTiki before entering the in-memory store.
//
// Documents are NOT rewritten to disk by the upgrader. They are persisted in
// the new format only when modified and saved through normal store operations.
type LegacyUpgrader struct{}

// UpgradeTiki normalizes legacy field values in a Tiki to current conventions.
// Currently handles:
//   - status "in_progress" → "inProgress" (snake_case → camelCase)
func (u *LegacyUpgrader) UpgradeTiki(tk *tikipkg.Tiki) {
	if tk == nil {
		return
	}
	if s, ok := tk.Fields["status"].(string); ok && s != "" {
		mapped := string(taskpkg.MapStatus(s))
		if mapped != s {
			tk.Set("status", mapped)
		}
	}
}

// UpgradeTask normalizes legacy field values to current conventions.
// Phase 5 compatibility adapter — delegates to UpgradeTiki via round-trip.
func (u *LegacyUpgrader) UpgradeTask(t *taskpkg.Task) *taskpkg.Task {
	t.Status = taskpkg.MapStatus(string(t.Status))
	return t
}
