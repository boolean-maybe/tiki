package main

import (
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/app"
	"github.com/boolean-maybe/tiki/internal/bootstrap"
	"github.com/boolean-maybe/tiki/internal/viewer"
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

	// Handle sysinfo command
	if len(os.Args) > 1 && os.Args[1] == "sysinfo" {
		if err := runSysInfo(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	// Initialize paths early - this must succeed for the application to function
	if err := config.InitPaths(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// Handle init command
	initRequested := len(os.Args) > 1 && os.Args[1] == "init"

	// Handle viewer mode (standalone markdown viewer)
	// "init" is reserved to prevent treating it as a markdown file
	viewerInput, runViewer, err := viewer.ParseViewerInput(os.Args[1:], map[string]struct{}{"init": {}})
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

// printUsage prints usage information when tiki is run in an uninitialized repo.
func printUsage() {
	fmt.Print(`tiki - Terminal-based task and documentation management

Usage:
  tiki              Launch TUI in initialized repo
  tiki init         Initialize project in current git repo
  tiki file.md/URL  View markdown file
  tiki sysinfo      Display system information
  tiki --version    Show version

Run 'tiki init' to initialize this repository.
`)
}