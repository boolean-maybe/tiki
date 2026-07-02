// Package gridlayout — layout_string.go
//
// ParseLayout accepts a block-scalar layout string (the value of `layout:` in
// workflow.yaml when written as a YAML `|` block scalar) and returns the same
// GridSpec that ParseGrid produces from a [][]string. It splits the string
// into rows on '\n' and cells on '|', honoring "..." quoted regions so a
// literal '|' inside a quoted string is not treated as a cell delimiter.
//
// Blank lines and lines whose first non-whitespace character is '#' are
// skipped. Each cell is trimmed of outer whitespace before being handed to
// the existing tokenizer via ParseGrid.
package gridlayout

import (
	"fmt"
	"strings"
)

// ParseLayout parses a block-scalar layout string into a GridSpec.
// Returns an error if the layout has no non-blank rows or if grid
// assembly fails.
func ParseLayout(s string) (GridSpec, error) {
	rows := splitLayoutString(s)
	if len(rows) == 0 {
		return GridSpec{}, fmt.Errorf("layout: empty")
	}
	return ParseGrid(rows)
}

// splitLayoutString splits a block-scalar layout into rows and cells.
// Cells are separated by '|', honoring "..." quoted regions. Blank lines
// and '#'-prefix comment lines are skipped. Each cell is trimmed of
// outer whitespace; quoted content is preserved verbatim including its
// inner whitespace.
func splitLayoutString(s string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		rows = append(rows, splitRowCells(line))
	}
	return rows
}

// splitRowCells walks a row character-by-character, splitting on '|' when
// outside a double-quoted region. Each resulting cell is trimmed.
func splitRowCells(row string) []string {
	var cells []string
	var current strings.Builder
	inQuote := false
	for _, r := range row {
		switch {
		case r == '"':
			inQuote = !inQuote
			current.WriteRune(r)
		case r == '|' && !inQuote:
			cells = append(cells, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	cells = append(cells, strings.TrimSpace(current.String()))
	return cells
}
