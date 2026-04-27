package main

import (
	"errors"
	"strings"
	"testing"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
)

func TestParseExecArgs_DefaultFormatIsTable(t *testing.T) {
	opts, err := parseExecArgs([]string{"select"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Statement != "select" {
		t.Errorf("statement = %q, want %q", opts.Statement, "select")
	}
	if opts.Format != rukiRuntime.OutputTable {
		t.Errorf("format = %v, want OutputTable", opts.Format)
	}
}

func TestParseExecArgs_FormatTableSpaceAndEqual(t *testing.T) {
	for _, args := range [][]string{
		{"--format", "table", "select"},
		{"--format=table", "select"},
		// statement-then-flag order is also valid
		{"select", "--format", "table"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			opts, err := parseExecArgs(args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.Format != rukiRuntime.OutputTable {
				t.Errorf("format = %v, want OutputTable", opts.Format)
			}
			if opts.Statement != "select" {
				t.Errorf("statement = %q, want %q", opts.Statement, "select")
			}
		})
	}
}

func TestParseExecArgs_FormatJSONSpaceAndEqual(t *testing.T) {
	for _, args := range [][]string{
		{"--format", "json", "select"},
		{"--format=json", "select"},
		{"select", "--format", "json"},
		{"select", "--format=json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			opts, err := parseExecArgs(args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.Format != rukiRuntime.OutputJSON {
				t.Errorf("format = %v, want OutputJSON", opts.Format)
			}
		})
	}
}

func TestParseExecArgs_MissingFormatValue(t *testing.T) {
	_, err := parseExecArgs([]string{"--format"})
	if err == nil {
		t.Fatal("expected error for missing format value")
	}
	if !strings.Contains(err.Error(), "requires a value") {
		t.Errorf("expected 'requires a value', got: %v", err)
	}
}

func TestParseExecArgs_UnsupportedFormat(t *testing.T) {
	_, err := parseExecArgs([]string{"--format", "yaml", "select"})
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("expected 'unsupported format', got: %v", err)
	}

	// also via --format=
	_, err = parseExecArgs([]string{"--format=yaml", "select"})
	if err == nil {
		t.Fatal("expected error for unsupported format (equal form)")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("expected 'unsupported format' (equal form), got: %v", err)
	}
}

func TestParseExecArgs_UnknownFlag(t *testing.T) {
	_, err := parseExecArgs([]string{"--weird", "select"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("expected 'unknown flag', got: %v", err)
	}
}

func TestParseExecArgs_NoStatement(t *testing.T) {
	_, err := parseExecArgs([]string{})
	if err == nil {
		t.Fatal("expected error for no statement")
	}
	if !strings.Contains(err.Error(), "missing ruki statement") {
		t.Errorf("expected 'missing ruki statement', got: %v", err)
	}

	// only flags, no statement
	_, err = parseExecArgs([]string{"--format", "json"})
	if err == nil {
		t.Fatal("expected error for flag-only args")
	}
	if !strings.Contains(err.Error(), "missing ruki statement") {
		t.Errorf("expected 'missing ruki statement' (flag only), got: %v", err)
	}
}

func TestParseExecArgs_MultipleStatements(t *testing.T) {
	_, err := parseExecArgs([]string{"select", "select where id=\"x\""})
	if err == nil {
		t.Fatal("expected error for multiple statements")
	}
	if !strings.Contains(err.Error(), "multiple statements") {
		t.Errorf("expected 'multiple statements', got: %v", err)
	}
}

func TestParseExecArgs_HelpFlag(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		_, err := parseExecArgs([]string{flag})
		if !errors.Is(err, errHelpRequested) {
			t.Errorf("flag %q: expected errHelpRequested, got %v", flag, err)
		}
	}
}

// --- `--` end-of-options marker ---

// regression: ruki allows `--` line comments, and any statement whose first
// character is `-` would otherwise be parsed as an unknown flag. The `--`
// end-of-options marker lets the caller force positional interpretation.
func TestParseExecArgs_DoubleDashEnablesDashLeadingStatement(t *testing.T) {
	stmt := "-- list ready\ncount(select where status = \"ready\")"

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			"ruki line comment",
			[]string{"--", stmt},
			stmt,
		},
		{
			"flag before --",
			[]string{"--format", "json", "--", stmt},
			stmt,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseExecArgs(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.Statement != tt.want {
				t.Errorf("statement = %q, want %q", opts.Statement, tt.want)
			}
		})
	}
}

// regression: flags after `--` must be treated as part of the statement (or
// the second positional), not parsed as options. This locks in the Unix-y
// end-of-options behavior.
func TestParseExecArgs_DoubleDashStopsFlagParsing(t *testing.T) {
	// --format appears *after* --, so it's a second statement, not a flag
	_, err := parseExecArgs([]string{"--", "select", "--format", "json"})
	if err == nil {
		t.Fatal("expected 'multiple statements' error when flags appear after --")
	}
	if !strings.Contains(err.Error(), "multiple statements") {
		t.Errorf("expected multiple statements error, got: %v", err)
	}
}

func TestParseExecArgs_UnknownFlagHintsAtDoubleDash(t *testing.T) {
	// a dash-leading positional that isn't a known flag should fail loudly
	// and suggest the -- escape hatch
	_, err := parseExecArgs([]string{"-- list ready\ncount(select)"})
	if err == nil {
		t.Fatal("expected error for dash-leading arg without --")
	}
	if !strings.Contains(err.Error(), "--") {
		t.Errorf("error should mention -- as escape, got: %v", err)
	}
}
