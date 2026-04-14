package service

import (
	"context"
	"log/slog"
	"os/exec"
	"time"
)

// shellCommandTimeout is the timeout for shell commands executed by pipes and triggers.
const shellCommandTimeout = 30 * time.Second

// RunShellCommand executes a shell command via "sh -c" with optional positional args.
// When args are provided, "_" occupies $0 and args land in $1, $2, etc.
// This is standard POSIX shell behavior for "sh -c <script> <$0> <$1> ...".
func RunShellCommand(ctx context.Context, cmdStr string, args ...string) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, shellCommandTimeout)
	defer cancel()
	argv := []string{"-c", cmdStr}
	if len(args) > 0 {
		argv = append(argv, "_")     // $0 placeholder
		argv = append(argv, args...) // $1, $2, ...
	}
	cmd := exec.CommandContext(runCtx, "sh", argv...) //nolint:gosec // cmdStr is a user-configured action, intentionally dynamic
	setProcessGroup(cmd)
	cmd.WaitDelay = 3 * time.Second
	return cmd.CombinedOutput()
}

// ExecutePipeCommand runs a pipe run() command with positional args for one row.
// Errors are logged and returned; callers decide whether to continue with remaining rows.
func ExecutePipeCommand(ctx context.Context, cmdStr string, args []string) error {
	output, err := RunShellCommand(ctx, cmdStr, args...)
	if err != nil {
		slog.Error("pipe run() command failed", "command", cmdStr, "args", args, "output", string(output), "error", err)
		return err
	}
	slog.Debug("pipe run() command succeeded", "command", cmdStr, "args", args)
	return nil
}
