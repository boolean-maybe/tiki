package tikistore

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestTikiIDListFieldRejectsMissingTarget(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "related", Type: workflow.TypeListRef},
	}); err != nil {
		t.Fatalf("register fields: %v", err)
	}
	t.Cleanup(teststatuses.Init)
	t.Setenv("HOME", t.TempDir())

	s, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	tk := tikipkg.New()
	tk.SetID("BBBBBB")
	tk.SetTitle("orphan")
	tk.Set("related", []string{"ZZZZZZ"})

	err = s.CreateTiki(tk)
	if err == nil {
		t.Fatal("expected missing-target error for tikiIdList field")
	}
	if !strings.Contains(err.Error(), "related") || !strings.Contains(err.Error(), "non-existent document") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormerTikiIDListFieldNameUsesDeclaredType(t *testing.T) {
	fields := teststatuses.CanonicalFields()
	for i := range fields {
		if fields[i].Name == "dependsOn" {
			fields[i].Type = workflow.TypeString
			fields[i].DefaultValue = nil
		}
	}
	config.ResetWorkflowFieldsForTest(fields)
	t.Cleanup(teststatuses.Init)
	t.Setenv("HOME", t.TempDir())

	s, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	tk := tikipkg.New()
	tk.SetID("BBBBBB")
	tk.SetTitle("plain string")
	tk.Set("dependsOn", "ZZZZZZ")

	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("string field was reference-validated: %v", err)
	}
}

// TestTikiIDListFieldRejectsNonBareID proves that reference validation rejects
// legacy TIKI-prefixed IDs. Bare IDs are the only valid form.
func TestTikiIDListFieldRejectsNonBareID(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "related", Type: workflow.TypeListRef},
	}); err != nil {
		t.Fatalf("register fields: %v", err)
	}
	t.Cleanup(teststatuses.Init)
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	// seed a valid target so the only possible failure is the format check.
	target := tikipkg.New()
	target.SetID("AAAAAA")
	target.SetTitle("target")
	target.Set("status", "ready")
	if err := s.CreateTiki(target); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	dependent := tikipkg.New()
	dependent.SetID("BBBBBB")
	dependent.SetTitle("dependent")
	dependent.Set("status", "ready")
	dependent.Set("related", []string{"TIKI-AAA"})
	err = s.CreateTiki(dependent)
	if err == nil {
		t.Fatal("expected error for non-bare reference id")
	}
	if !strings.Contains(err.Error(), "bare document id") {
		t.Fatalf("expected bare-id error, got: %v", err)
	}
}

// TestTikiIDListFieldAcceptsBareID proves a bare-ID reference resolves to any
// existing document, regardless of workflow status.
func TestTikiIDListFieldAcceptsBareID(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "related", Type: workflow.TypeListRef},
	}); err != nil {
		t.Fatalf("register fields: %v", err)
	}
	t.Cleanup(teststatuses.Init)
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()

	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	target := tikipkg.New()
	target.SetID("AAAAAA")
	target.SetTitle("target")
	target.Set("status", "ready")
	if err := s.CreateTiki(target); err != nil {
		t.Fatalf("seed: %v", err)
	}

	dependent := tikipkg.New()
	dependent.SetID("BBBBBB")
	dependent.SetTitle("dependent")
	dependent.Set("status", "ready")
	dependent.Set("related", []string{"AAAAAA"})
	if err := s.CreateTiki(dependent); err != nil {
		t.Fatalf("create dependent: %v", err)
	}
}
