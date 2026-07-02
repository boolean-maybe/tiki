package util

import "testing"

func TestSupportsKittyGraphics(t *testing.T) {
	t.Run("ghostty", func(t *testing.T) {
		clearCMUXEnv(t)
		t.Setenv("GHOSTTY_RESOURCES_DIR", "/Applications/Ghostty.app/Contents/Resources")

		if !SupportsKittyGraphics() {
			t.Fatal("expected Ghostty to support Kitty graphics")
		}
	})

	t.Run("cmux ghostty shell", func(t *testing.T) {
		t.Setenv("CMUX_SHELL_INTEGRATION", "1")
		t.Setenv("GHOSTTY_RESOURCES_DIR", "/Applications/cmux.app/Contents/Resources/ghostty")
		t.Setenv("TERM_PROGRAM", "ghostty")

		if SupportsKittyGraphics() {
			t.Fatal("expected cmux to disable Kitty graphics")
		}
	})
}

func clearCMUXEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CMUX_SHELL_INTEGRATION", "")
	t.Setenv("CMUX_BUNDLE_ID", "")
	t.Setenv("CMUX_SOCKET_PATH", "")
}
