package shell

import (
	"fmt"
	"io"
	"os/exec"
)

// Init initializes a new git repository at the given directory.
func Init(dir string) error {
	cmd := exec.Command("git", "init", dir) //nolint:gosec // G204: git init with caller-provided directory
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init %q: %s", dir, string(out))
	}
	return nil
}

// Clone clones a git repository from url into dir.
func Clone(url, dir string, stdout, stderr io.Writer) error {
	cmd := exec.Command("git", "clone", url, dir) //nolint:gosec // G204: git clone with caller-provided url and directory
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone %q: %w", url, err)
	}
	return nil
}
