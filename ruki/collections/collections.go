// Package collections holds small string-slice set helpers used by ruki and
// by tiki's store/config layers: trimming, de-duplication, and ref
// upper-casing.
package collections

import "strings"

// NormalizeStringSet trims values, drops empties, and removes duplicates.
func NormalizeStringSet(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if result == nil {
		return []string{}
	}
	return result
}

// NormalizeRefSet trims values, uppercases refs, drops empties, and removes duplicates.
func NormalizeRefSet(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToUpper(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if result == nil {
		return []string{}
	}
	return result
}
