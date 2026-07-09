package value

import (
	"testing"
	"time"
)

func TestParseDateTime(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantOK  bool
		wantStr string // FormatDateTime of the result, "" for zero
	}{
		{"valid", "2026-07-08 14:30", true, "2026-07-08 14:30"},
		{"empty", "", true, ""},
		{"whitespace", "   ", true, ""},
		{"date only is invalid", "2026-07-08", false, ""},
		{"garbage", "not-a-date", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ParseDateTime(c.in)
			if ok != c.wantOK {
				t.Fatalf("ParseDateTime(%q) ok = %v, want %v", c.in, ok, c.wantOK)
			}
			if FormatDateTime(got) != c.wantStr {
				t.Errorf("FormatDateTime(ParseDateTime(%q)) = %q, want %q",
					c.in, FormatDateTime(got), c.wantStr)
			}
		})
	}
}

func TestFormatDateTime(t *testing.T) {
	if got := FormatDateTime(time.Time{}); got != "" {
		t.Errorf("FormatDateTime(zero) = %q, want empty", got)
	}
	tm := time.Date(2026, 2, 28, 9, 5, 0, 0, time.UTC)
	if got := FormatDateTime(tm); got != "2026-02-28 09:05" {
		t.Errorf("FormatDateTime = %q, want 2026-02-28 09:05", got)
	}
}
