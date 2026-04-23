package gogit

import (
	"fmt"
	"io"

	gogitlib "github.com/go-git/go-git/v5"
)

// Init initializes a new git repository at the given directory.
func Init(dir string) error {
	if _, err := gogitlib.PlainInit(dir, false); err != nil {
		return fmt.Errorf("git init %q: %w", dir, err)
	}
	return nil
}

// Clone clones a git repository from url into dir.
func Clone(url, dir string, _, stderr io.Writer) error {
	_, err := gogitlib.PlainClone(dir, false, &gogitlib.CloneOptions{
		URL:      url,
		Progress: stderr,
	})
	if err != nil {
		return fmt.Errorf("git clone %q: %w", url, err)
	}
	return nil
}
