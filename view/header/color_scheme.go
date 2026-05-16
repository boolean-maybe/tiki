package header

import "github.com/boolean-maybe/tiki/theme"

// ColorScheme defines color pairs for different action categories
type ColorScheme struct {
	KeyColor   theme.Role
	LabelColor theme.Role
}

// getColorScheme returns the color scheme for the given action type.
// Colors are retrieved from the centralized theme.Roles().
// Falls back to global color scheme if the type is not found.
func getColorScheme(colorType int) ColorScheme {
	roles := theme.Roles()

	switch colorType {
	case colorTypeGlobal:
		return ColorScheme{
			KeyColor:   roles.Highlight(),
			LabelColor: roles.TextPrimary(),
		}
	case colorTypePlugin:
		return ColorScheme{
			KeyColor:   roles.StatusWarn(),
			LabelColor: roles.TextSecondary(),
		}
	case colorTypeView:
		return ColorScheme{
			KeyColor:   roles.AccentAction(),
			LabelColor: roles.TextMuted(),
		}
	default:
		// fallback to global colors
		return ColorScheme{
			KeyColor:   roles.Highlight(),
			LabelColor: roles.TextPrimary(),
		}
	}
}
