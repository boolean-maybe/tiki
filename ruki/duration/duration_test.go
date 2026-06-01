package duration

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input    string
		wantVal  int
		wantUnit string
	}{
		{"1sec", 1, "sec"},
		{"10secs", 10, "sec"},
		{"1min", 1, "min"},
		{"30mins", 30, "min"},
		{"1hour", 1, "hour"},
		{"2hours", 2, "hour"},
		{"1day", 1, "day"},
		{"7days", 7, "day"},
		{"1week", 1, "week"},
		{"3weeks", 3, "week"},
		{"1month", 1, "month"},
		{"6months", 6, "month"},
		{"1year", 1, "year"},
		{"2years", 2, "year"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			val, unit, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if val != tt.wantVal {
				t.Errorf("value = %d, want %d", val, tt.wantVal)
			}
			if unit != tt.wantUnit {
				t.Errorf("unit = %q, want %q", unit, tt.wantUnit)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"", "empty string"},
		{"min", "no digits"},
		{"30", "no unit"},
		{"30bogus", "unknown unit"},
		{"0xAhour", "non-decimal digits"},
		{"99999999999999999999sec", "integer overflow"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, _, err := Parse(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestToDuration(t *testing.T) {
	tests := []struct {
		unit string
		want time.Duration
	}{
		{"sec", time.Second},
		{"min", time.Minute},
		{"hour", time.Hour},
		{"day", 24 * time.Hour},
		{"week", 7 * 24 * time.Hour},
		{"month", 30 * 24 * time.Hour},
		{"year", 365 * 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.unit, func(t *testing.T) {
			got, err := ToDuration(1, tt.unit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ToDuration(1, %q) = %v, want %v", tt.unit, got, tt.want)
			}
		})
	}
}

func TestToDurationMultiplier(t *testing.T) {
	tests := []struct {
		value int
		unit  string
		want  time.Duration
	}{
		{3, "sec", 3 * time.Second},
		{5, "min", 5 * time.Minute},
		{2, "hour", 2 * time.Hour},
		{5, "day", 5 * 24 * time.Hour},
		{3, "week", 3 * 7 * 24 * time.Hour},
		{2, "month", 2 * 30 * 24 * time.Hour},
		{4, "year", 4 * 365 * 24 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d_%s", tt.value, tt.unit), func(t *testing.T) {
			got, err := ToDuration(tt.value, tt.unit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ToDuration(%d, %q) = %v, want %v", tt.value, tt.unit, got, tt.want)
			}
		})
	}
}

func TestToDurationUnknownUnit(t *testing.T) {
	_, err := ToDuration(1, "unknown")
	if err == nil {
		t.Fatal("expected error for unknown unit")
	}
}

func TestIsValidUnit(t *testing.T) {
	for _, u := range units {
		if !IsValidUnit(u.name) {
			t.Errorf("IsValidUnit(%q) = false, want true", u.name)
		}
	}
	if IsValidUnit("minute") {
		t.Error("IsValidUnit(\"minute\") = true, want false")
	}
	if IsValidUnit("bogus") {
		t.Error("IsValidUnit(\"bogus\") = true, want false")
	}
}

func TestPatternContainsAllUnits(t *testing.T) {
	p := Pattern()
	for _, u := range units {
		if !strings.Contains(p, u.name) {
			t.Errorf("Pattern() missing unit %q", u.name)
		}
	}
}

func TestPatternRegexMatchesAllUnits(t *testing.T) {
	re := regexp.MustCompile(`^\d+(?:` + Pattern() + `)s?$`)
	for _, u := range units {
		singular := "1" + u.name
		if !re.MatchString(singular) {
			t.Errorf("pattern does not match %q", singular)
		}
		plural := "1" + u.name + "s"
		if !re.MatchString(plural) {
			t.Errorf("pattern does not match %q", plural)
		}
	}
}

func TestPatternLongestFirst(t *testing.T) {
	p := Pattern()
	// "month" must appear before "min" so regex matches greedily
	monthIdx := strings.Index(p, "month")
	minIdx := strings.Index(p, "min")
	if monthIdx > minIdx {
		t.Errorf("Pattern() has 'min' before 'month': %s", p)
	}
}
