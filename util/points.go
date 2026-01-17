package util

// GeneratePointsVisual formats points as a visual representation using filled/unfilled circles.
// Points are scaled to a 0-10 display range based on maxPoints configuration.
//
// Parameters:
//   - points: The task's point value
//   - maxPoints: The configured maximum points value (for scaling)
//
// Returns: A string with filled (●) and unfilled (◦) circles representing the points value.
//
// Example:
//
//	GeneratePointsVisual(5, 10) returns "●●●●●◦◦◦◦◦" (5 filled, 5 unfilled)
func GeneratePointsVisual(points int, maxPoints int) string {
	const displayCircles = 10
	const filled = "●"
	const unfilled = "◦"

	// Scale points to 0-10 range based on configured max
	// Formula: displayPoints = (points * displayCircles) / maxPoints
	displayPoints := (points * displayCircles) / maxPoints

	// Clamp to 0-10 range
	if displayPoints < 0 {
		displayPoints = 0
	}
	if displayPoints > displayCircles {
		displayPoints = displayCircles
	}

	result := ""
	for i := 0; i < displayPoints; i++ {
		result += filled
	}
	for i := displayPoints; i < displayCircles; i++ {
		result += unfilled
	}

	return result
}
