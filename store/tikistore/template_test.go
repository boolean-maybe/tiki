package tikistore

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

// TestSetAuthorFromIdentity_EmailOnly_NoAngleEchoing reproduces a regression
// from the user()/display-string promotion work: when only identity.email is
// configured, currentUser returned (email, email) so author formatting
// produced `me@example.com <me@example.com>`. Attribution should just be the
// raw email in that case.
func TestSetAuthorFromIdentity_EmailOnly_NoAngleEchoing(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "", "me@example.com")

	// build a store with no git util so attribution flows through the
	// identity resolver's config layer only
	s := &TikiStore{identity: newIdentityResolver(nil)}

	task := &taskpkg.Task{}
	s.setAuthorFromIdentity(task)

	if task.CreatedBy != "me@example.com" {
		t.Errorf("CreatedBy = %q, want 'me@example.com' (no angle form when only email is configured)", task.CreatedBy)
	}
}

func TestSetAuthorFromIdentity_NameAndEmail_UsesAngleForm(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "Alice", "alice@example.com")

	s := &TikiStore{identity: newIdentityResolver(nil)}

	task := &taskpkg.Task{}
	s.setAuthorFromIdentity(task)

	if task.CreatedBy != "Alice <alice@example.com>" {
		t.Errorf("CreatedBy = %q, want 'Alice <alice@example.com>'", task.CreatedBy)
	}
}

func TestSetAuthorFromIdentity_NameOnly_UsesName(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "Alice", "")

	s := &TikiStore{identity: newIdentityResolver(nil)}

	task := &taskpkg.Task{}
	s.setAuthorFromIdentity(task)

	if task.CreatedBy != "Alice" {
		t.Errorf("CreatedBy = %q, want 'Alice'", task.CreatedBy)
	}
}

func TestBuildCustomFieldDefaults_NoCustomFields(t *testing.T) {
	workflow.ClearCustomFields()
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	defaults := buildCustomFieldDefaults()
	if defaults != nil {
		t.Errorf("expected nil map when no custom fields, got %v", defaults)
	}
}

func TestBuildCustomFieldDefaults_WithDefaults(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium", "high"}, DefaultValue: "medium"},
		{Name: "blocked", Type: workflow.TypeBool, DefaultValue: false},
		{Name: "notes", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	defaults := buildCustomFieldDefaults()
	if defaults == nil {
		t.Fatal("expected non-nil defaults map")
	}
	if v, ok := defaults["severity"]; !ok || v != "medium" {
		t.Errorf("severity = %v, want \"medium\"", v)
	}
	if v, ok := defaults["blocked"]; !ok || v != false {
		t.Errorf("blocked = %v, want false", v)
	}
	if _, ok := defaults["notes"]; ok {
		t.Error("notes should not have a default")
	}
}

func TestBuildCustomFieldDefaults_SliceDefaultCopied(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	original := []string{"a", "b", " a ", "b", ""}
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "labels", Type: workflow.TypeListString, DefaultValue: original},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	defaults := buildCustomFieldDefaults()
	got, ok := defaults["labels"].([]string)
	if !ok {
		t.Fatal("expected []string default")
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("labels = %v, want [a b]", got)
	}
	// mutating the returned slice should not affect the original
	got[0] = "mutated"
	if original[0] == "mutated" {
		t.Error("returned slice should be a copy, not the original")
	}
}
