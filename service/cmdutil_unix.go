//go:build !windows

package service

import (
	"os/exec"
	"syscall"
)

// setProcessGroup configures the command to run in its own process group
// and overrides Cancel to kill the entire group (parent + children).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// negative pid → kill the whole process group
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
