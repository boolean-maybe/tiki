package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/boolean-maybe/tiki/config"
)

// runWorkflow dispatches workflow subcommands. Returns an exit code.
func runWorkflow(args []string) int {
	if len(args) == 0 {
		printWorkflowUsage()
		return exitUsage
	}
	switch args[0] {
	case "reset":
		return runWorkflowReset(args[1:])
	case "install":
		return runWorkflowInstall(args[1:])
	case "describe":
		return runWorkflowDescribe(args[1:])
	case "--help", "-h":
		printWorkflowUsage()
		return exitOK
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown workflow command: %s\n", args[0])
		printWorkflowUsage()
		return exitUsage
	}
}

// runWorkflowReset implements `tiki workflow reset [target] --scope`.
func runWorkflowReset(args []string) int {
	positional, scope, err := parseScopeArgs(args)
	if errors.Is(err, errHelpRequested) {
		printWorkflowResetUsage()
		return exitOK
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		printWorkflowResetUsage()
		return exitUsage
	}

	target := config.ResetTarget(positional)
	if !config.ValidResetTarget(target) {
		_, _ = fmt.Fprintf(os.Stderr, "error: unknown target: %q (use config, workflow, or new)\n", positional)
		printWorkflowResetUsage()
		return exitUsage
	}

	affected, err := config.ResetConfig(scope, target)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitInternal
	}

	if len(affected) == 0 {
		fmt.Println("nothing to reset")
		return exitOK
	}
	for _, path := range affected {
		fmt.Println("reset", path)
	}
	return exitOK
}

// runWorkflowInstall implements `tiki workflow install <name> --scope`.
func runWorkflowInstall(args []string) int {
	name, scope, err := parseScopeArgs(args)
	if errors.Is(err, errHelpRequested) {
		printWorkflowInstallUsage()
		return exitOK
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		printWorkflowInstallUsage()
		return exitUsage
	}

	if name == "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: workflow name required")
		printWorkflowInstallUsage()
		return exitUsage
	}

	results, err := config.InstallWorkflow(name, scope, config.DefaultWorkflowBaseURL)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitInternal
	}

	for _, r := range results {
		if r.Changed {
			fmt.Println("installed", r.Path)
		} else {
			fmt.Println("unchanged", r.Path)
		}
	}
	return exitOK
}

// runWorkflowDescribe implements `tiki workflow describe <name>`.
// describe is a read-only network call, so scope flags are rejected
// to keep the CLI surface honest.
func runWorkflowDescribe(args []string) int {
	name, err := parsePositionalOnly(args)
	if errors.Is(err, errHelpRequested) {
		printWorkflowDescribeUsage()
		return exitOK
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		printWorkflowDescribeUsage()
		return exitUsage
	}

	if name == "" {
		_, _ = fmt.Fprintln(os.Stderr, "error: workflow name required")
		printWorkflowDescribeUsage()
		return exitUsage
	}

	desc, err := config.DescribeWorkflow(name, config.DefaultWorkflowBaseURL)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitInternal
	}
	if desc == "" {
		return exitOK
	}
	if strings.HasSuffix(desc, "\n") {
		fmt.Print(desc)
	} else {
		fmt.Println(desc)
	}
	return exitOK
}

// parseScopeArgs extracts an optional positional argument and a required --scope flag.
// Returns errHelpRequested for --help/-h.
func parseScopeArgs(args []string) (string, config.Scope, error) {
	var positional string
	var scopeStr string

	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			return "", "", errHelpRequested
		case "--global", "--local", "--current":
			if scopeStr != "" {
				return "", "", fmt.Errorf("only one scope allowed: already have --%s", scopeStr)
			}
			scopeStr = strings.TrimPrefix(arg, "--")
		default:
			if strings.HasPrefix(arg, "--") {
				return "", "", fmt.Errorf("unknown flag: %s", arg)
			}
			if positional != "" {
				return "", "", fmt.Errorf("multiple positional arguments: %q and %q", positional, arg)
			}
			positional = arg
		}
	}

	if scopeStr == "" {
		scopeStr = "local"
	}

	return positional, config.Scope(scopeStr), nil
}

// parsePositionalOnly extracts a single positional argument and rejects any
// flag other than --help/-h. Used by subcommands that don't take a scope.
func parsePositionalOnly(args []string) (string, error) {
	var positional string
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			return "", errHelpRequested
		}
		if strings.HasPrefix(arg, "--") {
			return "", fmt.Errorf("unknown flag: %s", arg)
		}
		if positional != "" {
			return "", fmt.Errorf("multiple positional arguments: %q and %q", positional, arg)
		}
		positional = arg
	}
	return positional, nil
}

func printWorkflowUsage() {
	fmt.Print(`Usage: tiki workflow <command>

Commands:
  reset [target] [--scope]      Reset config files to defaults
  install <name> [--scope]      Install a workflow from the tiki repository
  describe <name>               Fetch and print a workflow's description

Run 'tiki workflow <command> --help' for details.
`)
}

func printWorkflowResetUsage() {
	fmt.Print(`Usage: tiki workflow reset [target] [--scope]

Reset configuration files to their defaults.

Targets (omit to reset all):
  config     Reset config.yaml
  workflow   Reset workflow.yaml
  new        Reset new.md (task template)

Scopes (default: --local):
  --global   User config directory
  --local    Project config directory (.doc/)
  --current  Current working directory
`)
}

func printWorkflowInstallUsage() {
	fmt.Print(`Usage: tiki workflow install <name> [--scope]

Install a named workflow from the tiki repository.
Downloads workflow.yaml and new.md into the scope directory,
overwriting any existing files.

Scopes (default: --local):
  --global   User config directory
  --local    Project config directory (.doc/)
  --current  Current working directory

Example:
  tiki workflow install sprint --global
`)
}

func printWorkflowDescribeUsage() {
	fmt.Print(`Usage: tiki workflow describe <name>

Fetch a workflow's description from the tiki repository and print it.
Reads the top-level 'description' field of the named workflow.yaml.

Example:
  tiki workflow describe todo
`)
}
