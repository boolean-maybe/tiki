package sysinfo

import (
	"runtime"
	"testing"
)

func TestDetectTheme(t *testing.T) {
	tests := []struct {
		name      string
		colorFGBG string
		want      string
	}{
		{
			name:      "dark theme",
			colorFGBG: "15;0",
			want:      "dark",
		},
		{
			name:      "light theme",
			colorFGBG: "0;15",
			want:      "light",
		},
		{
			name:      "light theme with bg=8",
			colorFGBG: "0;8",
			want:      "light",
		},
		{
			name:      "dark theme with bg=7",
			colorFGBG: "15;7",
			want:      "dark",
		},
		{
			name:      "empty string",
			colorFGBG: "",
			want:      "unknown",
		},
		{
			name:      "invalid format - single value",
			colorFGBG: "15",
			want:      "unknown",
		},
		{
			name:      "multiple values - use last",
			colorFGBG: "15;0;8",
			want:      "light",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectTheme(tt.colorFGBG)
			if got != tt.want {
				t.Errorf("detectTheme(%q) = %q, want %q", tt.colorFGBG, got, tt.want)
			}
		})
	}
}

func TestGetColorSupportFromTerminfo(t *testing.T) {
	tests := []struct {
		name            string
		termEnv         string
		colortermEnv    string
		wantSupport     string
		wantColorCount  int
		shouldBeUnknown bool
	}{
		{
			name:            "empty TERM",
			termEnv:         "",
			colortermEnv:    "",
			wantSupport:     "unknown",
			wantColorCount:  0,
			shouldBeUnknown: true,
		},
		{
			name:            "invalid TERM",
			termEnv:         "nonexistent-terminal-type",
			colortermEnv:    "",
			wantSupport:     "unknown",
			wantColorCount:  0,
			shouldBeUnknown: true,
		},
		{
			name:            "COLORTERM=truecolor overrides TERM",
			termEnv:         "xterm-256color",
			colortermEnv:    "truecolor",
			wantSupport:     "truecolor",
			wantColorCount:  16777216,
			shouldBeUnknown: false,
		},
		{
			name:            "COLORTERM=24bit indicates truecolor",
			termEnv:         "xterm",
			colortermEnv:    "24bit",
			wantSupport:     "truecolor",
			wantColorCount:  16777216,
			shouldBeUnknown: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables for this test
			t.Setenv("TERM", tt.termEnv)
			// Always set COLORTERM (even to empty string) to override parent environment
			t.Setenv("COLORTERM", tt.colortermEnv)

			support, count := getColorSupportFromTerminfo()

			if tt.shouldBeUnknown {
				if support != "unknown" {
					t.Errorf("getColorSupportFromTerminfo() support = %q, want %q", support, "unknown")
				}
				if count != 0 {
					t.Errorf("getColorSupportFromTerminfo() count = %d, want 0", count)
				}
				return
			}

			if support != tt.wantSupport {
				t.Errorf("getColorSupportFromTerminfo() support = %q, want %q", support, tt.wantSupport)
			}
			if count != tt.wantColorCount {
				t.Errorf("getColorSupportFromTerminfo() count = %d, want %d", count, tt.wantColorCount)
			}
		})
	}

	// Test with actual TERM value (if available)
	t.Run("current TERM", func(t *testing.T) {
		support, count := getColorSupportFromTerminfo()

		// Should return valid results if TERM is set
		if support == "unknown" && count == 0 {
			t.Skip("TERM not set or invalid, skipping")
		}

		// Validate that returned values make sense
		validSupports := map[string]bool{
			"monochrome": true,
			"16-color":   true,
			"256-color":  true,
			"truecolor":  true,
			"unknown":    true,
		}

		if !validSupports[support] {
			t.Errorf("getColorSupportFromTerminfo() returned invalid support level: %q", support)
		}

		if count < 0 {
			t.Errorf("getColorSupportFromTerminfo() returned negative count: %d", count)
		}
	})
}

func TestNewSystemInfo(t *testing.T) {
	// Note: This test will use the actual environment
	// We can't fully mock the environment without complex setup

	info := NewSystemInfo()

	// Verify OS and Architecture are set
	if info.OS == "" {
		t.Error("NewSystemInfo() OS is empty")
	}
	if info.OS != runtime.GOOS {
		t.Errorf("NewSystemInfo() OS = %q, want %q", info.OS, runtime.GOOS)
	}

	if info.Architecture == "" {
		t.Error("NewSystemInfo() Architecture is empty")
	}
	if info.Architecture != runtime.GOARCH {
		t.Errorf("NewSystemInfo() Architecture = %q, want %q", info.Architecture, runtime.GOARCH)
	}

	// OSVersion should be set (might be "unknown")
	if info.OSVersion == "" {
		t.Error("NewSystemInfo() OSVersion is empty (expected at least 'unknown')")
	}

	// DetectedTheme should be "dark", "light", or "unknown"
	validThemes := map[string]bool{
		"dark":    true,
		"light":   true,
		"unknown": true,
	}
	if !validThemes[info.DetectedTheme] {
		t.Errorf("NewSystemInfo() DetectedTheme = %q, want one of [dark, light, unknown]", info.DetectedTheme)
	}

	// ColorSupport should be a valid value
	validSupports := map[string]bool{
		"monochrome": true,
		"16-color":   true,
		"256-color":  true,
		"truecolor":  true,
		"unknown":    true,
	}
	if !validSupports[info.ColorSupport] {
		t.Errorf("NewSystemInfo() ColorSupport = %q, want one of [monochrome, 16-color, 256-color, truecolor, unknown]", info.ColorSupport)
	}

	// Terminal dimensions should be 0 (no screen)
	if info.TerminalWidth != 0 {
		t.Errorf("NewSystemInfo() TerminalWidth = %d, want 0 (no screen)", info.TerminalWidth)
	}
	if info.TerminalHeight != 0 {
		t.Errorf("NewSystemInfo() TerminalHeight = %d, want 0 (no screen)", info.TerminalHeight)
	}

	// ProjectRoot should be set
	if info.ProjectRoot == "" {
		t.Error("NewSystemInfo() ProjectRoot is empty")
	}
}

func TestSystemInfoToMap(t *testing.T) {
	info := &SystemInfo{
		OS:            "darwin",
		Architecture:  "arm64",
		OSVersion:     "14.2.1",
		TermType:      "xterm-256color",
		ColorTerm:     "truecolor",
		ColorFGBG:     "15;0",
		DetectedTheme: "dark",
		ColorSupport:  "truecolor",
		ColorCount:    16777216,
	}

	m := info.ToMap()

	// Verify all fields are present
	expectedKeys := []string{
		"os", "architecture", "os_version", "term_type", "colorterm", "colorfgbg",
		"detected_theme", "terminal_width", "terminal_height",
		"color_support", "color_count", "xdg_config_home", "xdg_cache_home",
		"shell", "editor", "config_dir", "cache_dir", "project_root",
	}

	for _, key := range expectedKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("ToMap() missing key %q", key)
		}
	}

	// Verify values match
	if m["os"] != info.OS {
		t.Errorf("ToMap()[os] = %v, want %v", m["os"], info.OS)
	}
	if m["architecture"] != info.Architecture {
		t.Errorf("ToMap()[architecture] = %v, want %v", m["architecture"], info.Architecture)
	}
	if m["color_count"] != info.ColorCount {
		t.Errorf("ToMap()[color_count] = %v, want %v", m["color_count"], info.ColorCount)
	}
}

func TestSystemInfoString(t *testing.T) {
	info := &SystemInfo{
		OS:            "darwin",
		Architecture:  "arm64",
		OSVersion:     "14.2.1",
		TermType:      "xterm-256color",
		ColorTerm:     "truecolor",
		ColorFGBG:     "15;0",
		DetectedTheme: "dark",
		ColorSupport:  "truecolor",
		ColorCount:    16777216,
		Shell:         "/bin/zsh",
		Editor:        "vim",
		ConfigDir:     "/Users/test/.config/tiki",
		CacheDir:      "/Users/test/Library/Caches/tiki",
		ProjectRoot:   "/Users/test/project",
	}

	str := info.String()

	// Verify key sections are present
	expectedSections := []string{
		"System Information",
		"Terminal",
		"Environment",
		"Paths",
	}

	for _, section := range expectedSections {
		if !contains(str, section) {
			t.Errorf("String() missing section %q", section)
		}
	}

	// Verify key values are present
	expectedValues := []string{
		"darwin",
		"arm64",
		"14.2.1",
		"xterm-256color",
		"truecolor",
		"dark",
		"/bin/zsh",
		"vim",
		"/Users/test/.config/tiki",
	}

	for _, value := range expectedValues {
		if !contains(str, value) {
			t.Errorf("String() missing value %q", value)
		}
	}
}

func TestSystemInfoStringWithoutDimensions(t *testing.T) {
	info := &SystemInfo{
		OS:             "linux",
		Architecture:   "amd64",
		TerminalWidth:  0,
		TerminalHeight: 0,
	}

	str := info.String()

	// Should show "N/A" for dimensions when they're 0
	if !contains(str, "N/A") {
		t.Error("String() should show 'N/A' for dimensions when TerminalWidth and TerminalHeight are 0")
	}
}

func TestSystemInfoStringWithDimensions(t *testing.T) {
	info := &SystemInfo{
		OS:             "linux",
		Architecture:   "amd64",
		TerminalWidth:  120,
		TerminalHeight: 40,
	}

	str := info.String()

	// Should show actual dimensions
	if !contains(str, "120x40") {
		t.Error("String() should show '120x40' for dimensions")
	}
	if contains(str, "N/A") {
		t.Error("String() should not show 'N/A' when dimensions are set")
	}
}

func TestGetOSVersion(t *testing.T) {
	// Note: This test runs on the actual system
	// We can't easily mock OS-specific commands

	version := getOSVersion()

	// Version should be non-empty (even if it's "unknown")
	if version == "" {
		t.Error("getOSVersion() returned empty string")
	}

	// On current platform, should return something meaningful
	// (except on unsupported platforms where it returns "unknown")
	t.Logf("OS version detected: %s", version)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && containsLoop(s, substr))
}

func containsLoop(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
