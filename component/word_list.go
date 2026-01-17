package component

import (
	"strings"

	"github.com/boolean-maybe/tiki/config"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WordList displays a list of words space-separated with word wrapping.
// Words are never broken in the middle; wrapping occurs at word boundaries.
type WordList struct {
	*tview.Box
	words   []string
	fgColor tcell.Color
	bgColor tcell.Color
}

// NewWordList creates a new WordList component.
func NewWordList(words []string) *WordList {
	box := tview.NewBox()
	box.SetBorder(false) // No visible border
	return &WordList{
		Box:     box,
		words:   words,
		fgColor: tcell.ColorCornflowerBlue, // Soft Cobalt
		bgColor: tcell.ColorNavy,
	}
}

// SetWords updates the list of words to display.
func (w *WordList) SetWords(words []string) *WordList {
	w.words = words
	return w
}

// GetWords returns the current list of words.
func (w *WordList) GetWords() []string {
	return w.words
}

// SetColors sets the foreground and background colors.
func (w *WordList) SetColors(fg, bg tcell.Color) *WordList {
	w.fgColor = fg
	w.bgColor = bg
	return w
}

// Draw renders the WordList component.
func (w *WordList) Draw(screen tcell.Screen) {
	w.DrawForSubclass(screen, w)
	x, y, width, height := w.GetInnerRect()

	if width <= 0 || height <= 0 {
		return
	}

	wordStyle := tcell.StyleDefault.Foreground(w.fgColor).Background(w.bgColor)
	spaceStyle := tcell.StyleDefault.Background(config.GetContentBackgroundColor())

	currentX := x
	currentY := y

	for i, word := range w.words {
		wordLen := len(word)
		spaceLen := 0
		if i < len(w.words)-1 {
			spaceLen = 1 // Add space after word (except last word)
		}

		// Check if word fits on current line
		if currentX > x && currentX+wordLen > x+width {
			// Word doesn't fit, move to next line
			currentY++
			currentX = x

			// Check if we've run out of vertical space
			if currentY >= y+height {
				break
			}
		}

		// Check if word is too long for the entire line
		if wordLen > width {
			// Truncate word to fit (edge case for very narrow displays)
			word = word[:width]
			wordLen = width
		}

		// Draw the word with colored style
		for j, ch := range word {
			if currentX+j < x+width {
				screen.SetContent(currentX+j, currentY, ch, nil, wordStyle)
			}
		}
		currentX += wordLen

		// Draw space after word with default style (no custom colors)
		if spaceLen > 0 && currentX < x+width {
			screen.SetContent(currentX, currentY, ' ', nil, spaceStyle)
			currentX += spaceLen
		}
	}
}

// WrapWords is a helper function that returns the wrapped lines for display.
// This can be useful for testing or previewing the layout without drawing.
func (w *WordList) WrapWords(width int) []string {
	if width <= 0 {
		return []string{}
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range w.words {
		wordLen := len(word)
		currentLen := currentLine.Len()

		// Check if word fits on current line
		needsSpace := currentLen > 0
		spaceLen := 0
		if needsSpace {
			spaceLen = 1
		}

		if needsSpace && currentLen+spaceLen+wordLen > width {
			// Word doesn't fit, finalize current line and start new one
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		} else {
			// Word fits on current line
			if needsSpace {
				currentLine.WriteRune(' ')
			}
			currentLine.WriteString(word)
		}
	}

	// Add final line if not empty
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}
