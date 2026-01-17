package header

// ColorScheme defines color pairs for different action categories
type ColorScheme struct {
	KeyColor   string
	LabelColor string
}

// actionColorSchemes maps action types to their display colors.
// These colors are used in the context help widget to differentiate
// between global actions, plugin actions, and view-specific actions.
var actionColorSchemes = map[int]ColorScheme{
	colorTypeGlobal: {
		KeyColor:   "#ffff00", // yellow for global actions
		LabelColor: "#ffffff", // white for global action labels
	},
	colorTypePlugin: {
		KeyColor:   "#ff8c00", // orange for plugin actions
		LabelColor: "#b0b0b0", // light gray for plugin labels
	},
	colorTypeView: {
		KeyColor:   "#5fafff", // cyan for view-specific actions
		LabelColor: "#808080", // gray for view-specific labels
	},
}

// getColorScheme returns the color scheme for the given action type.
// Falls back to global color scheme if the type is not found.
func getColorScheme(colorType int) ColorScheme {
	if scheme, ok := actionColorSchemes[colorType]; ok {
		return scheme
	}
	return actionColorSchemes[colorTypeGlobal] // default fallback
}
