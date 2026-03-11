package util

import "os"

// SupportsKittyGraphics returns true if the terminal supports the Kitty
// graphics protocol (Unicode placeholders). Detection is env-var based:
// Kitty sets KITTY_WINDOW_ID, WezTerm sets WEZTERM_EXECUTABLE, Ghostty
// sets GHOSTTY_RESOURCES_DIR, and iTerm2/Konsole identify via TERM_PROGRAM.
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
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app", "Konsole":
		return true
	}
	return false
}
