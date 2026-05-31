package tikidetail

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

func init() {
	teststatuses.Init()
}

// registerExtraWorkflowFieldForTest adds an enum field to the canonical
// test catalog and returns a cleanup func that restores the canonical set.
// The name parameter is currently always "severity" in callers, but kept
// as a parameter so future tests with different custom enum names can
// reuse the helper.
//
//nolint:unparam // name is intentionally configurable for future callers
func registerExtraWorkflowFieldForTest(t *testing.T, name string, values []string) func() {
	t.Helper()
	enum := make([]workflow.EnumValue, len(values))
	for i, v := range values {
		enum[i] = workflow.EnumValue{Value: v}
	}
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: name, Type: workflow.TypeEnum, EnumValues: enum},
	}); err != nil {
		t.Fatalf("registerExtraWorkflowFieldForTest: %v", err)
	}
	return teststatuses.Init
}

// newTestViewTiki returns a fully-populated tiki used by view tests. The
// shape was previously defined alongside the now-deleted legacy detail
// view tests; centralizing it here keeps the configurable-view tests
// compiling without duplicating the construction.
func newTestViewTiki(id string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.SetID(id)
	tk.SetTitle("Test Tiki")
	tk.Set(tikipkg.FieldStatus, "ready")
	tk.Set(tikipkg.FieldType, "story")
	tk.Set(tikipkg.FieldPriority, "medium")
	tk.Set(tikipkg.FieldPoints, 5)
	tk.Set(tikipkg.FieldAssignee, "user@example.com")
	tk.Set("createdBy", "creator@example.com")
	tk.SetCreatedAt(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	tk.SetUpdatedAt(time.Date(2024, 1, 2, 14, 30, 0, 0, time.UTC))
	return tk
}
