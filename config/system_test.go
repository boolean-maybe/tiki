package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallDefaultWorkflow_PreservesExisting(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	ResetPathManager()
	t.Cleanup(ResetPathManager)

	configDir := filepath.Join(xdgDir, "tiki")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	existing := `version: "0.6.0"
fields:
  - name: assignee
    type: text
`
	path := GetUserConfigWorkflowFile()
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatalf("write existing workflow: %v", err)
	}

	if err := InstallDefaultWorkflow(); err != nil {
		t.Fatalf("InstallDefaultWorkflow: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if string(got) != existing {
		t.Fatalf("existing workflow was rewritten:\n%s", got)
	}
}
