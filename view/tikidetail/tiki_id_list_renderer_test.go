package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestRenderConfiguredFieldUsesDeclaredTypeForFormerTikiIDListName(t *testing.T) {
	fields := teststatuses.CanonicalFields()
	for i := range fields {
		if fields[i].Name == "dependsOn" {
			fields[i].Type = workflow.TypeString
		}
	}
	config.ResetWorkflowFieldsForTest(fields)
	t.Cleanup(teststatuses.Init)

	tikiStore := store.NewInMemoryStore()
	target := tikipkg.New()
	target.SetID("AAAAAA")
	target.SetTitle("resolved title")
	if err := tikiStore.CreateTiki(target); err != nil {
		t.Fatalf("create target: %v", err)
	}

	tk := tikipkg.New()
	tk.Set("dependsOn", "AAAAAA")
	rendered := renderConfiguredField(
		"dependsOn",
		tk,
		FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles(), Store: tikiStore},
	)
	text := drawPrimitive(t, rendered, 40, 1)
	if !strings.Contains(text, "AAAAAA") {
		t.Fatalf("rendered text %q does not contain field value", text)
	}
	if strings.Contains(text, target.Title()) {
		t.Fatalf("string field used tiki-id-list rendering: %q", text)
	}
}
