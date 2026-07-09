package util

import (
	"strings"

	"github.com/rivo/uniseg"
)

// TruncateText truncates text to maxWidth and appends a single-cell ellipsis
// "…" if it exceeds. Does not account for color codes - use
// TruncateTextWithColors for colored text.
func TruncateText(text string, maxWidth int) string {
	runes := []rune(text)
	if len(runes) <= maxWidth {
		return text
	}
	if maxWidth <= 1 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-1]) + "…"
}

// TruncateTextWithColors truncates text to fit within maxWidth display cells,
// accounting for tview color codes. If truncation occurs, appends a single-cell
// ellipsis "…" to indicate the text was cut. Color codes like [#ffffff] or [red]
// are not counted toward the visible width.
//
// Width is measured in display cells over grapheme clusters — matching
// tview.TaggedStringWidth, which the grid solver uses to size columns. Counting
// raw runes instead would over-count clusters carrying a zero-width combining
// mark (e.g. an emoji + U+FE0F variation selector), truncating text that in fact
// fits its allocated column. Clusters are also the cut unit, so a multi-rune
// glyph is never split mid-cluster.
func TruncateTextWithColors(text string, maxWidth int) string {
	if maxWidth <= 1 {
		return text
	}
	if visibleWidth(text) <= maxWidth {
		return text
	}

	// truncate to maxWidth-1 cells, reserving one cell for the ellipsis.
	target := maxWidth - 1

	var result strings.Builder
	width := 0
	forEachSegment(text, func(colorCode string, cluster string, clusterWidth int) bool {
		if colorCode != "" {
			result.WriteString(colorCode) // color codes are always kept, zero width
			return true
		}
		if width+clusterWidth > target {
			return false // this cluster would overflow — stop
		}
		result.WriteString(cluster)
		width += clusterWidth
		return true
	})

	result.WriteString("…")
	return result.String()
}

// visibleWidth returns the display-cell width of text, excluding tview color codes.
func visibleWidth(text string) int {
	total := 0
	forEachSegment(text, func(colorCode string, _ string, clusterWidth int) bool {
		if colorCode == "" {
			total += clusterWidth
		}
		return true
	})
	return total
}

// forEachSegment walks text as an alternating sequence of tview color codes and
// grapheme clusters, invoking fn for each. Exactly one of colorCode / cluster is
// non-empty per call; clusterWidth is the cluster's display width (0 for color
// codes). Iteration stops early when fn returns false.
func forEachSegment(text string, fn func(colorCode, cluster string, clusterWidth int) bool) {
	runes := []rune(text)
	for i := 0; i < len(runes); {
		if runes[i] == '[' {
			end := colorCodeEnd(runes, i)
			if end > i {
				if !fn(string(runes[i:end]), "", 0) {
					return
				}
				i = end
				continue
			}
		}
		cluster, _, clusterWidth := nextCluster(runes[i:])
		if !fn("", cluster, clusterWidth) {
			return
		}
		i += len([]rune(cluster))
	}
}

// colorCodeEnd returns the index just past the closing ']' of a color code that
// starts at start, or start itself if the run is not a closed color code.
func colorCodeEnd(runes []rune, start int) int {
	for j := start + 1; j < len(runes); j++ {
		if runes[j] == ']' {
			return j + 1
		}
	}
	return start
}

// nextCluster returns the first grapheme cluster of runes, its byte length, and
// its display width (in cells) via uniseg — the same clustering tview uses.
func nextCluster(runes []rune) (cluster string, bytes int, width int) {
	c, _, w, _ := uniseg.FirstGraphemeClusterInString(string(runes), -1)
	return c, len(c), w
}
