package pipe

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
)

// TestCreateInBareDirSucceeds verifies that creating a tiki from piped input in
// a fresh, non-initialized, non-repo directory succeeds — there is no longer an
// "init" precondition gate. The embedded default workflow supplies statuses, so
// the result is a workflow tiki with a generated id.
func TestCreateInBareDirSucceeds(t *testing.T) {
	// isolate config resolution from the developer's real ~/.config/tiki so the
	// workflow lookup falls through to the embedded default, not a stale local file.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Chdir(t.TempDir())
	config.ResetPathManager()
	if err := config.InitPaths(); err != nil {
		t.Fatalf("init paths: %v", err)
	}
	// seed the user-config default workflow, as every normal launch does; this
	// is independent of the init/git gate removal under test here.
	if err := config.InstallDefaultWorkflow(); err != nil {
		t.Fatalf("install default workflow: %v", err)
	}

	id, err := CreateTikiFromReader(strings.NewReader("My task\nsome body\n"))
	if err != nil {
		t.Fatalf("create failed in bare dir: %v", err)
	}
	if id == "" {
		t.Fatal("expected a new id")
	}
}
