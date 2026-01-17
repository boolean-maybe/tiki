package util

import "testing"

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		expected string
	}{
		{
			name:     "text fits exactly",
			text:     "hello",
			maxWidth: 5,
			expected: "hello",
		},
		{
			name:     "text is shorter",
			text:     "hi",
			maxWidth: 10,
			expected: "hi",
		},
		{
			name:     "text needs truncation",
			text:     "hello world",
			maxWidth: 8,
			expected: "hello...",
		},
		{
			name:     "very small width",
			text:     "hello",
			maxWidth: 3,
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateText(tt.text, tt.maxWidth)
			if result != tt.expected {
				t.Errorf("TruncateText(%q, %d) = %q, want %q", tt.text, tt.maxWidth, result, tt.expected)
			}
		})
	}
}

func TestTruncateTextWithColors(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		expected string
	}{
		{
			name:     "no color codes - fits",
			text:     "hello",
			maxWidth: 5,
			expected: "hello",
		},
		{
			name:     "no color codes - truncate",
			text:     "hello world",
			maxWidth: 8,
			expected: "hello...",
		},
		{
			name:     "with color codes - fits",
			text:     "[#ff0000]hello[-]",
			maxWidth: 5,
			expected: "[#ff0000]hello[-]",
		},
		{
			name:     "with color codes - truncate",
			text:     "[#ff0000]hello world[-]",
			maxWidth: 8,
			expected: "[#ff0000]hello...",
		},
		{
			name:     "multiple color codes",
			text:     "[red]hello[blue] world[-]",
			maxWidth: 8,
			expected: "[red]hello[blue]...",
		},
		{
			name:     "color at end",
			text:     "hello [#00ff00]world[-]",
			maxWidth: 8,
			expected: "hello...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateTextWithColors(tt.text, tt.maxWidth)
			if result != tt.expected {
				t.Errorf("TruncateTextWithColors(%q, %d) = %q, want %q", tt.text, tt.maxWidth, result, tt.expected)
			}
		})
	}
}
