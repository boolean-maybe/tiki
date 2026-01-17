package bootstrap

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/config"
)

// InitLogging sets up application logging with the configured log level.
// It opens a log file next to the executable (tiki.log) or falls back to stderr.
// Returns the configured log level.
func InitLogging(cfg *config.Config) slog.Level {
	logOutput := openLogOutput()
	logLevel := parseLogLevel(cfg.Logging.Level)
	logger := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)
	slog.Info("application starting up", "log_level", logLevel.String())
	return logLevel
}

// openLogOutput opens the configured log output destination, falling back to stderr.
func openLogOutput() *os.File {
	logOutput := os.Stderr
	exePath, err := os.Executable()
	if err != nil {
		return logOutput
	}

	logPath := filepath.Join(filepath.Dir(exePath), "tiki.log")
	//nolint:gosec // G302: 0644 is appropriate for log files
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return logOutput
	}
	// Let the OS close the file on exit
	return file
}

// parseLogLevel parses the configured log level string into slog.Level.
func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
