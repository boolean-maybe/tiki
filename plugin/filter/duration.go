package filter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Duration pattern: number followed by unit (month, week, day, hour, min)
var durationPattern = regexp.MustCompile(`^(\d+)(month|week|day|hour|min)s?$`)

// IsDurationLiteral checks if a string is a valid duration literal
func IsDurationLiteral(s string) bool {
	return durationPattern.MatchString(strings.ToLower(s))
}

// ParseDuration parses a duration literal like "24hour" or "1week"
func ParseDuration(s string) (time.Duration, error) {
	s = strings.ToLower(s)
	matches := durationPattern.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "min":
		return time.Duration(value) * time.Minute, nil
	case "hour":
		return time.Duration(value) * time.Hour, nil
	case "day":
		return time.Duration(value) * 24 * time.Hour, nil
	case "week":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case "month":
		return time.Duration(value) * 30 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("unknown duration unit: %s", unit)
}
