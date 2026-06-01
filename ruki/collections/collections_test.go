package collections

import (
	"reflect"
	"testing"
)

func TestNormalizeStringSet(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"nil yields empty non-nil", nil, []string{}},
		{"trims and drops empties", []string{" a ", "", "  ", "b"}, []string{"a", "b"}},
		{"removes duplicates preserving order", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"preserves case", []string{"Foo", "foo"}, []string{"Foo", "foo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeStringSet(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeStringSet(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeRefSet(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"nil yields empty non-nil", nil, []string{}},
		{"uppercases and trims", []string{" abc123 ", "def456"}, []string{"ABC123", "DEF456"}},
		{"dedupes case-insensitively after upper", []string{"abc", "ABC", "Abc"}, []string{"ABC"}},
		{"drops empties", []string{"", "  ", "x"}, []string{"X"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeRefSet(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeRefSet(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
