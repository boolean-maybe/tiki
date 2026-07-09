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
			expected: "hello w…",
		},
		{
			name:     "very small width",
			text:     "hello",
			maxWidth: 3,
			expected: "he…",
		},
		{
			name:     "single-cell ellipsis glyph, not three dots",
			text:     "abcdefghij",
			maxWidth: 5,
			expected: "abcd…",
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
			expected: "hello w…",
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
			expected: "[#ff0000]hello w…",
		},
		{
			name:     "multiple color codes",
			text:     "[red]hello[blue] world[-]",
			maxWidth: 8,
			expected: "[red]hello[blue] w…",
		},
		{
			name:     "color at end",
			text:     "hello [#00ff00]world[-]",
			maxWidth: 8,
			expected: "hello [#00ff00]w…",
		},
		{
			name:     "small width floor — returns unchanged",
			text:     "[red]hi[-]",
			maxWidth: 3,
			expected: "[red]hi[-]",
		},
		{
			// "⚙️" is base gear U+2699 + variation selector U+FE0F (2 runes),
			// one grapheme cluster of display width 2. "In Progress ⚙️" is 14
			// cells; the raw-rune counter over-counted the VS16 as a 15th cell.
			// Display-width counting sizes it to 14, so at its measured width
			// it must survive intact.
			name:     "emoji with variation selector fits by display width",
			text:     "In Progress ⚙️",
			maxWidth: 14,
			expected: "In Progress ⚙️",
		},
		{
			// the cluster is never split mid-glyph: at maxWidth=13 the 2-cell
			// gear would overflow, so it is dropped whole in favour of "…".
			name:     "cluster dropped whole, not split",
			text:     "In Progress ⚙️",
			maxWidth: 13,
			expected: "In Progress …",
		},
		{
			// wide (2-cell) emoji: "🔁" is one cluster of display width 2, so
			// "Regression 🔁" is 13 cells and must survive maxWidth=13.
			name:     "wide emoji counted as two cells",
			text:     "Regression 🔁",
			maxWidth: 13,
			expected: "Regression 🔁",
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
