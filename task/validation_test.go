package task

import (
	"strings"
	"testing"
)

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{"valid title", &Task{Title: "Valid Task"}, false},
		{"empty title", &Task{Title: ""}, true},
		{"whitespace title", &Task{Title: "   "}, true},
		{"very long title", &Task{Title: strings.Repeat("a", 201)}, true},
		{"max length title", &Task{Title: strings.Repeat("a", 200)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ValidateTitle(tt.task)
			if (msg != "") != tt.wantErr {
				t.Errorf("ValidateTitle() = %q, wantErr %v", msg, tt.wantErr)
			}
		})
	}
}

func TestIsValidPoints(t *testing.T) {
	tests := []struct {
		points int
		want   bool
	}{
		{0, true},   // unestimated is valid
		{-1, false}, // negative
		{1, true},
		{5, true},
		{10, true},  // max default
		{11, false}, // over max
	}
	for _, tt := range tests {
		if got := IsValidPoints(tt.points); got != tt.want {
			t.Errorf("IsValidPoints(%d) = %v, want %v", tt.points, got, tt.want)
		}
	}
}
