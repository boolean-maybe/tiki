package tikidetail

import (
	"testing"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestResolvedTypeUI(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "dueBy", Type: workflow.TypeTimestamp},
		{Name: "note", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("InitWith: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	cases := []struct {
		name   string
		wantOK bool
	}{
		{"due", true},               // descriptor-backed date
		{"dueBy", true},             // catalog-only timestamp
		{"note", true},              // catalog-only text
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
	if fieldIsReadOnly("due") {
		t.Fatal("due should not be read-only")
	}
}
