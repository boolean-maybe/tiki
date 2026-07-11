package tikidetail

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestResolvedTypeUI(t *testing.T) {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{Name: "deadline", Type: workflow.TypeDate},
		{Name: "dueBy", Type: workflow.TypeTimestamp},
		{Name: "note", Type: workflow.TypeString},
		{Name: "reviewer", Type: workflow.TypeUser},
		{Name: "assignee", Type: workflow.TypeString},
	})
	t.Cleanup(teststatuses.Init)

	cases := []struct {
		name   string
		wantOK bool
	}{
		{"deadline", true},          // catalog-only date
		{"dueBy", true},             // catalog-only timestamp
		{"note", true},              // catalog-only text
		{"reviewer", true},          // catalog-only user
		{"assignee", true},          // former user-field name follows workflow type
		{"nope-nonexistent", false}, // unknown field
	}
	for _, c := range cases {
		ui, ok := resolvedTypeUI(c.name)
		if ok != c.wantOK {
			t.Fatalf("%s: ok=%v want %v", c.name, ok, c.wantOK)
		}
		if ok && ui.Render == nil {
			t.Fatalf("%s: resolved TypeUI has nil Render", c.name)
		}
	}
	if !fieldIsReadOnly("createdAt") {
		t.Fatal("createdAt should be read-only")
	}
	if fieldIsReadOnly("deadline") {
		t.Fatal("date field should not be read-only")
	}
	ui, ok := resolvedTypeUI("assignee")
	if !ok {
		t.Fatal("assignee should resolve")
	}
	want, ok := LookupType(SemanticText)
	if !ok {
		t.Fatal("SemanticText should be registered")
	}
	if ui.Edit == nil || want.Edit == nil || ui.Edit == nil && want.Edit != nil {
		t.Fatal("unexpected nil edit factory")
	}
	if gotSem := semanticForValueType(workflow.TypeUser); gotSem != SemanticUser {
		t.Fatalf("semanticForValueType(TypeUser) = %q, want %q", gotSem, SemanticUser)
	}
}
