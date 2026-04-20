package ruki

import (
	"testing"
	"time"
)

func TestParseScalarTypeName(t *testing.T) {
	tests := []struct {
		input   string
		want    ValueType
		wantErr bool
	}{
		{"string", ValueString, false},
		{"int", ValueInt, false},
		{"bool", ValueBool, false},
		{"date", ValueDate, false},
		{"timestamp", ValueTimestamp, false},
		{"duration", ValueDuration, false},
		{"String", ValueString, false},
		{"INT", ValueInt, false},
		{"enum", 0, true},
		{"list", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseScalarTypeName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseScalarTypeName(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseScalarValue_String(t *testing.T) {
	val, err := ParseScalarValue(ValueString, "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if val != "hello world" {
		t.Fatalf("got %v, want %q", val, "hello world")
	}
}

func TestParseScalarValue_Int(t *testing.T) {
	val, err := ParseScalarValue(ValueInt, "42")
	if err != nil {
		t.Fatal(err)
	}
	if val != 42 {
		t.Fatalf("got %v, want 42", val)
	}

	val, err = ParseScalarValue(ValueInt, " 7 ")
	if err != nil {
		t.Fatal(err)
	}
	if val != 7 {
		t.Fatalf("got %v, want 7", val)
	}

	_, err = ParseScalarValue(ValueInt, "abc")
	if err == nil {
		t.Fatal("expected error for non-integer")
	}
}

func TestParseScalarValue_Bool(t *testing.T) {
	val, err := ParseScalarValue(ValueBool, "true")
	if err != nil {
		t.Fatal(err)
	}
	if val != true {
		t.Fatalf("got %v, want true", val)
	}

	val, err = ParseScalarValue(ValueBool, "False")
	if err != nil {
		t.Fatal(err)
	}
	if val != false {
		t.Fatalf("got %v, want false", val)
	}

	_, err = ParseScalarValue(ValueBool, "yes")
	if err == nil {
		t.Fatal("expected error for non-bool string")
	}
}

func TestParseScalarValue_Date(t *testing.T) {
	val, err := ParseScalarValue(ValueDate, "2025-03-15")
	if err != nil {
		t.Fatal(err)
	}
	tv, ok := val.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T", val)
	}
	if tv.Year() != 2025 || tv.Month() != 3 || tv.Day() != 15 {
		t.Fatalf("got %v", tv)
	}

	_, err = ParseScalarValue(ValueDate, "not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

func TestParseScalarValue_Timestamp_RFC3339(t *testing.T) {
	val, err := ParseScalarValue(ValueTimestamp, "2025-03-15T10:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	tv, ok := val.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T", val)
	}
	if tv.Hour() != 10 || tv.Minute() != 30 {
		t.Fatalf("got %v", tv)
	}
}

func TestParseScalarValue_Timestamp_DateFallback(t *testing.T) {
	val, err := ParseScalarValue(ValueTimestamp, "2025-03-15")
	if err != nil {
		t.Fatal(err)
	}
	tv, ok := val.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T", val)
	}
	if tv.Year() != 2025 || tv.Month() != 3 || tv.Day() != 15 {
		t.Fatalf("got %v", tv)
	}
}

func TestParseScalarValue_Timestamp_Invalid(t *testing.T) {
	_, err := ParseScalarValue(ValueTimestamp, "yesterday")
	if err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
}

func TestParseScalarValue_Duration(t *testing.T) {
	val, err := ParseScalarValue(ValueDuration, "2day")
	if err != nil {
		t.Fatal(err)
	}
	d, ok := val.(time.Duration)
	if !ok {
		t.Fatalf("expected time.Duration, got %T", val)
	}
	if d != 2*24*time.Hour {
		t.Fatalf("got %v, want %v", d, 2*24*time.Hour)
	}

	_, err = ParseScalarValue(ValueDuration, "abc")
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestParseScalarValue_UnsupportedType(t *testing.T) {
	_, err := ParseScalarValue(ValueEnum, "test")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}
