package ruki

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/util/duration"
)

const dateFormat = "2006-01-02"

// ParseDateString parses a YYYY-MM-DD date string into a time.Time.
// Shared by DSL literal parsing (lower.go) and user input parsing.
func ParseDateString(s string) (time.Time, error) {
	return time.Parse(dateFormat, s)
}

// ParseDurationString parses a duration string like "2day" into its (value, unit) components.
// Shared by DSL literal parsing (lower.go) and user input parsing.
func ParseDurationString(s string) (int, string, error) {
	return duration.Parse(s)
}

// ParseScalarTypeName maps a canonical scalar type name to a ValueType.
// Only the 6 user-inputtable scalar types are accepted.
func ParseScalarTypeName(name string) (ValueType, error) {
	switch strings.ToLower(name) {
	case "string":
		return ValueString, nil
	case "int":
		return ValueInt, nil
	case "bool":
		return ValueBool, nil
	case "date":
		return ValueDate, nil
	case "timestamp":
		return ValueTimestamp, nil
	case "duration":
		return ValueDuration, nil
	default:
		return 0, fmt.Errorf("unsupported input type %q (supported: string, int, bool, date, timestamp, duration)", name)
	}
}

// ParseScalarValue parses user-supplied text into a native runtime value
// matching what the ruki executor produces for the given type.
func ParseScalarValue(typ ValueType, text string) (interface{}, error) {
	switch typ {
	case ValueString:
		return text, nil

	case ValueInt:
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("expected integer, got %q", text)
		}
		return n, nil

	case ValueBool:
		b, err := parseBoolString(strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("expected true or false, got %q", text)
		}
		return b, nil

	case ValueDate:
		t, err := ParseDateString(strings.TrimSpace(text))
		if err != nil {
			return nil, fmt.Errorf("expected date (YYYY-MM-DD), got %q", text)
		}
		return t, nil

	case ValueTimestamp:
		trimmed := strings.TrimSpace(text)
		t, err := time.Parse(time.RFC3339, trimmed)
		if err == nil {
			return t, nil
		}
		t, err = ParseDateString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("expected timestamp (RFC3339 or YYYY-MM-DD), got %q", text)
		}
		return t, nil

	case ValueDuration:
		trimmed := strings.TrimSpace(text)
		val, unit, err := ParseDurationString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("expected duration (e.g. 2day, 1week), got %q", text)
		}
		d, err := duration.ToDuration(val, unit)
		if err != nil {
			return nil, err
		}
		return d, nil

	default:
		return nil, fmt.Errorf("type %s is not a supported input scalar", typeName(typ))
	}
}
