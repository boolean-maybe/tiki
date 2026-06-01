package idfmt

import "testing"

func TestIsValidID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"ABC123", true},
		{"000000", true},
		{"ZZZZZZ", true},
		{"abc123", false}, // lowercase not accepted
		{"ABC12", false},  // too short
		{"ABC1234", false},
		{"ABC12!", false},
		{"", false},
		{"TIKI-ABC123", false}, // legacy form is NOT a valid bare ID
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := IsValidID(tt.id); got != tt.want {
				t.Errorf("IsValidID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
