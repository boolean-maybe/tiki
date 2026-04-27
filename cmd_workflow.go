package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/ruki"
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
		_, _ = fmt.Fprintf(os.Stderr, "error: unknown target: %q (use config or workflow)\n", positional)
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
		_, _ = fmt.Fprintln(os.Stderr, "error: workflow source required")
		printWorkflowInstallUsage()
		return exitUsage
	}

	if _, err := config.ResolveDir(scope); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitInternal
	}

	src, err := config.ClassifyWorkflowInput(name)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitInternal
	}

	content, err := config.FetchWorkflowContent(src)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitInternal
	}

	if src.Kind != config.WorkflowSourceEmbedded {
		vw, err := config.ValidateWorkflowContent(content)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: invalid workflow: %v\n", err)
			return exitInternal
		}
		if err := validateWorkflowViews(vw, content); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: invalid workflow: %v\n", err)
			return exitInternal
		}
	}

	results, err := config.InstallWorkflowFromContent(content, scope)
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
		_, _ = fmt.Fprintln(os.Stderr, "error: workflow source required")
		printWorkflowDescribeUsage()
		return exitUsage
	}

	src, err := config.ClassifyWorkflowInput(name)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitInternal
	}

	desc, err := config.DescribeWorkflow(src)
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

// validateWorkflowViews validates triggers and plugin views in a non-embedded
// workflow. Requires the ValidatedWorkflow (from config.ValidateWorkflowContent)
// and the raw YAML content (written to a temp file for plugin loading).
func validateWorkflowViews(vw *config.ValidatedWorkflow, content string) error {
	schema := runtime.NewSchemaFromRegistries(vw.StatusReg, vw.TypeReg, vw.FieldDefs)

	if len(vw.TriggerDefs) > 0 {
		parser := ruki.NewParser(schema)
		for i, def := range vw.TriggerDefs {
			desc := def.Description
			if desc == "" {
				desc = fmt.Sprintf("#%d", i+1)
			}
			if _, err := parser.ParseAndValidateRule(def.Ruki); err != nil {
				return fmt.Errorf("trigger %q: %w", desc, err)
			}
		}
	}

	tmp, err := os.CreateTemp("", "tiki-validate-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write([]byte(content)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// suppress INFO logs from plugin loader during validation-only pass
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	_, err = plugin.LoadPluginsFromFile(tmp.Name(), schema)
	slog.SetDefault(prev)

	return err
}

func printWorkflowUsage() {
	fmt.Print(`Usage: tiki workflow <command>

Commands:
  reset [target] [--scope]      Reset config files to defaults
  install <source> [--scope]    Install a workflow (embedded name, file path, or URL)
  describe <source>             Print a workflow's description

Run 'tiki workflow <command> --help' for details.
`)
}

func printWorkflowResetUsage() {
	fmt.Print(`Usage: tiki workflow reset [target] [--scope]

Reset configuration files to their defaults.

Targets (omit to reset all):
  config     Reset config.yaml
  workflow   Reset workflow.yaml

Scopes (default: --local):
  --global   User config directory
  --local    Project config directory (.doc/)
  --current  Current working directory
`)
}

func printWorkflowInstallUsage() {
	fmt.Print(`Usage: tiki workflow install <source> [--scope]

Install a workflow from an embedded name, local file, or URL.
Writes workflow.yaml into the scope directory,
overwriting any existing file.

Sources:
  Embedded names:  kanban, todo, bug-tracker
  File path:       ./my-workflow.yaml, /path/to/workflow.yaml
  URL:             https://example.com/workflow.yaml

Scopes (default: --local):
  --global   User config directory
  --local    Project config directory (.doc/)
  --current  Current working directory

Examples:
  tiki workflow install kanban --global
  tiki workflow install ./custom.yaml --local
  tiki workflow install https://example.com/workflow.yaml --global
`)
}

func printWorkflowDescribeUsage() {
	fmt.Print(`Usage: tiki workflow describe <source>

Print a workflow's description. Reads the top-level 'description' field.

Sources:
  Embedded names:  kanban, todo, bug-tracker
  File path:       ./my-workflow.yaml
  URL:             https://example.com/workflow.yaml

Examples:
  tiki workflow describe todo
  tiki workflow describe ./custom.yaml
  tiki workflow describe https://example.com/workflow.yaml
`)
}
