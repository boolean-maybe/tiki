package statusline

import (
	"fmt"
	"strings"
)

// blockRamp maps fill level 0..4 to a block glyph. Index 0 is the empty track;
// index 4 is a full cell. Intermediate indices give sub-cell resolution and
// double as the comet's fade tail. Index 1 shares the track glyph so the
// lightest lit level stays visible against the track; ▒ ▓ █ are the gradient.
var blockRamp = []rune{'░', '░', '▒', '▓', '█'}

const (
	blockTrack = '░'
	blockFull  = '█'
)

// RenderProgressBar builds a block-shade progress bar of cols cells. total > 0
// renders a determinate bar with a trailing " NN%"; total <= 0 renders an
// indeterminate bar (a comet sweeping by frame, no percentage).
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
	maxLevel := len(blockRamp) - 1
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
		b.WriteRune(blockRamp[level])
	}
	return fmt.Sprintf("%s %d%%", b.String(), pct)
}

// tailLen is the number of fading cells trailing the comet head.
const tailLen = 3

// renderIndeterminate draws a comet: a bright head sweeping left→right with a
// fading tail, wrapping over cols+tailLen so it fully exits and re-enters.
func renderIndeterminate(frame, cols int) string {
	head := frame % (cols + tailLen)
	var b strings.Builder
	for i := 0; i < cols; i++ {
		b.WriteRune(cometGlyph(head - i))
	}
	return b.String()
}

// cometGlyph maps distance-behind-head d to a glyph: 0=head █, 1=▓, 2=▒,
// 3=track. Cells ahead of the head (d<0) and beyond the tail are track.
func cometGlyph(d int) rune {
	switch d {
	case 0:
		return blockFull
	case 1:
		return '▓'
	case 2:
		return '▒'
	default:
		return blockTrack
	}
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
