package tikistore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestLoadTemplateTask_CwdWins(t *testing.T) {
	// user config with priority 1
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	userTikiDir := filepath.Join(userDir, "tiki")
	if err := os.MkdirAll(userTikiDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userTikiDir, "new.md"),
		[]byte("---\ntitle: user\npriority: 1\ntype: story\nstatus: backlog\n---"), 0644); err != nil {
		t.Fatal(err)
	}

	// cwd with priority 5 (should win)
	cwdDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwdDir, "new.md"),
		[]byte("---\ntitle: cwd\npriority: 5\ntype: bug\nstatus: backlog\n---"), 0644); err != nil {
		t.Fatal(err)
	}

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	config.ResetPathManager()

	task, err := loadTemplateTask()
	if err != nil {
		t.Fatalf("loadTemplateTask() error: %v", err)
	}
	if task == nil {
		t.Fatal("loadTemplateTask() returned nil")
		return
	}
	if task.Title != "cwd" {
		t.Errorf("title = %q, want \"cwd\"", task.Title)
	}
	if task.Priority != 5 {
		t.Errorf("priority = %d, want 5", task.Priority)
	}
}

func TestLoadTemplateTask_EmbeddedFallback(t *testing.T) {
	// no new.md anywhere
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	cwdDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(cwdDir)

	config.ResetPathManager()

	task, err := loadTemplateTask()
	if err != nil {
		t.Fatalf("loadTemplateTask() error: %v", err)
	}
	if task == nil {
		t.Fatal("loadTemplateTask() returned nil, expected embedded template")
	}
	// embedded template has type: story and status: backlog
	if task.Type != "story" {
		t.Errorf("type = %q, want \"story\" from embedded template", task.Type)
	}
	if task.Status != "backlog" {
		t.Errorf("status = %q, want \"backlog\" from embedded template", task.Status)
	}
}

func TestParseTaskTemplate_BadCustomFieldDemotedToUnknown(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	// register an int custom field so we can provoke a coercion failure
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "score", Type: workflow.TypeInt},
	}); err != nil {
		t.Fatalf("register custom fields: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	// template has valid built-in fields AND an invalid custom field (string for int)
	tmpl := []byte("---\ntitle: keep me\npriority: 3\ntype: bug\nstatus: backlog\nscore: not_a_number\n---\nsome description")

	task, err := parseTaskTemplate(tmpl)
	if err != nil {
		t.Fatalf("template should load despite bad custom field, got: %v", err)
	}
	if task == nil {
		t.Fatal("expected non-nil task")
	}
	// built-in fields should still be populated
	if task.Title != "keep me" {
		t.Errorf("title = %q, want %q", task.Title, "keep me")
	}
	// bad custom field value should not appear in CustomFields
	if task.CustomFields != nil {
		if _, exists := task.CustomFields["score"]; exists {
			t.Error("bad custom field value should not appear in CustomFields")
		}
	}
}
