package util

import "os"

// SupportsKittyGraphics returns true if the terminal supports the Kitty
// graphics protocol (Unicode placeholders). Detection is env-var based:
// Kitty sets KITTY_WINDOW_ID, WezTerm sets WEZTERM_EXECUTABLE, Ghostty
// sets GHOSTTY_RESOURCES_DIR, and Konsole identifies via TERM_PROGRAM.
func SupportsKittyGraphics() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if os.Getenv("WEZTERM_EXECUTABLE") != "" {
		return true
	}
	if os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return true
	}
	if os.Getenv("TERM_PROGRAM") == "Konsole" {
		return true
	}
	return false
}
