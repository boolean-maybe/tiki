package task

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/config"
)

// ValidateTitle returns an error message if the task title is invalid.
func ValidateTitle(t *Task) string {
	title := strings.TrimSpace(t.Title)
	if title == "" {
		return "title is required"
	}
	const maxTitleLength = 200
	if len(title) > maxTitleLength {
		return fmt.Sprintf("title exceeds maximum length of %d characters", maxTitleLength)
	}
	return ""
}

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
