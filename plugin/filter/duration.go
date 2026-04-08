package filter

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/util/duration"
)

// durationPattern matches a number followed by a duration unit, with optional plural "s".
var durationPattern = regexp.MustCompile(`^(\d+)(` + duration.Pattern() + `)s?$`)

// IsDurationLiteral checks if a string is a valid duration literal.
func IsDurationLiteral(s string) bool {
	return durationPattern.MatchString(strings.ToLower(s))
}

// ParseDuration parses a duration literal like "24hour" or "1week".
func ParseDuration(s string) (time.Duration, error) {
	val, unit, err := duration.Parse(strings.ToLower(s))
	if err != nil {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}
	return duration.ToDuration(val, unit)
}
