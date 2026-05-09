package tikistore

import (
	"strings"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
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
		mapped := upgradeLegacyStatus(s)
		if mapped != s {
			tk.Set("status", mapped)
		}
	}
	upgradeLegacyPriority(tk)
}

// upgradeLegacyStatus normalizes a raw status key to canonical camelCase
// against the loaded status field's enum. Splits on "_", "-", " ", and
// camelCase boundaries, lowercases, then re-camelizes. Resolution order:
//  1. normalized form is a recognized enum key → use it.
//  2. status field is configured but the normalized form is not in the
//     enum (e.g. legacy "closed", "todo") → fall back to the workflow's
//     declared default so the document loads with a valid status rather
//     than staying stale-unknown.
//  3. status field is not configured (plain-document workflow) → return
//     the normalized form, since there is nothing to validate against.
//
// Returning the original on the no-config path mirrors the priority
// upgrader's "leave it alone if there's no enum to validate against"
// stance.
func upgradeLegacyStatus(s string) string {
	normalized := normalizeCamelKey(s)
	if normalized == "" {
		return s
	}
	fd, ok := workflow.Field("status")
	if !ok || fd.Type != workflow.TypeEnum {
		return normalized
	}
	if fd.IsValidEnum(normalized) {
		return normalized
	}
	if def := fd.EnumDefault(); def != "" {
		return def
	}
	return normalized
}

// normalizeCamelKey lowercases, splits on separator/camel boundaries, then
// rejoins as camelCase. e.g. "in_progress" → "inProgress",
// "IN PROGRESS" → "inProgress".
func normalizeCamelKey(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var words []string
	for _, p := range parts {
		words = append(words, splitCamelBoundaries(p)...)
	}
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	for i, w := range words {
		if i == 0 {
			b.WriteString(strings.ToLower(w))
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]))
		b.WriteString(strings.ToLower(w[1:]))
	}
	return b.String()
}

func splitCamelBoundaries(s string) []string {
	if s == "" {
		return nil
	}
	var words []string
	start := 0
	for i := 1; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' && s[i-1] >= 'a' && s[i-1] <= 'z' {
			words = append(words, s[start:i])
			start = i
		}
	}
	words = append(words, s[start:])
	return words
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
	fd, ok := workflow.Field("priority")
	if !ok || fd.Type != workflow.TypeEnum {
		return
	}
	allowed := fd.AllowedValues()
	if rank >= 1 && rank <= len(allowed) {
		tk.Set("priority", allowed[rank-1])
		return
	}
	// rank is out of range for the configured enum — fall back to the
	// declared default so the tiki ends up with a valid key rather than
	// staying stale.
	if def := fd.EnumDefault(); def != "" {
		tk.Set("priority", def)
	}
}
