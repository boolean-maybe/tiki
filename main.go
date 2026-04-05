package main

import (
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/app"
	"github.com/boolean-maybe/tiki/internal/bootstrap"
	"github.com/boolean-maybe/tiki/internal/pipe"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/internal/viewer"
	"github.com/boolean-maybe/tiki/service"
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

	// Initialize paths early - this must succeed for the application to function
	if err := config.InitPaths(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
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

	// Handle init command
	initRequested := len(os.Args) > 1 && os.Args[1] == "init"

	// Handle viewer mode (standalone markdown viewer)
	// "init" is reserved to prevent treating it as a markdown file
	viewerInput, runViewer, err := viewer.ParseViewerInput(os.Args[1:], map[string]struct{}{"init": {}, "demo": {}, "exec": {}})
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
	if !initRequested && !config.IsProjectInitialized() {
		printUsage()
		return
	}

	// Bootstrap application (handles init prompt if needed when initRequested)
	result, err := bootstrap.Bootstrap(tikiSkillMdContent, dokiSkillMdContent)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if result == nil {
		// User chose not to proceed with project initialization
		return
	}

	// Cleanup on exit
	defer result.App.Stop()
	defer result.HeaderWidget.Cleanup()
	defer result.RootLayout.Cleanup()
	defer result.CancelFunc()

	// Run application
	if err := app.Run(result.App, result.RootLayout); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}

	// Save user preferences on shutdown
	if err := config.SaveHeaderVisible(result.HeaderConfig.GetUserPreference()); err != nil {
		slog.Warn("failed to save header visibility preference", "error", err)
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

const demoRepoURL = "https://github.com/boolean-maybe/tiki-demo.git"
const demoDirName = "tiki-demo"

// runDemo clones the demo repository if needed and changes into it.
// Must be called before config.InitPaths() so the PathManager captures the demo dir as project root.
func runDemo() error {
	info, err := os.Stat(demoDirName)
	if err == nil && info.IsDir() {
		fmt.Printf("using existing %s directory\n", demoDirName)
	} else {
		fmt.Printf("cloning demo project into %s...\n", demoDirName)
		//nolint:gosec // G204: fixed URL, not user-controlled
		cmd := exec.Command("git", "clone", demoRepoURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
	}
	if err := os.Chdir(demoDirName); err != nil {
		return fmt.Errorf("change to demo directory: %w", err)
	}
	return nil
}

// exit codes for tiki exec
const (
	exitOK             = 0
	exitInternal       = 1
	exitUsage          = 2
	exitStartupFailure = 3
	exitQueryError     = 4
)

// runExec implements `tiki exec '<statement>'`. Returns an exit code.
func runExec(args []string) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintln(os.Stderr, "usage: tiki exec '<ruki-statement>'")
		return exitUsage
	}

	if err := bootstrap.EnsureGitRepo(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitStartupFailure
	}

	if !config.IsProjectInitialized() {
		_, _ = fmt.Fprintln(os.Stderr, "error: project not initialized: run 'tiki init' first")
		return exitStartupFailure
	}

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		return exitStartupFailure
	}

	bootstrap.InitCLILogging(cfg)

	if err := config.InstallDefaultWorkflow(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: install default workflow: %v\n", err)
	}

	if err := config.LoadStatusRegistry(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: load status registry: %v\n", err)
		return exitStartupFailure
	}

	gate := service.BuildGate()

	_, taskStore, err := bootstrap.InitStores()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: initialize store: %v\n", err)
		return exitStartupFailure
	}
	gate.SetStore(taskStore)

	if err := rukiRuntime.RunQuery(gate, args[0], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		return exitQueryError
	}

	return exitOK
}

// printUsage prints usage information when tiki is run in an uninitialized repo.
func printUsage() {
	fmt.Print(`tiki - Terminal-based task and documentation management

Usage:
  tiki                       Launch TUI in initialized repo
  tiki init                  Initialize project in current git repo
  tiki exec '<statement>'    Execute a ruki query and exit
  tiki demo                  Clone demo project and launch TUI
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
