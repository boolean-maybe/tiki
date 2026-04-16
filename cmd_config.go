package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/boolean-maybe/tiki/config"
)

// runConfig dispatches config subcommands. Returns an exit code.
func runConfig(args []string) int {
	if len(args) == 0 {
		printConfigUsage()
		return exitUsage
	}
	switch args[0] {
	case "reset":
		return runConfigReset(args[1:])
	case "--help", "-h":
		printConfigUsage()
		return exitOK
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown config command: %s\n", args[0])
		printConfigUsage()
		return exitUsage
	}
}

// runConfigReset implements `tiki config reset [target] --scope`.
func runConfigReset(args []string) int {
	target, scope, err := parseConfigResetArgs(args)
	if errors.Is(err, errHelpRequested) {
		printConfigResetUsage()
		return exitOK
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		printConfigResetUsage()
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

// parseConfigResetArgs parses arguments for `tiki config reset`.
func parseConfigResetArgs(args []string) (config.ResetTarget, config.ResetScope, error) {
	var targetStr string
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
			if targetStr != "" {
				return "", "", fmt.Errorf("multiple targets specified: %q and %q", targetStr, arg)
			}
			targetStr = arg
		}
	}

	if scopeStr == "" {
		return "", "", fmt.Errorf("scope required: --global, --local, or --current")
	}

	target := config.ResetTarget(targetStr)
	switch target {
	case config.TargetAll, config.TargetConfig, config.TargetWorkflow, config.TargetNew:
		// valid
	default:
		return "", "", fmt.Errorf("unknown target: %q (use config, workflow, or new)", targetStr)
	}

	return target, config.ResetScope(scopeStr), nil
}

func printConfigUsage() {
	fmt.Print(`Usage: tiki config <command>

Commands:
  reset [target] --scope   Reset config files to defaults

Run 'tiki config reset --help' for details.
`)
}

func printConfigResetUsage() {
	fmt.Print(`Usage: tiki config reset [target] --scope

Reset configuration files to their defaults.

Targets (omit to reset all):
  config     Reset config.yaml
  workflow   Reset workflow.yaml
  new        Reset new.md (task template)

Scopes (required):
  --global   User config directory
  --local    Project config directory (.doc/)
  --current  Current working directory
`)
}
