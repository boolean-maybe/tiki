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
//   - priority numeric ranks (1..N) → workflow enum keys (high, medium-high, ...)
//
// The priority migration runs after load-time coerceCustomValue has already
// stashed any int value as "stale unknown" — calling tk.Set on the upgraded
// key clears that stale marker so the field re-enters the validated set
// and round-trips correctly on save.
func (u *LegacyUpgrader) UpgradeTiki(tk *tikipkg.Tiki) {
	if tk == nil {
		return
	}
	if s, ok := tk.Fields["status"].(string); ok && s != "" {
		mapped := taskpkg.MapStatus(s)
		if mapped != s {
			tk.Set("status", mapped)
		}
	}
	upgradeLegacyPriority(tk)
}

// upgradeLegacyPriority converts a numeric priority rank stored in the
// Fields map (the pre-Phase-3 representation) to the canonical enum key.
// Accepts int and float64 (YAML numeric decoding lands on either depending
// on form). Out-of-range ranks fall back to the configured default; rank
// resolution that fails (no priority enum loaded) leaves the field alone
// so the stale-unknown marker preserves the original bytes for repair.
func upgradeLegacyPriority(tk *tikipkg.Tiki) {
	raw, ok := tk.Fields["priority"]
	if !ok {
		return
	}
	var rank int
	switch v := raw.(type) {
	case int:
		rank = v
	case int64:
		rank = int(v)
	case float64:
		// Only accept whole-number floats. YAML decodes "2" as int but
		// "2.0" as float64; either is a valid legacy rank. A non-integer
		// like 2.9 was never a valid priority — leave it as stale-unknown
		// so it surfaces during repair rather than silently truncating
		// to rank 2.
		if v != float64(int(v)) {
			return
		}
		rank = int(v)
	default:
		return // already a string, or some unsupported shape
	}
	if key, ok := taskpkg.LegacyPriorityKeyFromInt(rank); ok {
		tk.Set("priority", key)
		return
	}
	// rank is out of range for the configured enum — fall back to the
	// declared default so the tiki ends up with a valid key rather than
	// staying stale.
	if def := taskpkg.DefaultPriority(); def != "" {
		tk.Set("priority", def)
	}
}
