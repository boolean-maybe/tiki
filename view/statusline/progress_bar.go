package statusline

import (
	"fmt"
	"strings"
)

// brailleRamp maps fill level 0..6 to a braille glyph. Index 0 is the empty
// track; index 6 is a full cell. Intermediate indices give sub-cell resolution.
var brailleRamp = []rune{'⣀', '⣄', '⣤', '⣦', '⣶', '⣷', '⣿'}

const (
	brailleEmpty = '⣀'
	brailleFull  = '⣿'
)

// RenderProgressBar builds a braille progress bar of cols cells. total > 0
// renders a determinate bar with a trailing " NN%"; total <= 0 renders an
// indeterminate bar (a lit segment positioned by frame, no percentage).
func RenderProgressBar(done, total, frame, cols int) string {
	if cols < 1 {
		cols = 1
	}
	if total <= 0 {
		return renderIndeterminate(frame, cols)
	}
	return renderDeterminate(done, total, cols)
}

func renderDeterminate(done, total, cols int) string {
	ratio := clampRatio(float64(done) / float64(total))
	pct := int(ratio*100 + 0.5)

	// total fill expressed in ramp sub-units across the whole bar
	maxLevel := len(brailleRamp) - 1
	subUnits := int(ratio*float64(cols*maxLevel) + 0.5)

	var b strings.Builder
	for i := 0; i < cols; i++ {
		level := subUnits - i*maxLevel
		if level < 0 {
			level = 0
		}
		if level > maxLevel {
			level = maxLevel
		}
		b.WriteRune(brailleRamp[level])
	}
	return fmt.Sprintf("%s %d%%", b.String(), pct)
}

func renderIndeterminate(frame, cols int) string {
	const segLen = 3
	pos := 0
	if cols > segLen {
		pos = frame % (cols - segLen + 1)
	}
	var b strings.Builder
	for i := 0; i < cols; i++ {
		if i >= pos && i < pos+segLen {
			b.WriteRune(brailleFull)
			continue
		}
		b.WriteRune(brailleEmpty)
	}
	return b.String()
}

func clampRatio(r float64) float64 {
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}
