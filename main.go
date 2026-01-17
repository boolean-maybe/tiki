package main

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/app"
	"github.com/boolean-maybe/tiki/internal/bootstrap"
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

	// Bootstrap application
	result, err := bootstrap.Bootstrap(tikiSkillMdContent, dokiSkillMdContent)
	if err != nil {
		return
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
	app.Run(result.App, result.RootLayout)

	// Save user preferences on shutdown
	if err := config.SaveHeaderVisible(result.HeaderConfig.GetUserPreference()); err != nil {
		slog.Warn("failed to save header visibility preference", "error", err)
	}

	// Keep logLevel variable referenced so it isn't optimized away in some builds
	_ = result.LogLevel
}
