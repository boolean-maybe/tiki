package value

import "github.com/boolean-maybe/tiki/config"

// IsValidPoints checks if a points value is within the valid range.
// Points == 0 is the explicit "unestimated" sentinel and is always valid;
// only positive values out of [1, maxPoints] fail validation.
func IsValidPoints(points int) bool {
	if points == 0 {
		return true
	}
	if points < 0 {
		return false
	}
	return points <= config.GetMaxPoints()
}
