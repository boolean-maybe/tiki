package service

import (
	"strings"

	"github.com/atotto/clipboard"
)

// ExecuteClipboardPipe writes the given rows to the system clipboard.
// Fields within a row are tab-separated; rows are newline-separated.
func ExecuteClipboardPipe(rows [][]string) error {
	lines := make([]string, len(rows))
	for i, row := range rows {
		lines[i] = strings.Join(row, "\t")
	}
	return clipboard.WriteAll(strings.Join(lines, "\n"))
}
