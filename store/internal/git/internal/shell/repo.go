package shell

import (
	"fmt"
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
