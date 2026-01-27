package sysinfo

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/gdamore/tcell/v2"
)

// SystemInfo contains client environment information including OS details,
// terminal capabilities, and environment variables affecting visual appearance.
type SystemInfo struct {
	// OS Information
	OS           string // runtime.GOOS (darwin, linux, windows)
	Architecture string // runtime.GOARCH (amd64, arm64, etc.)
	OSVersion    string // OS version (from getOSVersion())

	// Terminal Information
	TermType      string // $TERM
	ColorTerm     string // $COLORTERM (truecolor indicator)
	ColorFGBG     string // $COLORFGBG
	DetectedTheme string // "dark", "light", "unknown"

	// Terminal Dimensions (requires tcell.Screen)
	TerminalWidth  int
	TerminalHeight int

	// Terminal Capabilities
	ColorSupport string // "monochrome", "16-color", "256-color", "truecolor"
	ColorCount   int    // Actual color count

	// Environment Variables
	XDGConfigHome string // $XDG_CONFIG_HOME
	XDGCacheHome  string // $XDG_CACHE_HOME
	Shell         string // $SHELL
	Editor        string // $VISUAL or $EDITOR

	// Paths (from config.PathManager)
	ConfigDir   string // From config.GetConfigDir()
	CacheDir    string // From config.GetCacheDir()
	ProjectRoot string // From os.Getwd()
}

// NewSystemInfo collects all system information using terminfo lookup (no screen needed).
// This is the fastest method and suitable for CLI commands and early bootstrap.
// Terminal dimensions are not available without a running screen.
func NewSystemInfo() *SystemInfo {
	info := &SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		OSVersion:    getOSVersion(),

		TermType:  os.Getenv("TERM"),
		ColorTerm: os.Getenv("COLORTERM"),
		ColorFGBG: os.Getenv("COLORFGBG"),

		XDGConfigHome: os.Getenv("XDG_CONFIG_HOME"),
		XDGCacheHome:  os.Getenv("XDG_CACHE_HOME"),
		Shell:         os.Getenv("SHELL"),
	}

	// Determine editor (prefer $VISUAL over $EDITOR)
	if visual := os.Getenv("VISUAL"); visual != "" {
		info.Editor = visual
	} else {
		info.Editor = os.Getenv("EDITOR")
	}

	// Detect theme from COLORFGBG
	info.DetectedTheme = detectTheme(info.ColorFGBG)

	// Get color support from terminfo (no screen needed)
	info.ColorSupport, info.ColorCount = getColorSupportFromTerminfo()

	// Get paths from config (these require config.InitPaths() to be called first)
	// If not initialized, these will be empty strings (panics are recovered)
	info.ConfigDir = getConfigDirSafe()
	info.CacheDir = getCacheDirSafe()

	// Get project root
	if cwd, err := os.Getwd(); err == nil {
		info.ProjectRoot = cwd
	}

	return info
}

// NewSystemInfoWithScreen collects all system information including actual terminal
// dimensions from a running screen. This requires an initialized tcell.Screen.
func NewSystemInfoWithScreen(screen tcell.Screen) *SystemInfo {
	info := NewSystemInfo()

	// Get actual terminal dimensions
	info.TerminalWidth, info.TerminalHeight = screen.Size()

	// Get verified color support from running screen
	info.ColorSupport, info.ColorCount = getColorSupportFromScreen(screen)

	return info
}

// ToMap returns system information as a map for serialization.
func (s *SystemInfo) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"os":              s.OS,
		"architecture":    s.Architecture,
		"os_version":      s.OSVersion,
		"term_type":       s.TermType,
		"colorterm":       s.ColorTerm,
		"colorfgbg":       s.ColorFGBG,
		"detected_theme":  s.DetectedTheme,
		"terminal_width":  s.TerminalWidth,
		"terminal_height": s.TerminalHeight,
		"color_support":   s.ColorSupport,
		"color_count":     s.ColorCount,
		"xdg_config_home": s.XDGConfigHome,
		"xdg_cache_home":  s.XDGCacheHome,
		"shell":           s.Shell,
		"editor":          s.Editor,
		"config_dir":      s.ConfigDir,
		"cache_dir":       s.CacheDir,
		"project_root":    s.ProjectRoot,
	}
}

// String returns a human-readable string representation of system information.
func (s *SystemInfo) String() string {
	var b strings.Builder

	b.WriteString("System Information\n")
	b.WriteString("==================\n")
	fmt.Fprintf(&b, "OS:           %s\n", s.OS)
	fmt.Fprintf(&b, "Architecture: %s\n", s.Architecture)
	fmt.Fprintf(&b, "OS Version:   %s\n", s.OSVersion)
	b.WriteString("\n")

	b.WriteString("Terminal\n")
	b.WriteString("--------\n")
	fmt.Fprintf(&b, "Type:         %s\n", s.TermType)
	fmt.Fprintf(&b, "COLORTERM:    %s\n", s.ColorTerm)
	fmt.Fprintf(&b, "COLORFGBG:    %s\n", s.ColorFGBG)
	fmt.Fprintf(&b, "Theme:        %s\n", s.DetectedTheme)
	fmt.Fprintf(&b, "Color Support: %s (%d colors)\n", s.ColorSupport, s.ColorCount)
	if s.TerminalWidth > 0 && s.TerminalHeight > 0 {
		fmt.Fprintf(&b, "Dimensions:   %dx%d\n", s.TerminalWidth, s.TerminalHeight)
	} else {
		b.WriteString("Dimensions:   N/A (not available without running app)\n")
	}
	b.WriteString("\n")

	b.WriteString("Environment\n")
	b.WriteString("-----------\n")
	fmt.Fprintf(&b, "Shell:        %s\n", s.Shell)
	fmt.Fprintf(&b, "Editor:       %s\n", s.Editor)
	fmt.Fprintf(&b, "XDG Config:   %s\n", s.XDGConfigHome)
	fmt.Fprintf(&b, "XDG Cache:    %s\n", s.XDGCacheHome)
	b.WriteString("\n")

	b.WriteString("Paths\n")
	b.WriteString("-----\n")
	fmt.Fprintf(&b, "Config Dir:   %s\n", s.ConfigDir)
	fmt.Fprintf(&b, "Cache Dir:    %s\n", s.CacheDir)
	fmt.Fprintf(&b, "Project Root: %s\n", s.ProjectRoot)

	return b.String()
}

// detectTheme parses $COLORFGBG to determine if terminal has dark or light background.
// Format: "fg;bg" where bg >= 8 indicates light background.
// Returns "dark", "light", or "unknown".
//
// Note: This is based on config.GetEffectiveTheme() but fixes a bug where string
// comparison fails for multi-digit values (e.g., "15" < "8" in string comparison).
func detectTheme(colorFGBG string) string {
	if colorFGBG == "" {
		return "unknown"
	}

	parts := strings.Split(colorFGBG, ";")
	if len(parts) >= 2 {
		bgStr := parts[len(parts)-1]

		// Parse as integer for proper comparison
		// This fixes the bug in config.GetEffectiveTheme() where "15" >= "8" fails
		var bgValue int
		if _, err := fmt.Sscanf(bgStr, "%d", &bgValue); err != nil {
			return "unknown"
		}

		// 0-7 = dark colors, 8+ = light colors
		if bgValue >= 8 {
			return "light"
		}
		return "dark"
	}

	return "unknown"
}

// getColorSupportFromTerminfo queries terminfo database for color capabilities
// without initializing a screen. This is fast (~1-5ms) and suitable for early
// bootstrap decisions.
//
// Checks $COLORTERM first (modern terminals use this to advertise truecolor support),
// then falls back to terminfo database via $TERM.
//
// Note: $TERM is not 100% reliable in edge cases (SSH forwarding, tmux/screen
// misconfiguration), but this is the same method tcell uses internally and is
// accurate for ~95% of users.
func getColorSupportFromTerminfo() (string, int) {
	// Check $COLORTERM first - modern terminals set this to indicate truecolor support
	// even when $TERM is xterm-256color
	if colorterm := os.Getenv("COLORTERM"); colorterm != "" {
		// Common values: "truecolor", "24bit"
		if colorterm == "truecolor" || colorterm == "24bit" {
			return "truecolor", 16777216
		}
	}

	term := os.Getenv("TERM")
	if term == "" {
		return "unknown", 0
	}

	ti, err := tcell.LookupTerminfo(term)
	if err != nil || ti == nil {
		return "unknown", 0
	}

	colors := ti.Colors

	switch {
	case colors >= 16777216:
		return "truecolor", colors
	case colors >= 256:
		return "256-color", colors
	case colors >= 16:
		return "16-color", colors
	case colors >= 2:
		return "monochrome", colors
	default:
		return "unknown", colors
	}
}

// getColorSupportFromScreen queries color capabilities from a running tcell screen.
// This is accurate but requires an initialized screen.
func getColorSupportFromScreen(screen tcell.Screen) (string, int) {
	colors := screen.Colors()

	switch {
	case colors >= 16777216:
		return "truecolor", colors
	case colors >= 256:
		return "256-color", colors
	case colors >= 16:
		return "16-color", colors
	case colors >= 2:
		return "monochrome", colors
	default:
		return "unknown", colors
	}
}

// getOSVersion attempts to detect OS version using platform-specific commands.
// Returns "unknown" if detection fails (graceful fallback).
func getOSVersion() string {
	switch runtime.GOOS {
	case "darwin":
		return getMacOSVersion()
	case "linux":
		return getLinuxVersion()
	case "windows":
		return getWindowsVersion()
	default:
		return "unknown"
	}
}

// getMacOSVersion runs sw_vers -productVersion to get macOS version.
func getMacOSVersion() string {
	cmd := exec.Command("sw_vers", "-productVersion")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// getLinuxVersion tries to parse /etc/os-release or runs lsb_release.
func getLinuxVersion() string {
	// Try /etc/os-release first (most modern distros)
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				// Remove PRETTY_NAME=" and trailing "
				version := strings.TrimPrefix(line, "PRETTY_NAME=")
				version = strings.Trim(version, "\"")
				return version
			}
		}
	}

	// Fallback to lsb_release
	cmd := exec.Command("lsb_release", "-d")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	// Parse "Description:\tUbuntu 22.04 LTS"
	line := strings.TrimSpace(string(output))
	if parts := strings.SplitN(line, "\t", 2); len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}

	return strings.TrimSpace(line)
}

// getWindowsVersion runs ver command to get Windows version.
func getWindowsVersion() string {
	cmd := exec.Command("cmd", "/c", "ver")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// getConfigDirSafe safely retrieves the config directory without panicking.
// Returns empty string if config.InitPaths() hasn't been called.
func getConfigDirSafe() string {
	defer func() {
		// recover from panic if PathManager not initialized
		_ = recover()
	}()
	return config.GetConfigDir()
}

// getCacheDirSafe safely retrieves the cache directory without panicking.
// Returns empty string if config.InitPaths() hasn't been called.
func getCacheDirSafe() string {
	defer func() {
		// recover from panic if PathManager not initialized
		_ = recover()
	}()
	return config.GetCacheDir()
}
