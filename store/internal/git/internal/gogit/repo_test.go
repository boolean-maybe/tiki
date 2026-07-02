package gogit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/store/internal/git/internal/gogit"
)

func TestInit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	if err := gogit.Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf(".git not created: %v", err)
	}
}
