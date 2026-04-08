package tikistore

import (
	taskpkg "github.com/boolean-maybe/tiki/task"
)

// LegacyUpgrader transforms tasks loaded from disk to conform to current
// naming conventions. Every task loaded from a file passes through
// UpgradeTask before entering the in-memory store.
//
// Tasks are NOT rewritten to disk by the upgrader. They are persisted in
// the new format only when modified and saved through normal store operations.
type LegacyUpgrader struct{}

// UpgradeTask normalizes legacy field values to current conventions.
// Currently handles:
//   - status "in_progress" → "inProgress" (snake_case → camelCase)
func (u *LegacyUpgrader) UpgradeTask(t *taskpkg.Task) *taskpkg.Task {
	t.Status = taskpkg.MapStatus(string(t.Status))
	return t
}
