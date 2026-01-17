package util

import (
	"testing"
)

func TestAnsiConverter_Convert(t *testing.T) {
	converter := NewAnsiConverter(true)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "foreground and background with bold",
			input:    "\x1b[38;5;228;48;5;63;1mTest Heading\x1b[0m",
			expected: "[#ffff66:#3333ff:b]Test Heading[-:-:-]",
		},
		{
			name:     "foreground only",
			input:    "\x1b[38;5;228mYellow text\x1b[0m",
			expected: "[#ffff66:-:-]Yellow text[-:-:-]",
		},
		{
			name:     "background only",
			input:    "\x1b[48;5;63mPurple background\x1b[0m",
			expected: "[-:#3333ff:-]Purple background[-:-:-]",
		},
		{
			name:     "bold only",
			input:    "\x1b[1mBold text\x1b[0m",
			expected: "[-:-:b]Bold text[-:-:-]",
		},
		{
			name:     "no formatting",
			input:    "Plain text",
			expected: "Plain text",
		},
		{
			name:     "multiple sequences",
			input:    "\x1b[38;5;228mYellow\x1b[0m normal \x1b[1mbold\x1b[0m",
			expected: "[#ffff66:-:-]Yellow[-:-:-] normal [-:-:b]bold[-:-:-]",
		},
		{
			name:     "RGB foreground",
			input:    "\x1b[38;2;255;0;0mRed text\x1b[0m",
			expected: "[#ff0000:-:-]Red text[-:-:-]",
		},
		{
			name:     "RGB background",
			input:    "\x1b[48;2;0;255;0mGreen background\x1b[0m",
			expected: "[-:#00ff00:-]Green background[-:-:-]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.Convert(tt.input)
			if result != tt.expected {
				t.Errorf("Convert() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestAnsiConverter_Disabled(t *testing.T) {
	converter := NewAnsiConverter(false)

	input := "\x1b[38;5;228;48;5;63;1mTest\x1b[0m"
	result := converter.Convert(input)

	// When disabled, should return input unchanged
	if result != input {
		t.Errorf("Disabled converter should return input unchanged, got %q", result)
	}
}

func TestAnsi256ToRGB(t *testing.T) {
	tests := []struct {
		code    int
		r, g, b int
	}{
		{63, 51, 51, 255},    // Purple (from 216-color cube)
		{228, 255, 255, 102}, // Yellow (from 216-color cube)
		{0, 0, 0, 0},         // Black
		{15, 255, 255, 255},  // White
		{232, 8, 8, 8},       // First grayscale
		{255, 238, 238, 238}, // Last grayscale
	}

	for _, tt := range tests {
		r, g, b := Ansi256ToRGB(tt.code)
		if r != tt.r || g != tt.g || b != tt.b {
			t.Errorf("Ansi256ToRGB(%d) = RGB(%d,%d,%d), want RGB(%d,%d,%d)",
				tt.code, r, g, b, tt.r, tt.g, tt.b)
		}
	}
}
