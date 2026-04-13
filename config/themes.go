package config

// Theme registry: maps theme names to palette constructors, dark/light classification,
// chroma syntax theme, and navidown markdown renderer style.

import (
	"log/slog"
	"sort"
)

// ThemeInfo holds all metadata for a named theme.
type ThemeInfo struct {
	Light         bool           // true = light base, false = dark base
	ChromaTheme   string         // chroma syntax theme for code blocks
	NavidownStyle string         // navidown markdown renderer style name
	Palette       func() Palette // palette constructor
}

// themeRegistry maps theme names to their ThemeInfo.
// "dark" and "light" are the built-in base themes; named themes extend this.
var themeRegistry = map[string]ThemeInfo{
	// built-in base themes
	"dark":  {Light: false, ChromaTheme: "nord", NavidownStyle: "dark", Palette: DarkPalette},
	"light": {Light: true, ChromaTheme: "github", NavidownStyle: "light", Palette: LightPalette},

	// named dark themes
	"dracula":          {Light: false, ChromaTheme: "dracula", NavidownStyle: "dracula", Palette: DraculaPalette},
	"tokyo-night":      {Light: false, ChromaTheme: "tokyonight-night", NavidownStyle: "tokyo-night", Palette: TokyoNightPalette},
	"gruvbox-dark":     {Light: false, ChromaTheme: "gruvbox", NavidownStyle: "gruvbox-dark", Palette: GruvboxDarkPalette},
	"catppuccin-mocha": {Light: false, ChromaTheme: "catppuccin-mocha", NavidownStyle: "catppuccin-mocha", Palette: CatppuccinMochaPalette},
	"solarized-dark":   {Light: false, ChromaTheme: "solarized-dark256", NavidownStyle: "solarized-dark", Palette: SolarizedDarkPalette},
	"nord":             {Light: false, ChromaTheme: "nord", NavidownStyle: "nord", Palette: NordPalette},
	"monokai":          {Light: false, ChromaTheme: "monokai", NavidownStyle: "monokai", Palette: MonokaiPalette},
	"one-dark":         {Light: false, ChromaTheme: "onedark", NavidownStyle: "one-dark", Palette: OneDarkPalette},

	// named light themes
	"catppuccin-latte": {Light: true, ChromaTheme: "catppuccin-latte", NavidownStyle: "catppuccin-latte", Palette: CatppuccinLattePalette},
	"solarized-light":  {Light: true, ChromaTheme: "solarized-light", NavidownStyle: "solarized-light", Palette: SolarizedLightPalette},
	"gruvbox-light":    {Light: true, ChromaTheme: "gruvbox-light", NavidownStyle: "gruvbox-light", Palette: GruvboxLightPalette},
	"github-light":     {Light: true, ChromaTheme: "github", NavidownStyle: "github-light", Palette: GithubLightPalette},
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

// PaletteForTheme returns the Palette for the effective theme.
func PaletteForTheme() Palette {
	return lookupTheme().Palette()
}

// ChromaThemeForEffective returns the chroma syntax theme name for the effective theme.
func ChromaThemeForEffective() string {
	return lookupTheme().ChromaTheme
}

// ThemeNames returns a sorted list of all registered theme names.
func ThemeNames() []string {
	names := make([]string, 0, len(themeRegistry))
	for name := range themeRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
