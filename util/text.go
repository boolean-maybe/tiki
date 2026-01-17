package util

import "strings"

// TruncateText truncates text to maxWidth and adds "..." if it exceeds.
// Does not account for color codes - use TruncateTextWithColors for colored text.
func TruncateText(text string, maxWidth int) string {
	if maxWidth <= 3 {
		return text
	}

	runes := []rune(text)
	if len(runes) <= maxWidth {
		return text
	}

	return string(runes[:maxWidth-3]) + "..."
}

// TruncateTextWithColors truncates text to fit within maxWidth, accounting for tview color codes.
// If truncation occurs, appends "..." to indicate the text was cut.
// Color codes like [#ffffff] or [red] are not counted toward the visible width.
func TruncateTextWithColors(text string, maxWidth int) string {
	if maxWidth <= 3 {
		return text
	}

	runes := []rune(text)

	// First pass: count visible characters (excluding color codes)
	visibleCount := 0
	inColorCode := false
	for i := 0; i < len(runes); i++ {
		if runes[i] == '[' {
			inColorCode = true
		} else if inColorCode && runes[i] == ']' {
			inColorCode = false
		} else if !inColorCode {
			visibleCount++
		}
	}

	// If visible content fits, return original text
	if visibleCount <= maxWidth {
		return text
	}

	// Need to truncate - rebuild text up to maxWidth-3 visible chars, then add "..."
	targetLen := maxWidth - 3
	if targetLen < 0 {
		targetLen = 0
	}

	var result strings.Builder
	visibleCount = 0
	inColorCode = false

	for i := 0; i < len(runes); i++ {
		if runes[i] == '[' {
			// Start of color code - always include it
			result.WriteRune(runes[i])
			inColorCode = true
		} else if inColorCode {
			// Inside color code - always include it
			result.WriteRune(runes[i])
			if runes[i] == ']' {
				inColorCode = false
			}
		} else {
			// Visible character
			if visibleCount < targetLen {
				result.WriteRune(runes[i])
				visibleCount++
			} else {
				// Reached target length, stop
				break
			}
		}
	}

	result.WriteString("...")
	return result.String()
}
