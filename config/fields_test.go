package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/workflow"
)

// setupLoadCustomFieldsTest creates temp dirs and configures the path manager
// so LoadCustomFields can discover workflow.yaml files.
func setupLoadCustomFieldsTest(t *testing.T) (cwdDir string) {
	t.Helper()
	workflow.ClearCustomFields()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir()
	docDir := filepath.Join(projectDir, ".doc")
	if err := os.MkdirAll(docDir, 0750); err != nil {
		t.Fatal(err)
	}

	cwdDir = t.TempDir()
	originalDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	_ = os.Chdir(cwdDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = projectDir

	return cwdDir
}

func TestLoadCustomFields_BasicTypes(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)

	content := `
fields:
  - name: notes
    type: text
  - name: score
    type: integer
  - name: active
    type: boolean
  - name: startedAt
    type: datetime
  - name: labels
    type: stringList
  - name: related
    type: taskIdList
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := LoadCustomFields(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		name     string
		wantType workflow.ValueType
	}{
		{"notes", workflow.TypeString},
		{"score", workflow.TypeInt},
		{"active", workflow.TypeBool},
		{"startedAt", workflow.TypeTimestamp},
		{"labels", workflow.TypeListString},
		{"related", workflow.TypeListRef},
	}
	for _, c := range checks {
		f, ok := workflow.Field(c.name)
		if !ok {
			t.Errorf("Field(%q) not found", c.name)
			continue
		}
		if f.Type != c.wantType {
			t.Errorf("Field(%q).Type = %v, want %v", c.name, f.Type, c.wantType)
		}
		if !f.Custom {
			t.Errorf("Field(%q).Custom = false, want true", c.name)
		}
	}
}

func TestLoadCustomFields_EnumWithValues(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)

	content := `
fields:
  - name: severity
    type: enum
    values: [low, medium, high, critical]
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := LoadCustomFields(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f, ok := workflow.Field("severity")
	if !ok {
		t.Fatal("severity field not found")
	}
	if f.Type != workflow.TypeEnum {
		t.Errorf("severity.Type = %v, want TypeEnum", f.Type)
	}
	wantVals := []string{"low", "medium", "high", "critical"}
	if len(f.AllowedValues) != len(wantVals) {
		t.Fatalf("severity.AllowedValues length = %d, want %d", len(f.AllowedValues), len(wantVals))
	}
	for i, v := range wantVals {
		if f.AllowedValues[i] != v {
			t.Errorf("AllowedValues[%d] = %q, want %q", i, f.AllowedValues[i], v)
		}
	}
}

func TestLoadCustomFields_BadTypeRejected(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)

	content := `
fields:
  - name: broken
    type: nosuchtype
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := LoadCustomFields()
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestLoadCustomFields_EnumWithoutValues(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)

	content := `
fields:
  - name: severity
    type: enum
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := LoadCustomFields()
	if err == nil {
		t.Fatal("expected error for enum without values")
	}
}

func TestLoadCustomFields_ConflictingRedefinition(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)
	pm := mustGetPathManager()

	// write field definition in project config
	projectWorkflow := filepath.Join(pm.ProjectConfigDir(), "workflow.yaml")
	content1 := `
fields:
  - name: score
    type: integer
`
	if err := os.WriteFile(projectWorkflow, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}

	// write conflicting definition in cwd
	content2 := `
fields:
  - name: score
    type: text
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	err := LoadCustomFields()
	if err == nil {
		t.Fatal("expected error for conflicting redefinition")
	}
}

func TestLoadCustomFields_IdenticalRedefinition(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)
	pm := mustGetPathManager()

	// write identical definitions in two locations
	content := `
fields:
  - name: score
    type: integer
`
	projectWorkflow := filepath.Join(pm.ProjectConfigDir(), "workflow.yaml")
	if err := os.WriteFile(projectWorkflow, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := LoadCustomFields(); err != nil {
		t.Fatalf("identical redefinition should succeed: %v", err)
	}

	f, ok := workflow.Field("score")
	if !ok {
		t.Fatal("score field not found")
	}
	if f.Type != workflow.TypeInt {
		t.Errorf("score.Type = %v, want TypeInt", f.Type)
	}
}
