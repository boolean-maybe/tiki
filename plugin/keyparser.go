package plugin

import (
	"fmt"
	"strings"

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
			return tcell.Key(char[0]), 0, tcell.ModCtrl, nil
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
