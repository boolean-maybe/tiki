//go:build windows

package service

import "os/exec"

// setProcessGroup is a no-op on Windows.
// Windows has no process-group kill equivalent to Unix's kill(-pgid).
// cmd.WaitDelay (set by the caller) bounds the pipe drain if children outlive the parent.
func setProcessGroup(_ *exec.Cmd) {}
