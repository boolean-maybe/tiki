package header

import "github.com/boolean-maybe/tiki/config"

// ColorScheme defines color pairs for different action categories
type ColorScheme struct {
	KeyColor   string
	LabelColor string
}

// getColorScheme returns the color scheme for the given action type.
// Colors are retrieved from the centralized config.GetColors().
// Falls back to global color scheme if the type is not found.
func getColorScheme(colorType int) ColorScheme {
	colors := config.GetColors()

	switch colorType {
	case colorTypeGlobal:
		return ColorScheme{
			KeyColor:   colors.HeaderActionGlobalKeyColor,
			LabelColor: colors.HeaderActionGlobalLabelColor,
		}
	case colorTypePlugin:
		return ColorScheme{
			KeyColor:   colors.HeaderActionPluginKeyColor,
			LabelColor: colors.HeaderActionPluginLabelColor,
		}
	case colorTypeView:
		return ColorScheme{
			KeyColor:   colors.HeaderActionViewKeyColor,
			LabelColor: colors.HeaderActionViewLabelColor,
		}
	default:
		// Fallback to global colors
		return ColorScheme{
			KeyColor:   colors.HeaderActionGlobalKeyColor,
			LabelColor: colors.HeaderActionGlobalLabelColor,
		}
	}
}
