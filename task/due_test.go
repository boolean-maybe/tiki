package task

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDueValue_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		expectZero  bool
		expectValue string // YYYY-MM-DD format if not zero
		wantErr     bool
	}{
		// Valid scenarios
		{
			name:       "empty due (omitted)",
			yaml:       "other: value",
			expectZero: true,
			wantErr:    false,
		},
		{
			name:       "empty string",
			yaml:       "due: ''",
			expectZero: true,
			wantErr:    false,
		},
		{
			name:        "valid date",
			yaml:        "due: 2026-03-16",
			expectZero:  false,
			expectValue: "2026-03-16",
			wantErr:     false,
		},
		{
			name:        "valid date with quotes",
			yaml:        "due: '2026-03-16'",
			expectZero:  false,
			expectValue: "2026-03-16",
			wantErr:     false,
		},
		{
			name:        "valid date with whitespace",
			yaml:        "due: '  2026-03-16  '",
			expectZero:  false,
			expectValue: "2026-03-16",
			wantErr:     false,
		},

		// Invalid scenarios - should default to zero with no error
		{
			name:       "invalid date format",
			yaml:       "due: 03/16/2026",
			expectZero: true,
			wantErr:    false,
		},
		{
			name:       "invalid date value",
			yaml:       "due: 2026-13-45",
			expectZero: true,
			wantErr:    false,
		},
		{
			name:       "number instead of string",
			yaml:       "due: 20260316",
			expectZero: true,
			wantErr:    false,
		},
		{
			name:       "boolean instead of string",
			yaml:       "due: true",
			expectZero: true,
			wantErr:    false,
		},
		{
			name:       "object instead of string",
			yaml:       "due:\n  key: value",
			expectZero: true,
			wantErr:    false,
		},
		{
			name:       "null value",
			yaml:       "due: null",
			expectZero: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				Due DueValue `yaml:"due,omitempty"`
			}

			var result testStruct
			err := yaml.Unmarshal([]byte(tt.yaml), &result)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.expectZero {
				if !result.Due.IsZero() {
					t.Errorf("UnmarshalYAML() expected zero time, got = %v", result.Due.ToTime())
				}
			} else {
				if result.Due.IsZero() {
					t.Errorf("UnmarshalYAML() expected non-zero time, got zero")
				}
				got := result.Due.Format(DateFormat)
				if got != tt.expectValue {
					t.Errorf("UnmarshalYAML() got = %v, expected %v", got, tt.expectValue)
				}
			}
		})
	}
}

func TestDueValue_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		due      DueValue
		expected string
	}{
		{
			name:     "zero time",
			due:      DueValue{},
			expected: "due: \"\"\n",
		},
		{
			name:     "valid date",
			due:      DueValue{Time: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
			expected: "due: \"2026-03-16\"\n",
		},
		{
			name:     "different valid date",
			due:      DueValue{Time: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)},
			expected: "due: \"2026-12-31\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				Due DueValue `yaml:"due"`
			}

			input := testStruct{Due: tt.due}
			got, err := yaml.Marshal(input)
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}

			if string(got) != tt.expected {
				t.Errorf("MarshalYAML() got = %q, expected %q", string(got), tt.expected)
			}
		})
	}
}

func TestDueValue_ToTime(t *testing.T) {
	tests := []struct {
		name     string
		due      DueValue
		expected time.Time
	}{
		{
			name:     "zero time",
			due:      DueValue{},
			expected: time.Time{},
		},
		{
			name:     "valid date",
			due:      DueValue{Time: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
			expected: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.due.ToTime()
			if !got.Equal(tt.expected) {
				t.Errorf("ToTime() got = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestParseDueDate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTime  string // YYYY-MM-DD format, or empty for zero
		wantValid bool
	}{
		{
			name:      "valid date",
			input:     "2026-03-16",
			wantTime:  "2026-03-16",
			wantValid: true,
		},
		{
			name:      "valid date with whitespace",
			input:     "  2026-03-16  ",
			wantTime:  "2026-03-16",
			wantValid: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantTime:  "",
			wantValid: true,
		},
		{
			name:      "whitespace only",
			input:     "   ",
			wantTime:  "",
			wantValid: true,
		},
		{
			name:      "invalid format",
			input:     "03/16/2026",
			wantTime:  "",
			wantValid: false,
		},
		{
			name:      "invalid date",
			input:     "2026-13-45",
			wantTime:  "",
			wantValid: false,
		},
		{
			name:      "partial date",
			input:     "2026-03",
			wantTime:  "",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := ParseDueDate(tt.input)
			if valid != tt.wantValid {
				t.Errorf("ParseDueDate() valid = %v, wantValid %v", valid, tt.wantValid)
				return
			}

			if tt.wantTime == "" {
				if !got.IsZero() {
					t.Errorf("ParseDueDate() expected zero time, got = %v", got)
				}
			} else {
				if got.IsZero() {
					t.Errorf("ParseDueDate() expected non-zero time, got zero")
				}
				gotStr := got.Format(DateFormat)
				if gotStr != tt.wantTime {
					t.Errorf("ParseDueDate() got = %v, wantTime %v", gotStr, tt.wantTime)
				}
			}
		})
	}
}

func TestDueValue_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		due  DueValue
	}{
		{
			name: "zero time",
			due:  DueValue{},
		},
		{
			name: "valid date",
			due:  DueValue{Time: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
		},
		{
			name: "different valid date",
			due:  DueValue{Time: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				Due DueValue `yaml:"due"`
			}

			// Marshal
			input := testStruct{Due: tt.due}
			yamlBytes, err := yaml.Marshal(input)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// Unmarshal
			var output testStruct
			err = yaml.Unmarshal(yamlBytes, &output)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			// Compare
			got := output.Due.ToTime()
			expected := tt.due.ToTime()
			if !got.Equal(expected) {
				t.Errorf("Round trip failed: got = %v, expected %v", got, expected)
			}
		})
	}
}

func TestDueValue_OmitEmpty(t *testing.T) {
	type testStruct struct {
		Due DueValue `yaml:"due,omitempty"`
	}

	t.Run("zero time omitted", func(t *testing.T) {
		input := testStruct{Due: DueValue{}}
		yamlBytes, err := yaml.Marshal(input)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		// Should be empty (field omitted)
		if string(yamlBytes) != "{}\n" {
			t.Errorf("omitempty failed: got = %q, expected %q", string(yamlBytes), "{}\n")
		}
	})

	t.Run("non-zero time included", func(t *testing.T) {
		input := testStruct{Due: DueValue{Time: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)}}
		yamlBytes, err := yaml.Marshal(input)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}

		// Should contain the due field
		var output testStruct
		err = yaml.Unmarshal(yamlBytes, &output)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if output.Due.IsZero() {
			t.Errorf("non-zero time should not be omitted")
		}
	})
}
