package plugin

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// keyName returns a human-readable string for the activation key
func keyName(key tcell.Key, r rune) string {
	if key == tcell.KeyRune {
		return string(r)
	}
	return tcell.KeyNames[key]
}

// parseKey parses a key string into a tcell.Key, a rune, and a modifier mask.
// Supported formats:
// - single rune: "B" (case-preserving)
// - function keys: "F1", "F2", ..., "F12" (case-insensitive)
// - ctrl combos: "Ctrl-U", "Ctrl-F1" (case-insensitive)
// - alt combos: "Alt-M", "Alt-F2" (case-insensitive)
// - shift combos: "Shift-F", "Shift-F3" (case-insensitive)
func parseKey(s string) (tcell.Key, rune, tcell.ModMask, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, 0, nil
	}

	upper := strings.ToUpper(s)

	// Handle Ctrl-X notation
	if strings.HasPrefix(upper, "CTRL-") {
		rest := upper[5:]
		// Check for F-keys first (before treating F as a letter)
		if key, ok := parseFunctionKey(rest); ok {
			return key, 0, tcell.ModCtrl, nil
		}
		char := []rune(rest)
		if len(char) == 1 && char[0] >= 'A' && char[0] <= 'Z' {
			// tcell.KeyCtrlA is 65 ('A'), KeyCtrlZ is 90 ('Z')
			return tcell.Key(char[0]), 0, tcell.ModCtrl, nil //nolint:gosec // G115: char[0] bounded 'A'-'Z' (65-90), safe to convert to int16
		}
		return 0, 0, 0, fmt.Errorf("invalid ctrl key: %q (expected Ctrl-A..Ctrl-Z or Ctrl-F1..Ctrl-F12)", s)
	}

	// Handle Alt-X notation
	if strings.HasPrefix(upper, "ALT-") {
		rest := upper[4:]
		// Check for F-keys first (before treating F as a letter)
		if key, ok := parseFunctionKey(rest); ok {
			return key, 0, tcell.ModAlt, nil
		}
		char := []rune(rest)
		if len(char) == 1 && char[0] >= 'A' && char[0] <= 'Z' {
			return tcell.KeyRune, char[0], tcell.ModAlt, nil
		}
		return 0, 0, 0, fmt.Errorf("invalid alt key: %q (expected Alt-A..Alt-Z or Alt-F1..Alt-F12)", s)
	}

	// Handle Shift-X notation
	if strings.HasPrefix(upper, "SHIFT-") {
		rest := upper[6:]
		// Check for F-keys first (before treating F as a letter)
		if key, ok := parseFunctionKey(rest); ok {
			return key, 0, tcell.ModShift, nil
		}
		char := []rune(rest)
		if len(char) == 1 && char[0] >= 'A' && char[0] <= 'Z' {
			return tcell.KeyRune, char[0], tcell.ModShift, nil
		}
		return 0, 0, 0, fmt.Errorf("invalid shift key: %q (expected Shift-A..Shift-Z or Shift-F1..Shift-F12)", s)
	}

	// Check for standalone F-keys
	if key, ok := parseFunctionKey(upper); ok {
		return key, 0, 0, nil
	}

	// Otherwise require exactly one rune.
	runes := []rune(s)
	if len(runes) != 1 {
		return 0, 0, 0, fmt.Errorf("invalid key: %q (expected single character, F1..F12, Ctrl-X, Alt-X, or Shift-X)", s)
	}
	return tcell.KeyRune, runes[0], 0, nil
}

// parseFunctionKey parses function key notation (F1, F2, ..., F12)
// Returns the tcell.Key constant and true if valid, 0 and false otherwise
func parseFunctionKey(s string) (tcell.Key, bool) {
	if !strings.HasPrefix(s, "F") {
		return 0, false
	}

	// Parse the number after 'F'
	numStr := s[1:]
	if numStr == "" {
		return 0, false
	}

	// Simple integer parsing for 1-12
	var num int
	for _, ch := range numStr {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		num = num*10 + int(ch-'0')
	}

	// Map to tcell key constants (F1 = KeyF1, F2 = KeyF2, etc.)
	// tcell.KeyF1 = 279, KeyF2 = 280, ..., KeyF12 = 290
	if num >= 1 && num <= 12 {
		//nolint:gosec // G115: num is bounded 1-12, safe to convert to int16
		return tcell.Key(278 + num), true
	}

	return 0, false
}

// normalizeParsedKey collapses equivalent bindings to a canonical form.
// Shift+letter that only aliases the uppercase rune drops ModShift.
// Examples: Shift-x → ('X', 0), Shift-X → ('X', 0), X → ('X', 0), x stays ('x', 0).
func normalizeParsedKey(key tcell.Key, r rune, mod tcell.ModMask) (tcell.Key, rune, tcell.ModMask) {
	if key == tcell.KeyRune && mod == tcell.ModShift && r >= 'A' && r <= 'Z' {
		return tcell.KeyRune, r, 0
	}
	return key, r, mod
}

// formatKeyStr produces a canonical string from a normalized binding.
// Modified keys use Modifier-X format (e.g. "Ctrl-U", "Alt-M", "Shift-F3").
// Standalone function keys use "F5" format.
// Plain runes use string(r).
func formatKeyStr(key tcell.Key, r rune, mod tcell.ModMask) string {
	var prefix string
	switch {
	case mod&tcell.ModCtrl != 0:
		prefix = "Ctrl-"
	case mod&tcell.ModAlt != 0:
		prefix = "Alt-"
	case mod&tcell.ModShift != 0:
		prefix = "Shift-"
	}

	if key == tcell.KeyRune {
		return prefix + string(r)
	}

	// ctrl+letter keys (KeyCtrlA=65 .. KeyCtrlZ=90) already include "Ctrl-" in
	// their tcell.KeyNames entry, so use the letter directly to avoid doubling.
	if key >= tcell.KeyCtrlA && key <= tcell.KeyCtrlZ {
		letter := rune(key-tcell.KeyCtrlA) + 'A'
		// prefix already has "Ctrl-" from ModCtrl; for bare ctrl keys without
		// explicit ModCtrl (shouldn't happen after normalization), still produce "Ctrl-X"
		if prefix == "" {
			prefix = "Ctrl-"
		}
		return prefix + string(letter)
	}

	// function keys
	if key >= tcell.KeyF1 && key <= tcell.KeyF12 {
		num := int(key-tcell.KeyF1) + 1
		return fmt.Sprintf("%sF%d", prefix, num)
	}

	// fallback to tcell name (without doubling modifier)
	if name, ok := tcell.KeyNames[key]; ok {
		return prefix + name
	}
	return prefix + fmt.Sprintf("Key(%d)", key)
}

// parseCanonicalKey is the single entry point for all config-originated key parsing.
// It parses, normalizes, validates standalone runes, and returns the canonical KeyStr.
func parseCanonicalKey(s string) (tcell.Key, rune, tcell.ModMask, string, error) {
	key, r, mod, err := parseKey(s)
	if err != nil {
		return 0, 0, 0, "", err
	}

	// empty key binding (valid for optional activation keys)
	if key == 0 && r == 0 && mod == 0 {
		return 0, 0, 0, "", nil
	}

	key, r, mod = normalizeParsedKey(key, r, mod)

	// standalone rune validation: reject non-printable runes
	if key == tcell.KeyRune && mod == 0 && !unicode.IsPrint(r) {
		return 0, 0, 0, "", fmt.Errorf("key must be a printable character, got %q", string(r))
	}

	keyStr := formatKeyStr(key, r, mod)
	return key, r, mod, keyStr, nil
}
