package main

import (
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/app"
	"github.com/boolean-maybe/tiki/internal/bootstrap"
	"github.com/boolean-maybe/tiki/internal/pipe"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/internal/viewer"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/util/sysinfo"
)

//go:embed ai/skills/tiki/SKILL.md
var tikiSkillMdContent string

//go:embed ai/skills/doki/SKILL.md
var dokiSkillMdContent string

// main runs the application bootstrap and starts the TUI.
func main() {
	// Handle version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("tiki version %s\ncommit: %s\nbuilt: %s\n",
			config.Version, config.GitCommit, config.BuildDate)
		os.Exit(0)
	}

	// Handle help flag
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printUsage()
		os.Exit(0)
	}

	// Handle sysinfo command
	if len(os.Args) > 1 && os.Args[1] == "sysinfo" {
		if err := runSysInfo(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	// Handle demo command — must run before InitPaths so that os.Chdir takes effect
	if len(os.Args) > 1 && os.Args[1] == "demo" {
		if err := runDemo(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}

	// Handle init command — must run before InitPaths (chdir may change cwd)
	if len(os.Args) > 1 && os.Args[1] == "init" {
		os.Exit(runInit(os.Args[2:]))
	}

	// Initialize paths early - this must succeed for the application to function
	if err := config.InitPaths(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// Handle workflow command
	if len(os.Args) > 1 && os.Args[1] == "workflow" {
		os.Exit(runWorkflow(os.Args[2:]))
	}

	// Handle exec command: execute ruki statement and exit
	if len(os.Args) > 1 && os.Args[1] == "exec" {
		os.Exit(runExec(os.Args[2:]))
	}

	// Handle piped stdin: create a task and exit without launching TUI
	if pipe.IsPipedInput() && !pipe.HasPositionalArgs(os.Args[1:]) {
		taskID, err := pipe.CreateTaskFromReader(os.Stdin)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println(taskID)
		return
	}

	// Handle viewer mode (standalone markdown viewer)
	viewerInput, runViewer, err := viewer.ParseViewerInput(os.Args[1:], map[string]struct{}{"demo": {}, "exec": {}, "workflow": {}})
	if err != nil {
		if errors.Is(err, viewer.ErrMultipleInputs) {
			_, _ = fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if runViewer {
		if err := viewer.Run(viewerInput); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	// Check if project is initialized before launching TUI
	if !config.IsProjectInitialized() {
		printUsage()
		return
	}

	// Bootstrap application
	result, err := bootstrap.Bootstrap(tikiSkillMdContent, dokiSkillMdContent)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if result == nil {
		return
	}

	// Cleanup on exit
	defer result.App.Stop()
	defer result.HeaderWidget.Cleanup()
	defer result.RootLayout.Cleanup()
	defer result.ActionPalette.Cleanup()
	defer result.CancelFunc()

	// Run application
	if err := app.Run(result.App, result.AppRoot); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}

	// Keep logLevel variable referenced so it isn't optimized away in some builds
	_ = result.LogLevel
}

// runSysInfo handles the sysinfo command, displaying system and terminal environment information.
func runSysInfo() error {
	// Initialize paths first (needed for ConfigDir, CacheDir)
	if err := config.InitPaths(); err != nil {
		return fmt.Errorf("initialize paths: %w", err)
	}

	info := sysinfo.NewSystemInfo()

	// Print formatted system information
	fmt.Print(info.String())

	return nil
}

// errHelpRequested is returned by arg parsers when the user asks for help.
// Callers should print usage and exit cleanly — not treat it as a real error.
var errHelpRequested = errors.New("help requested")

// exit codes for CLI subcommands
const (
	exitOK             = 0
	exitInternal       = 1
	exitUsage          = 2
	exitStartupFailure = 3
	exitQueryError     = 4
)

// ExecOpts holds parsed arguments for the exec subcommand.
type ExecOpts struct {
	Statement string
	Format    rukiRuntime.OutputFormat
}

// parseExecArgs parses `tiki exec` arguments, accepting a single ruki
// statement plus an optional `--format table|json` flag. Follows the
// parseInitArgs style.
//
// Supports the standard `--` end-of-options marker: every arg after `--` is
// treated as positional, so ruki statements that legitimately start with `-`
// (most commonly a leading `-- ruki line comment`) can be passed without
// being mistaken for flags.
func parseExecArgs(args []string) (ExecOpts, error) {
	opts := ExecOpts{Format: rukiRuntime.OutputTable}
	var statement string
	endOfOptions := false

	setFormat := func(value, origin string) error {
		fmtVal, err := parseExecFormat(value)
		if err != nil {
			return fmt.Errorf("%s %s", origin, err.Error())
		}
		opts.Format = fmtVal
		return nil
	}

	takeStatement := func(arg string) error {
		if statement != "" {
			return fmt.Errorf("multiple statements: only one ruki statement is allowed")
		}
		statement = arg
		return nil
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfOptions {
			if err := takeStatement(arg); err != nil {
				return ExecOpts{}, err
			}
			continue
		}

		switch {
		case arg == "--":
			endOfOptions = true

		case arg == "--help" || arg == "-h":
			return ExecOpts{}, errHelpRequested

		case arg == "--format":
			i++
			if i >= len(args) {
				return ExecOpts{}, fmt.Errorf("--format requires a value")
			}
			if err := setFormat(args[i], "--format"); err != nil { //nolint:gosec // G602: bounds checked above
				return ExecOpts{}, err
			}

		case strings.HasPrefix(arg, "--format="):
			if err := setFormat(strings.TrimPrefix(arg, "--format="), "--format="); err != nil {
				return ExecOpts{}, err
			}

		case strings.HasPrefix(arg, "-"):
			return ExecOpts{}, fmt.Errorf("unknown flag: %s (use -- to pass a statement that starts with '-')", arg)

		default:
			if err := takeStatement(arg); err != nil {
				return ExecOpts{}, err
			}
		}
	}

	if statement == "" {
		return ExecOpts{}, fmt.Errorf("missing ruki statement")
	}

	opts.Statement = statement
	return opts, nil
}

// parseExecFormat maps the --format value to an OutputFormat. Only `table`
// and `json` are accepted.
func parseExecFormat(value string) (rukiRuntime.OutputFormat, error) {
	switch value {
	case "table":
		return rukiRuntime.OutputTable, nil
	case "json":
		return rukiRuntime.OutputJSON, nil
	default:
		return 0, fmt.Errorf("unsupported format %q (supported: table, json)", value)
	}
}

// runExec implements `tiki exec [--format table|json] '<statement>'`. Returns an exit code.
func runExec(args []string) int {
	opts, err := parseExecArgs(args)
	if err != nil {
		if errors.Is(err, errHelpRequested) {
			printExecUsage()
			return exitOK
		}
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		printExecUsage()
		return exitUsage
	}

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return exitStartupFailure
	}

	bootstrap.InitCLILogging(cfg)

	if name := config.GetStoreName(); name != "tiki" {
		_, _ = fmt.Fprintf(os.Stderr, "error: unknown store backend: %q (supported: tiki)\n", name)
		return exitStartupFailure
	}

	if config.GetStoreGit() {
		if err := bootstrap.EnsureGitRepo(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error:", err)
			return exitStartupFailure
		}
	}

	if !config.IsProjectInitialized() {
		_, _ = fmt.Fprintln(os.Stderr, "error: project not initialized: run 'tiki init' first")
		return exitStartupFailure
	}

	if err := config.InstallDefaultWorkflow(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: install default workflow: %v\n", err)
	}

	if err := config.LoadWorkflowRegistries(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: load workflow registries: %v\n", err)
		return exitStartupFailure
	}

	gate := service.BuildGate()

	_, taskStore, err := bootstrap.InitStores()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: initialize store: %v\n", err)
		return exitStartupFailure
	}
	gate.SetStore(taskStore)

	// load triggers so exec queries fire them — same identity projection as
	// bootstrap and the runtime executor, so email-only configs resolve user()
	schema := rukiRuntime.NewSchema()
	userFunc, err := store.CurrentUserDisplayFunc(taskStore)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: resolve current user: %v\n", err)
		return exitStartupFailure
	}
	if _, _, err := service.LoadAndRegisterTriggers(gate, schema, userFunc); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: load triggers: %v\n", err)
		return exitStartupFailure
	}

	runOpts := rukiRuntime.RunQueryOptions{OutputFormat: opts.Format}
	if err := rukiRuntime.RunQueryWithOptions(gate, opts.Statement, os.Stdout, runOpts); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitQueryError
	}

	return exitOK
}

// printExecUsage prints usage for the exec subcommand.
func printExecUsage() {
	fmt.Print(`Usage: tiki exec [options] [--] '<ruki-statement>'

Execute a ruki statement and exit. Requires an initialized project.

Options:
  --format <table|json>    Output format (default: table)
  --                       End of options; everything after is the statement
  -h, --help               Show this help message

Examples:
  tiki exec 'select where status = "ready"'
  tiki exec --format json 'select id, title where status = "ready"'
  tiki exec --format=json 'count(select)'

  # statements that start with '-' need the -- marker
  # (e.g. a leading '--' ruki line comment)
  tiki exec -- '-- backlog count
count(select where status != "done")'
`)
}

// printUsage prints usage information when tiki is run in an uninitialized repo.
func printUsage() {
	fmt.Print(`tiki - Terminal-based task and documentation management

Usage:
  tiki                       Launch TUI in initialized repo
  tiki init [dir] [options]    Initialize project (exits without launching TUI)
  tiki exec [--format table|json] '<statement>'    Execute a ruki query and exit
  tiki workflow reset [target]  Reset config files (--global, --local, --current)
  tiki workflow install <source> Install a workflow (--global, --local, --current)
  tiki demo                  Launch demo project (extracts embedded files on first run)
  tiki file.md/URL           View markdown file or image
  echo "Title" | tiki        Create task from piped input
  tiki sysinfo               Display system information
  tiki --help                Show this help message
  tiki --version             Show version

Options:
  --log-level <level>   Set log level (debug, info, warn, error)

Run 'tiki init' to initialize this repository.
`)
}
