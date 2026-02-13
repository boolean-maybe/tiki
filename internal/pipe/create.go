package pipe

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/bootstrap"
)

// IsPipedInput reports whether stdin is connected to a pipe or redirected file
// rather than a terminal (character device).
func IsPipedInput() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice == 0
}

// HasPositionalArgs reports whether args contains any non-flag positional arguments.
// It skips known flag-value pairs (e.g. --log-level debug) so that only real
// positional arguments like file paths, "-", "init", etc. are detected.
func HasPositionalArgs(args []string) bool {
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--" {
			return true // everything after "--" is positional
		}
		// Bare "-" means "read stdin for the viewer", treat as positional
		if arg == "-" {
			return true
		}
		if strings.HasPrefix(arg, "-") {
			if arg == "--log-level" {
				skipNext = true
			}
			continue
		}
		return true
	}
	return false
}

// CreateTaskFromReader reads piped input, parses it into title/description,
// and creates a new tiki task. Returns the task ID (e.g. "TIKI-ABC123").
func CreateTaskFromReader(r io.Reader) (string, error) {
	// Suppress info/debug logs for the non-interactive pipe path.
	// The pipe path bypasses bootstrap (which normally configures logging),
	// so the default slog handler would write INFO+ messages to stderr.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	})))

	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}

	input := strings.TrimSpace(string(data))
	if input == "" {
		return "", fmt.Errorf("empty input: title is required")
	}

	title, description := parseInput(input)

	if err := bootstrap.EnsureGitRepo(); err != nil {
		return "", err
	}

	if !config.IsProjectInitialized() {
		return "", fmt.Errorf("project not initialized: run 'tiki init' first")
	}

	tikiStore, _, err := bootstrap.InitStores()
	if err != nil {
		return "", fmt.Errorf("initialize store: %w", err)
	}

	task, err := tikiStore.NewTaskTemplate()
	if err != nil {
		return "", fmt.Errorf("create task template: %w", err)
	}

	task.Title = title
	task.Description = description

	if errs := task.Validate(); errs.HasErrors() {
		return "", fmt.Errorf("validation failed: %s", errs.Error())
	}

	if err := tikiStore.CreateTask(task); err != nil {
		return "", fmt.Errorf("create task: %w", err)
	}

	return task.ID, nil
}

// parseInput splits piped text into title and description.
// Single line: entire input becomes the title, no description.
// Multi-line: first line is the title, everything after is the description (trimmed).
func parseInput(input string) (title, description string) {
	first, rest, found := strings.Cut(input, "\n")
	if !found {
		return strings.TrimSpace(input), ""
	}
	return strings.TrimSpace(first), strings.TrimSpace(rest)
}
