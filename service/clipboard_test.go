package service

import (
	"testing"

	"github.com/atotto/clipboard"
)

func TestExecuteClipboardPipe(t *testing.T) {
	// skip on headless CI where clipboard is not available
	if err := clipboard.WriteAll("test"); err != nil {
		t.Skipf("clipboard not available: %v", err)
	}

	tests := []struct {
		name string
		rows [][]string
		want string
	}{
		{"single field single row", [][]string{{"TIKI-000001"}}, "TIKI-000001"},
		{"multi-field single row", [][]string{{"TIKI-000001", "Fix bug"}}, "TIKI-000001\tFix bug"},
		{"multi-row", [][]string{{"TIKI-000001", "Fix bug"}, {"TIKI-000002", "Add tests"}}, "TIKI-000001\tFix bug\nTIKI-000002\tAdd tests"},
		{"empty rows", [][]string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ExecuteClipboardPipe(tt.rows); err != nil {
				t.Fatalf("ExecuteClipboardPipe() error: %v", err)
			}
			got, err := clipboard.ReadAll()
			if err != nil {
				t.Fatalf("clipboard.ReadAll() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("clipboard = %q, want %q", got, tt.want)
			}
		})
	}
}
