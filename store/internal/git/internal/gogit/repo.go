package gogit

import (
	"fmt"

	gogitlib "github.com/go-git/go-git/v5"
)

// Init initializes a new git repository at the given directory.
func Init(dir string) error {
	if _, err := gogitlib.PlainInit(dir, false); err != nil {
		return fmt.Errorf("git init %q: %w", dir, err)
	}
	return nil
}
