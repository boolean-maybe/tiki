package task

import (
	"testing"

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
		{RecurrenceMonthly, "Monthly"},
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
		{"Monthly", RecurrenceMonthly},
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
	if values[len(values)-1] != "Monthly" {
		t.Errorf("last value should be Monthly, got %q", values[len(values)-1])
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
