package util

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// FormatKeyBinding returns a display string for a keyboard shortcut.
// It handles rune keys, special keys, modifiers (Shift, Ctrl, Alt), and
// control character sequences (Ctrl+A through Ctrl+Z).
//
// Examples:
//   - FormatKeyBinding(tcell.KeyCtrlS, 0, 0) → "Ctrl+S"
//   - FormatKeyBinding(tcell.KeyEnter, 0, tcell.ModShift) → "Shift+Enter"
//   - FormatKeyBinding(tcell.KeyEscape, 0, 0) → "Esc"
//   - FormatKeyBinding(tcell.KeyRune, 's', tcell.ModCtrl) → "Ctrl+s"
func FormatKeyBinding(key tcell.Key, ch rune, mod tcell.ModMask) string {
	// For rune keys (including with modifiers like Ctrl+R), build the full string
	if ch != 0 {
		prefix := ""
		if mod&tcell.ModShift != 0 {
			prefix += "Shift+"
		}
		if mod&tcell.ModCtrl != 0 {
			prefix += "Ctrl+"
		}
		if mod&tcell.ModAlt != 0 {
			prefix += "Alt+"
		}
		return prefix + string(ch)
	}

	// For special keys, check if tcell already provides the full name with modifiers
	// Note: This must come before the Ctrl+letter check because some named keys
	// (Tab=9, Enter=13, Backspace=8) have values in the 1-26 range
	if name, ok := tcell.KeyNames[key]; ok {
		// If the key name already includes a modifier (e.g., "Ctrl-R"), use it as-is
		if strings.Contains(name, "Ctrl-") || strings.Contains(name, "Alt-") ||
			strings.Contains(name, "Shift-") || strings.Contains(name, "Meta-") {
			return name
		}

		// Otherwise, build modifier prefix and append key name
		prefix := ""
		if mod&tcell.ModShift != 0 {
			prefix += "Shift+"
		}
		if mod&tcell.ModCtrl != 0 {
			prefix += "Ctrl+"
		}
		if mod&tcell.ModAlt != 0 {
			prefix += "Alt+"
		}
		return prefix + name
	}

	// Handle Ctrl+letter keys (KeyCtrlA=1 through KeyCtrlZ=26, not in KeyNames map)
	// This is a fallback for control keys that don't have explicit names
	if key >= 1 && key <= 26 {
		letter := rune('A' + key - 1)
		return "Ctrl+" + string(letter)
	}

	return "?"
}
