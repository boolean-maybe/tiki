package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// AnsiConverter converts ANSI escape sequences to tview color tags
type AnsiConverter struct {
	enabled bool
}

// NewAnsiConverter creates a new ANSI converter
// enabled: if false, returns text unchanged (uses tview.TranslateANSI as fallback)
func NewAnsiConverter(enabled bool) *AnsiConverter {
	return &AnsiConverter{
		enabled: enabled,
	}
}

// Convert translates ANSI escape sequences to tview color tags
// Properly handles foreground, background, and bold attributes
func (c *AnsiConverter) Convert(text string) string {
	if !c.enabled {
		// Fallback to tview's built-in translator
		// Note: tview.TranslateANSI doesn't handle background colors properly
		return text
	}

	// Pattern matches ANSI SGR sequences: ESC[...m
	pattern := regexp.MustCompile(`\x1b\[([0-9;]+)m`)

	result := strings.Builder{}
	lastIndex := 0

	// Current state
	var fgColor, bgColor string
	bold := false

	matches := pattern.FindAllStringSubmatchIndex(text, -1)
	for _, match := range matches {
		// Append text before this escape sequence
		result.WriteString(text[lastIndex:match[0]])

		// Extract the parameter string (e.g., "38;5;228;48;5;63;1")
		params := text[match[2]:match[3]]

		// Parse and apply the SGR parameters
		newFg, newBg, newBold := parseSGR(params, fgColor, bgColor, bold)

		// If state changed, emit tview color tag
		if newFg != fgColor || newBg != bgColor || newBold != bold {
			fgColor = newFg
			bgColor = newBg
			bold = newBold

			result.WriteString(formatTviewTag(fgColor, bgColor, bold))
		}

		lastIndex = match[1]
	}

	// Append remaining text
	result.WriteString(text[lastIndex:])

	return result.String()
}

// parseSGR parses SGR (Select Graphic Rendition) parameters
// Returns updated foreground, background, and bold state
func parseSGR(params string, currentFg, currentBg string, currentBold bool) (fg, bg string, bold bool) {
	fg = currentFg
	bg = currentBg
	bold = currentBold

	parts := strings.Split(params, ";")

	for i := 0; i < len(parts); i++ {
		code, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}

		switch code {
		case 0:
			// Reset all attributes
			fg = ""
			bg = ""
			bold = false
		case 1:
			// Bold
			bold = true
		case 22:
			// Normal intensity (not bold)
			bold = false
		case 38:
			// Foreground color (extended)
			if i+2 < len(parts) && parts[i+1] == "5" {
				// 256-color mode: ESC[38;5;Nm
				if colorCode, err := strconv.Atoi(parts[i+2]); err == nil {
					fg = Ansi256ToHex(colorCode)
					i += 2
				}
			} else if i+4 < len(parts) && parts[i+1] == "2" {
				// RGB mode: ESC[38;2;R;G;Bm
				r, _ := strconv.Atoi(parts[i+2])
				g, _ := strconv.Atoi(parts[i+3])
				b, _ := strconv.Atoi(parts[i+4])
				fg = fmt.Sprintf("#%02x%02x%02x", r, g, b)
				i += 4
			}
		case 48:
			// Background color (extended)
			if i+2 < len(parts) && parts[i+1] == "5" {
				// 256-color mode: ESC[48;5;Nm
				if colorCode, err := strconv.Atoi(parts[i+2]); err == nil {
					bg = Ansi256ToHex(colorCode)
					i += 2
				}
			} else if i+4 < len(parts) && parts[i+1] == "2" {
				// RGB mode: ESC[48;2;R;G;Bm
				r, _ := strconv.Atoi(parts[i+2])
				g, _ := strconv.Atoi(parts[i+3])
				b, _ := strconv.Atoi(parts[i+4])
				bg = fmt.Sprintf("#%02x%02x%02x", r, g, b)
				i += 4
			}
		case 39:
			// Default foreground color
			fg = ""
		case 49:
			// Default background color
			bg = ""
		}
	}

	return fg, bg, bold
}

// formatTviewTag formats a tview color tag: [foreground:background:attributes]
func formatTviewTag(fg, bg string, bold bool) string {
	// tview format: [foreground:background:attributes]
	// Use "-" for default values

	if fg == "" {
		fg = "-"
	}
	if bg == "" {
		bg = "-"
	}

	attr := "-"
	if bold {
		attr = "b"
	}

	return fmt.Sprintf("[%s:%s:%s]", fg, bg, attr)
}

// Ansi256ToHex converts ANSI 256 color code to hex color
func Ansi256ToHex(code int) string {
	r, g, b := Ansi256ToRGB(code)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// Ansi256ToRGB converts ANSI 256 color code to RGB values
func Ansi256ToRGB(code int) (r, g, b int) {
	if code < 16 {
		// Standard 16 colors
		standardColors := [][]int{
			{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
			{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
			{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
			{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
		}
		if code < len(standardColors) {
			return standardColors[code][0], standardColors[code][1], standardColors[code][2]
		}
	} else if code >= 16 && code <= 231 {
		// 216-color cube (6x6x6)
		code -= 16
		b := code % 6
		g := (code / 6) % 6
		r := code / 36
		// Each step is 51 (255/5)
		return r * 51, g * 51, b * 51
	} else if code >= 232 && code <= 255 {
		// Grayscale (24 shades)
		gray := 8 + (code-232)*10
		return gray, gray, gray
	}
	return 0, 0, 0
}
