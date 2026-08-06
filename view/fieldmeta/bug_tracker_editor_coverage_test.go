package fieldmeta_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/view/fieldmeta"
)

// TestBundledBugTracker_EveryFieldHasEditor asserts that every field declared in
// the bundled bug-tracker workflow is editable in the detail view — except the
// one documented deferred stub, dependsOn (tikiIdList → dependency-picker
// follow-up). This is the whole-workflow coverage check behind the generic-field-
// editors work: it loads the ACTUAL embedded workflow (not a hand-built catalog)
// so a new field added to bug-tracker.yaml without a matching editor fails here.
func TestBundledBugTracker_EveryFieldHasEditor(t *testing.T) {
	content, ok := config.LookupEmbeddedWorkflow("bug-tracker")
	if !ok {
		t.Fatal("embedded bug-tracker workflow not found")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(src, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp workflow: %v", err)
	}
	fields, err := config.LoadWorkflowFieldsFromFile(src)
	if err != nil {
		t.Fatalf("load bug-tracker fields: %v", err)
	}
	// LoadWorkflowFieldsFromFile only parses+validates; register the defs so
	// workflow.Field (which fieldmeta.FieldHasEditor reads) resolves them.
	config.ResetWorkflowFieldsForTest(fields)
	t.Cleanup(config.ClearWorkflowFields)

	// dependsOn (tikiIdList) is the one intentional non-editable field — its
	// picker is a documented follow-up. Everything else must be editable.
	const deferredStub = "dependsOn"
	sawStub := false
	for _, fd := range fields {
		editable := fieldmeta.FieldHasEditor(fd.Name)
		if fd.Name == deferredStub {
			sawStub = true
			if editable {
				t.Errorf("%s (%v) unexpectedly editable — the tikiIdList picker is a deferred stub", fd.Name, fd.Type)
			}
			continue
		}
		if !editable {
			t.Errorf("bug-tracker field %q (type %v) has NO editor — every non-stub declared field must be editable", fd.Name, fd.Type)
		}
	}
	if !sawStub {
		t.Fatalf("expected a %q field in bug-tracker.yaml; workflow shape changed — update this test", deferredStub)
	}
}
