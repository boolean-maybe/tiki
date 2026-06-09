package config

// Theme registry: maps theme names to dark/light classification, chroma syntax
// theme, and navidown markdown renderer style. Palette construction now lives
// in the theme package; this file retains only the surrounding metadata that
// non-color subsystems (markdown rendering, syntax highlighting) consult.

import (
	"log/slog"
)

// ThemeInfo holds metadata for a named theme.
type ThemeInfo struct {
	Light         bool   // true = light base, false = dark base
	ChromaTheme   string // chroma syntax theme for code blocks
	NavidownStyle string // navidown markdown renderer style name
}

// themeRegistry maps theme names to their ThemeInfo.
// "dark" and "light" are the built-in base themes; named themes extend this.
var themeRegistry = map[string]ThemeInfo{
	// built-in base themes
	"dark":  {Light: false, ChromaTheme: "nord", NavidownStyle: "dark"},
	"light": {Light: true, ChromaTheme: "github", NavidownStyle: "light"},

	// named dark themes
	"dracula":          {Light: false, ChromaTheme: "dracula", NavidownStyle: "dracula"},
	"tokyo-night":      {Light: false, ChromaTheme: "tokyonight-night", NavidownStyle: "tokyo-night"},
	"gruvbox-dark":     {Light: false, ChromaTheme: "gruvbox", NavidownStyle: "gruvbox-dark"},
	"catppuccin-mocha": {Light: false, ChromaTheme: "catppuccin-mocha", NavidownStyle: "catppuccin-mocha"},
	"solarized-dark":   {Light: false, ChromaTheme: "solarized-dark256", NavidownStyle: "solarized-dark"},
	"nord":             {Light: false, ChromaTheme: "nord", NavidownStyle: "nord"},
	"monokai":          {Light: false, ChromaTheme: "monokai", NavidownStyle: "monokai"},
	"one-dark":         {Light: false, ChromaTheme: "onedark", NavidownStyle: "one-dark"},

	// named light themes
	"catppuccin-latte": {Light: true, ChromaTheme: "catppuccin-latte", NavidownStyle: "catppuccin-latte"},
	"solarized-light":  {Light: true, ChromaTheme: "solarized-light", NavidownStyle: "solarized-light"},
	"gruvbox-light":    {Light: true, ChromaTheme: "gruvbox-light", NavidownStyle: "gruvbox-light"},
	"github-light":     {Light: true, ChromaTheme: "github", NavidownStyle: "github-light"},
}

var defaultTheme = themeRegistry["dark"]

// lookupTheme returns the ThemeInfo for the effective theme.
// Logs a warning and returns the dark theme for unrecognized names.
func lookupTheme() ThemeInfo {
	name := GetEffectiveTheme()
	if info, ok := themeRegistry[name]; ok {
		return info
	}
	slog.Warn("unknown theme, falling back to dark", "theme", name)
	return defaultTheme
}

// IsLightTheme returns true if the effective theme has a light background.
func IsLightTheme() bool {
	return lookupTheme().Light
}

// GetNavidownStyle returns the navidown markdown renderer style for the effective theme.
func GetNavidownStyle() string {
	return lookupTheme().NavidownStyle
}

// ChromaThemeForEffective returns the chroma syntax theme name for the effective theme.
func ChromaThemeForEffective() string {
	return lookupTheme().ChromaTheme
}
