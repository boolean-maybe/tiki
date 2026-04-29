package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/repair"
)

// runRepair implements `tiki repair ids [--check|--fix] [--regenerate-duplicates]`.
// Exits with a non-zero status when --check finds issues, so it is usable as
// a CI gate.
func runRepair(args []string) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "usage: tiki repair ids [--check|--fix] [--regenerate-duplicates]")
		return 2
	}
	switch args[0] {
	case "ids":
		return runRepairIDs(args[1:])
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown repair subcommand: %s\n", args[0])
		return 2
	}
}

func runRepairIDs(args []string) int {
	fs := flag.NewFlagSet("tiki repair ids", flag.ContinueOnError)
	check := fs.Bool("check", false, "report issues without modifying files")
	fix := fs.Bool("fix", false, "write fixes to disk")
	regenDup := fs.Bool("regenerate-duplicates", false, "with --fix, regenerate ids for duplicates (keeps the first sorted path unchanged)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *check == *fix {
		_, _ = fmt.Fprintln(os.Stderr, "exactly one of --check or --fix is required")
		return 2
	}

	mode := repair.ModeCheck
	if *fix {
		mode = repair.ModeFix
	}

	// Phase 2: repair walks the unified document root so it sees the same
	// files the store loads — both legacy `.doc/tiki/*.md` and new
	// `.doc/<ID>.md`, including nested layouts.
	dir := config.GetDocDir()
	rep, err := repair.RepairIDs(repair.Options{
		Dir:                  dir,
		Mode:                 mode,
		RegenerateDuplicates: *regenDup,
	})
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	repair.WriteReport(os.Stdout, rep, mode)

	if mode == repair.ModeCheck && rep.HasIssues() {
		return 1
	}
	return 0
}
