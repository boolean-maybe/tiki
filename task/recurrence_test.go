package task

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRecurrenceValue_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		expectEmpty bool
		expectValue Recurrence
	}{
		{
			name:        "empty (omitted)",
			yaml:        "other: value",
			expectEmpty: true,
		},
		{
			name:        "empty string",
			yaml:        "recurrence: ''",
			expectEmpty: true,
		},
		{
			name:        "valid daily cron",
			yaml:        "recurrence: '0 0 * * *'",
			expectEmpty: false,
			expectValue: RecurrenceDaily,
		},
		{
			name:        "valid weekly monday",
			yaml:        "recurrence: '0 0 * * MON'",
			expectEmpty: false,
			expectValue: "0 0 * * MON",
		},
		{
			name:        "valid monthly",
			yaml:        "recurrence: '0 0 1 * *'",
			expectEmpty: false,
			expectValue: RecurrenceMonthly,
		},
		{
			name:        "case insensitive",
			yaml:        "recurrence: '0 0 * * mon'",
			expectEmpty: false,
			expectValue: "0 0 * * MON",
		},
		{
			name:        "invalid cron defaults to empty",
			yaml:        "recurrence: '*/5 * * * *'",
			expectEmpty: true,
		},
		{
			name:        "random string defaults to empty",
			yaml:        "recurrence: 'every tuesday'",
			expectEmpty: true,
		},
		{
			name:        "number defaults to empty",
			yaml:        "recurrence: 42",
			expectEmpty: true,
		},
		{
			name:        "boolean defaults to empty",
			yaml:        "recurrence: true",
			expectEmpty: true,
		},
		{
			name:        "null value",
			yaml:        "recurrence: null",
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				Recurrence RecurrenceValue `yaml:"recurrence,omitempty"`
			}

			var result testStruct
			err := yaml.Unmarshal([]byte(tt.yaml), &result)
			if err != nil {
				t.Fatalf("UnmarshalYAML() error = %v", err)
			}

			if tt.expectEmpty {
				if result.Recurrence.Value != RecurrenceNone {
					t.Errorf("expected empty recurrence, got %q", result.Recurrence.Value)
				}
			} else {
				if result.Recurrence.Value != tt.expectValue {
					t.Errorf("got %q, expected %q", result.Recurrence.Value, tt.expectValue)
				}
			}
		})
	}
}

func TestRecurrenceValue_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		value    RecurrenceValue
		expected string
	}{
		{
			name:     "empty",
			value:    RecurrenceValue{},
			expected: "recurrence: \"\"\n",
		},
		{
			name:     "daily",
			value:    RecurrenceValue{Value: RecurrenceDaily},
			expected: "recurrence: 0 0 * * *\n",
		},
		{
			name:     "weekly monday",
			value:    RecurrenceValue{Value: "0 0 * * MON"},
			expected: "recurrence: 0 0 * * MON\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				Recurrence RecurrenceValue `yaml:"recurrence"`
			}

			got, err := yaml.Marshal(testStruct{Recurrence: tt.value})
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}

			if string(got) != tt.expected {
				t.Errorf("got %q, expected %q", string(got), tt.expected)
			}
		})
	}
}

func TestRecurrenceValue_RoundTrip(t *testing.T) {
	values := []Recurrence{
		RecurrenceNone,
		RecurrenceDaily,
		"0 0 * * MON",
		"0 0 * * FRI",
		RecurrenceMonthly,
	}

	for _, v := range values {
		t.Run(string(v), func(t *testing.T) {
			type testStruct struct {
				Recurrence RecurrenceValue `yaml:"recurrence"`
			}

			input := testStruct{Recurrence: RecurrenceValue{Value: v}}
			data, err := yaml.Marshal(input)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			var output testStruct
			if err := yaml.Unmarshal(data, &output); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if output.Recurrence.Value != v {
				t.Errorf("round-trip failed: got %q, expected %q", output.Recurrence.Value, v)
			}
		})
	}
}

func TestRecurrenceValue_OmitEmpty(t *testing.T) {
	type testStruct struct {
		Recurrence RecurrenceValue `yaml:"recurrence,omitempty"`
	}

	t.Run("empty omitted", func(t *testing.T) {
		got, err := yaml.Marshal(testStruct{})
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if string(got) != "{}\n" {
			t.Errorf("omitempty failed: got %q, expected %q", string(got), "{}\n")
		}
	})

	t.Run("non-empty included", func(t *testing.T) {
		got, err := yaml.Marshal(testStruct{Recurrence: RecurrenceValue{Value: RecurrenceDaily}})
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if string(got) == "{}\n" {
			t.Error("non-empty value should not be omitted")
		}
	})
}

func TestParseRecurrence(t *testing.T) {
	tests := []struct {
		input string
		want  Recurrence
		ok    bool
	}{
		{"0 0 * * *", RecurrenceDaily, true},
		{"0 0 * * MON", "0 0 * * MON", true},
		{"0 0 * * mon", "0 0 * * MON", true},
		{"0 0 1 * *", RecurrenceMonthly, true},
		{"0 0 * * SUN", "0 0 * * SUN", true},
		{"", RecurrenceNone, true},
		{"*/5 * * * *", RecurrenceNone, false},
		{"every day", RecurrenceNone, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseRecurrence(tt.input)
			if ok != tt.ok {
				t.Errorf("ParseRecurrence(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("ParseRecurrence(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRecurrenceDisplay(t *testing.T) {
	tests := []struct {
		input Recurrence
		want  string
	}{
		{RecurrenceNone, "None"},
		{RecurrenceDaily, "Daily"},
		{"0 0 * * MON", "Weekly on Monday"},
		{"0 0 * * TUE", "Weekly on Tuesday"},
		{"0 0 * * WED", "Weekly on Wednesday"},
		{"0 0 * * THU", "Weekly on Thursday"},
		{"0 0 * * FRI", "Weekly on Friday"},
		{"0 0 * * SAT", "Weekly on Saturday"},
		{"0 0 * * SUN", "Weekly on Sunday"},
		{RecurrenceMonthly, "Monthly on the 1st"},
		{"unknown", "None"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := RecurrenceDisplay(tt.input)
			if got != tt.want {
				t.Errorf("RecurrenceDisplay(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRecurrenceFromDisplay(t *testing.T) {
	tests := []struct {
		input string
		want  Recurrence
	}{
		{"None", RecurrenceNone},
		{"Daily", RecurrenceDaily},
		{"Weekly on Monday", "0 0 * * MON"},
		{"weekly on monday", "0 0 * * MON"},
		{"Monthly on the 1st", RecurrenceMonthly},
		{"monthly on the 1st", RecurrenceMonthly},
		{"unknown", RecurrenceNone},
		{"", RecurrenceNone},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := RecurrenceFromDisplay(tt.input)
			if got != tt.want {
				t.Errorf("RecurrenceFromDisplay(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAllRecurrenceDisplayValues(t *testing.T) {
	values := AllRecurrenceDisplayValues()
	if len(values) == 0 {
		t.Fatal("expected at least one display value")
	}
	if values[0] != "None" {
		t.Errorf("first value should be None, got %q", values[0])
	}
	if values[len(values)-1] != "Monthly on the 1st" {
		t.Errorf("last value should be 'Monthly on the 1st', got %q", values[len(values)-1])
	}
	// every display value must round-trip through RecurrenceFromDisplay → RecurrenceDisplay
	for _, v := range values {
		cron := RecurrenceFromDisplay(v)
		got := RecurrenceDisplay(cron)
		if got != v {
			t.Errorf("round-trip failed for %q: RecurrenceDisplay(RecurrenceFromDisplay(%q)) = %q", v, v, got)
		}
	}
}

func TestIsValidRecurrence(t *testing.T) {
	tests := []struct {
		input Recurrence
		want  bool
	}{
		{RecurrenceNone, true},
		{RecurrenceDaily, true},
		{"0 0 * * MON", true},
		{"0 0 * * TUE", true},
		{"0 0 * * WED", true},
		{"0 0 * * THU", true},
		{"0 0 * * FRI", true},
		{"0 0 * * SAT", true},
		{"0 0 * * SUN", true},
		{RecurrenceMonthly, true},
		{"*/5 * * * *", false},
		{"bogus", false},
		{"0 0 * * MON ", false}, // trailing space
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := IsValidRecurrence(tt.input)
			if got != tt.want {
				t.Errorf("IsValidRecurrence(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRecurrenceValue_ToRecurrence(t *testing.T) {
	t.Run("zero value returns RecurrenceNone", func(t *testing.T) {
		var rv RecurrenceValue
		if rv.ToRecurrence() != RecurrenceNone {
			t.Errorf("got %q, want %q", rv.ToRecurrence(), RecurrenceNone)
		}
	})
	t.Run("daily value returns RecurrenceDaily", func(t *testing.T) {
		rv := RecurrenceValue{Value: RecurrenceDaily}
		if rv.ToRecurrence() != RecurrenceDaily {
			t.Errorf("got %q, want %q", rv.ToRecurrence(), RecurrenceDaily)
		}
	})
}

func TestOrdinalSuffix(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "st"}, {2, "nd"}, {3, "rd"}, {4, "th"},
		{11, "th"}, {12, "th"}, {13, "th"},
		{21, "st"}, {22, "nd"}, {23, "rd"},
		{31, "st"},
	}
	for _, tt := range tests {
		got := OrdinalSuffix(tt.n)
		if got != tt.want {
			t.Errorf("OrdinalSuffix(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestMonthlyRecurrence(t *testing.T) {
	tests := []struct {
		day  int
		want Recurrence
	}{
		{1, "0 0 1 * *"},
		{15, "0 0 15 * *"},
		{31, "0 0 31 * *"},
		{0, RecurrenceNone},
		{32, RecurrenceNone},
	}
	for _, tt := range tests {
		got := MonthlyRecurrence(tt.day)
		if got != tt.want {
			t.Errorf("MonthlyRecurrence(%d) = %q, want %q", tt.day, got, tt.want)
		}
	}
}

func TestIsMonthlyRecurrence(t *testing.T) {
	tests := []struct {
		r       Recurrence
		wantDay int
		wantOk  bool
	}{
		{"0 0 1 * *", 1, true},
		{"0 0 15 * *", 15, true},
		{"0 0 31 * *", 31, true},
		{"0 0 * * *", 0, false},   // daily, not monthly
		{"0 0 * * MON", 0, false}, // weekly
		{"0 0 0 * *", 0, false},   // day 0 is invalid
		{"0 0 32 * *", 0, false},  // day 32 is invalid
	}
	for _, tt := range tests {
		day, ok := IsMonthlyRecurrence(tt.r)
		if ok != tt.wantOk || day != tt.wantDay {
			t.Errorf("IsMonthlyRecurrence(%q) = (%d, %v), want (%d, %v)", tt.r, day, ok, tt.wantDay, tt.wantOk)
		}
	}
}

func TestMonthlyDisplay(t *testing.T) {
	tests := []struct {
		day  int
		want string
	}{
		{1, "Monthly on the 1st"},
		{2, "Monthly on the 2nd"},
		{3, "Monthly on the 3rd"},
		{4, "Monthly on the 4th"},
		{15, "Monthly on the 15th"},
		{21, "Monthly on the 21st"},
		{31, "Monthly on the 31st"},
	}
	for _, tt := range tests {
		got := MonthlyDisplay(tt.day)
		if got != tt.want {
			t.Errorf("MonthlyDisplay(%d) = %q, want %q", tt.day, got, tt.want)
		}
	}
}

func TestFrequencyFromRecurrence(t *testing.T) {
	tests := []struct {
		r    Recurrence
		want RecurrenceFrequency
	}{
		{RecurrenceNone, FrequencyNone},
		{RecurrenceDaily, FrequencyDaily},
		{"0 0 * * MON", FrequencyWeekly},
		{"0 0 * * FRI", FrequencyWeekly},
		{RecurrenceMonthly, FrequencyMonthly},
		{"0 0 15 * *", FrequencyMonthly},
		{"bogus", FrequencyNone},
	}
	for _, tt := range tests {
		got := FrequencyFromRecurrence(tt.r)
		if got != tt.want {
			t.Errorf("FrequencyFromRecurrence(%q) = %q, want %q", tt.r, got, tt.want)
		}
	}
}

func TestWeekdayFromRecurrence(t *testing.T) {
	tests := []struct {
		r      Recurrence
		want   string
		wantOk bool
	}{
		{"0 0 * * MON", "Monday", true},
		{"0 0 * * SUN", "Sunday", true},
		{"0 0 * * *", "", false},
		{"0 0 1 * *", "", false},
	}
	for _, tt := range tests {
		got, ok := WeekdayFromRecurrence(tt.r)
		if ok != tt.wantOk || got != tt.want {
			t.Errorf("WeekdayFromRecurrence(%q) = (%q, %v), want (%q, %v)", tt.r, got, ok, tt.want, tt.wantOk)
		}
	}
}

func TestDayOfMonthFromRecurrence(t *testing.T) {
	tests := []struct {
		r      Recurrence
		want   int
		wantOk bool
	}{
		{"0 0 15 * *", 15, true},
		{"0 0 * * MON", 0, false},
	}
	for _, tt := range tests {
		got, ok := DayOfMonthFromRecurrence(tt.r)
		if ok != tt.wantOk || got != tt.want {
			t.Errorf("DayOfMonthFromRecurrence(%q) = (%d, %v), want (%d, %v)", tt.r, got, ok, tt.want, tt.wantOk)
		}
	}
}

func TestRecurrenceDisplay_Monthly(t *testing.T) {
	tests := []struct {
		input Recurrence
		want  string
	}{
		{"0 0 15 * *", "Monthly on the 15th"},
		{"0 0 2 * *", "Monthly on the 2nd"},
		{"0 0 31 * *", "Monthly on the 31st"},
	}
	for _, tt := range tests {
		got := RecurrenceDisplay(tt.input)
		if got != tt.want {
			t.Errorf("RecurrenceDisplay(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsValidRecurrence_Monthly(t *testing.T) {
	tests := []struct {
		input Recurrence
		want  bool
	}{
		{"0 0 15 * *", true},
		{"0 0 31 * *", true},
		{"0 0 0 * *", false},
		{"0 0 32 * *", false},
	}
	for _, tt := range tests {
		got := IsValidRecurrence(tt.input)
		if got != tt.want {
			t.Errorf("IsValidRecurrence(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseRecurrence_Monthly(t *testing.T) {
	tests := []struct {
		input string
		want  Recurrence
		ok    bool
	}{
		{"0 0 15 * *", "0 0 15 * *", true},
		{"0 0 31 * *", "0 0 31 * *", true},
		{"0 0 0 * *", RecurrenceNone, false},
	}
	for _, tt := range tests {
		got, ok := ParseRecurrence(tt.input)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseRecurrence(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestWeeklyRecurrence(t *testing.T) {
	tests := []struct {
		dayName string
		want    Recurrence
	}{
		{"Monday", "0 0 * * MON"},
		{"Sunday", "0 0 * * SUN"},
		{"Invalid", RecurrenceNone},
	}
	for _, tt := range tests {
		got := WeeklyRecurrence(tt.dayName)
		if got != tt.want {
			t.Errorf("WeeklyRecurrence(%q) = %q, want %q", tt.dayName, got, tt.want)
		}
	}
}

func TestRecurrenceValue_IsZero(t *testing.T) {
	tests := []struct {
		name string
		rv   RecurrenceValue
		want bool
	}{
		{"zero value", RecurrenceValue{}, true},
		{"explicit none", RecurrenceValue{Value: RecurrenceNone}, true},
		{"daily", RecurrenceValue{Value: RecurrenceDaily}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rv.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextOccurrenceFrom(t *testing.T) {
	d := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name string
		r    Recurrence
		ref  time.Time
		want time.Time
	}{
		{"none returns zero", RecurrenceNone, d(2026, 3, 24), time.Time{}},
		{"daily returns tomorrow", RecurrenceDaily, d(2026, 3, 24), d(2026, 3, 25)},

		// weekly: today is that day → next week
		{"weekly monday on monday", WeeklyRecurrence("Monday"), d(2026, 3, 23), d(2026, 3, 30)},
		// weekly: today is wednesday, next monday is 5 days out
		{"weekly monday on wednesday", WeeklyRecurrence("Monday"), d(2026, 3, 25), d(2026, 3, 30)},
		// weekly: today is thursday, next friday is 1 day out
		{"weekly friday on thursday", WeeklyRecurrence("Friday"), d(2026, 3, 26), d(2026, 3, 27)},
		// weekly: today is sunday, next monday is tomorrow
		{"weekly monday on sunday", WeeklyRecurrence("Monday"), d(2026, 3, 29), d(2026, 3, 30)},

		// monthly: before target day → this month
		{"monthly 15th on 10th", MonthlyRecurrence(15), d(2026, 3, 10), d(2026, 3, 15)},
		// monthly: on target day → next month
		{"monthly 15th on 15th", MonthlyRecurrence(15), d(2026, 3, 15), d(2026, 4, 15)},
		// monthly: past target day → next month
		{"monthly 15th on 20th", MonthlyRecurrence(15), d(2026, 3, 20), d(2026, 4, 15)},
		// monthly: day 31 in february → capped to feb 28
		{"monthly 31st in feb", MonthlyRecurrence(31), d(2026, 2, 1), d(2026, 2, 28)},
		// monthly: day 31 on 31st → next month (april has 30 days, capped)
		{"monthly 31st on mar 31", MonthlyRecurrence(31), d(2026, 3, 31), d(2026, 4, 30)},
		// monthly: day 1 at year boundary
		{"monthly 1st on dec 5", MonthlyRecurrence(1), d(2026, 12, 5), d(2027, 1, 1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextOccurrenceFrom(tt.r, tt.ref)
			if !got.Equal(tt.want) {
				t.Errorf("NextOccurrenceFrom(%q, %v) = %v, want %v", tt.r, tt.ref.Format("2006-01-02"), got.Format("2006-01-02"), tt.want.Format("2006-01-02"))
			}
		})
	}
}
