package tikistore

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/workflow"
)

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
	original := []string{"a", "b"}
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
