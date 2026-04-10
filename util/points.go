package util

import (
	"strings"

	"github.com/boolean-maybe/tiki/config"
)

// GeneratePointsVisual formats points as a visual representation using a styled bar.
// Points are scaled to a 0-10 display range based on maxPoints configuration.
//
// Parameters:
//   - points: The task's point value
//   - maxPoints: The configured maximum points value (for scaling)
//   - filledColor: Color for filled segments
//   - unfilledColor: Color for unfilled segments
//
// Returns: A string with colored filled (❚) and unfilled (❘) segments representing the points value.
//
//	Uses tview color tag format for proper rendering in the TUI.
//
// Example:
//
//	GeneratePointsVisual(7, 10, filledColor, unfilledColor) returns a bar with 7 blue segments and 3 gray segments
func GeneratePointsVisual(points int, maxPoints int, filledColor config.Color, unfilledColor config.Color) string {
	const displaySegments = 10
	const filledChar = "❚"
	const unfilledChar = "❘"
	const resetColor = "[-]" // Reset to default in tview format

	// Scale points to 0-10 range based on configured max
	// Formula: displayPoints = (points * displaySegments) / maxPoints
	displayPoints := (points * displaySegments) / maxPoints

	// Clamp to 0-10 range
	displayPoints = max(0, min(displayPoints, displaySegments))

	var result strings.Builder
	result.Grow(50) // Pre-allocate for color tags + characters

	// Add filled segments
	if displayPoints > 0 {
		result.WriteString(filledColor.Tag().String())
		for i := 0; i < displayPoints; i++ {
			result.WriteString(filledChar)
		}
	}

	// Add unfilled segments
	if displayPoints < displaySegments {
		result.WriteString(unfilledColor.Tag().String())
		for i := displayPoints; i < displaySegments; i++ {
			result.WriteString(unfilledChar)
		}
	}

	// Reset color at the end
	result.WriteString(resetColor)

	return result.String()
}
