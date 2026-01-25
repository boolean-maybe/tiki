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

	// Initialize paths early - this must succeed for the application to function
	if err := config.InitPaths(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// Handle viewer mode (standalone markdown viewer)
	viewerInput, runViewer, err := viewer.ParseViewerInput(os.Args[1:], map[string]struct{}{})
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

	// Bootstrap application
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
