package duration

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// unit pairs a canonical short name with its time.Duration equivalent.
type unit struct {
	name     string
	duration time.Duration
}

// units is the canonical, immutable list of supported duration units.
var units = [...]unit{
	{"sec", time.Second},
	{"min", time.Minute},
	{"hour", time.Hour},
	{"day", 24 * time.Hour},
	{"week", 7 * 24 * time.Hour},
	{"month", 30 * 24 * time.Hour},
	{"year", 365 * 24 * time.Hour},
}

var unitMap map[string]time.Duration

func init() {
	unitMap = make(map[string]time.Duration, len(units))
	for _, u := range units {
		unitMap[u.name] = u.duration
	}
}

// Parse splits a duration string like "30min" or "2days" into its numeric
// value and canonical unit name. Plural trailing "s" is stripped.
func Parse(s string) (int, string, error) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i == len(s) {
		return 0, "", fmt.Errorf("invalid duration literal %q", s)
	}

	val, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0, "", fmt.Errorf("invalid duration value in %q: %w", s, err)
	}

	unit := strings.TrimSuffix(s[i:], "s")
	if !IsValidUnit(unit) {
		return 0, "", fmt.Errorf("unknown duration unit %q in %q", unit, s)
	}
	return val, unit, nil
}

// ToDuration converts a value and canonical unit name to a time.Duration.
func ToDuration(value int, unit string) (time.Duration, error) {
	d, ok := unitMap[unit]
	if !ok {
		return 0, fmt.Errorf("unknown duration unit %q", unit)
	}
	return time.Duration(value) * d, nil
}

// IsValidUnit reports whether name is a recognized duration unit.
func IsValidUnit(name string) bool {
	_, ok := unitMap[name]
	return ok
}

// Pattern returns a regex alternation of all unit names, sorted longest-first
// so that greedy matching prefers "month" over "min".
func Pattern() string {
	names := make([]string, len(units))
	for i, u := range units {
		names[i] = u.name
	}
	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) > len(names[j])
	})
	return strings.Join(names, "|")
}
