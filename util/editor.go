package util

import (
	"os"
	"os/exec"
)

// GetDefaultEditor returns the user's preferred editor from environment variables.
// Checks VISUAL, then EDITOR, then falls back to vi.
func GetDefaultEditor() string {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi" // fallback default
	}
	return editor
}

// OpenInEditor opens the specified file in the user's default editor.
// The function blocks until the editor exits.
// Returns any error that occurred while running the editor.
func OpenInEditor(filename string) error {
	editor := GetDefaultEditor()

	//nolint:gosec // G204: editor from env var or system default, filename is controlled
	cmd := exec.Command(editor, filename)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
