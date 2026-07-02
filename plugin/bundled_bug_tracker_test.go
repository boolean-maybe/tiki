package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/gridlayout"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/workflow"
)

// TestBundledBugTracker_DetailUsesFieldCaptions guards the bug-tracker Detail
// view against the orphaned-value regression. The view originally used literal
// caption cells for every field ("Status:", "Author:", "Severity:", …), which
// carry no field name and so do not participate in caption↔value co-shedding
// (see CLAUDE.md). At narrow widths a literal caption can shed while its value
// survives, leaving an unlabeled column. Converting them to field.caption cells
// makes each field appear as exactly one DisplayCaption anchor paired with one
// value anchor, so the solver sheds the pair together.
//
// We assert on a representative mix: system fields whose captions resolve via
// the registry (createdBy/createdAt/updatedAt) and custom fields whose captions
// are declared in the workflow `fields:` list (severity/environment/reportedBy).
func TestBundledBugTracker_DetailUsesFieldCaptions(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	src := filepath.Join(filepath.Dir(wd), "config", "workflows", "bug-tracker.yaml")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("bundled bug-tracker not at expected path %s: %v", src, err)
	}
	// the loader validates layout field refs against a ruki schema, so the
	// schema must carry bug-tracker's own custom fields (severity, environment,
	// …) plus the system fields (id/createdAt/createdBy/updatedAt) — testSchema()
	// only knows the kanban field set and would reject them. Mirror what startup
	// assembles (SystemFields ∪ workflow fields) without touching global state.
	customFields, err := config.LoadWorkflowFieldsFromFile(src)
	if err != nil {
		t.Fatalf("load bug-tracker fields: %v", err)
	}
	schema := rukiRuntime.NewSchemaFromFields(append(workflow.SystemFields(), customFields...))
	plugins, _, errs := loadPluginsFromFile(src, schema)
	if len(errs) != 0 {
		t.Fatalf("bundled bug-tracker did not load cleanly: %v", errs)
	}
	var detail *DetailPlugin
	for _, p := range plugins {
		if dp, ok := p.(*DetailPlugin); ok && dp.Name == "Detail" {
			detail = dp
			break
		}
	}
	if detail == nil {
		t.Fatal("bundled bug-tracker does not contain a Detail view")
	}

	// each of these is a plain scalar field with a standalone value cell, so it
	// must resolve to exactly one caption anchor and one value anchor. (status
	// and type live inside composites, so they are not standalone AnchorFields —
	// excluded here for the same reason the kanban Detail test asserts on
	// priority rather than status.)
	fields := []string{
		"createdBy", "createdAt", "updatedAt", // system, registry-labelled
		"severity", "environment", "reportedBy", // custom, workflow-declared captions
	}
	for _, field := range fields {
		var captionAnchors, valueAnchors int
		for _, a := range detail.Layout.Anchors {
			if a.Kind != gridlayout.AnchorField || a.Name != field {
				continue
			}
			if a.Display == gridlayout.DisplayCaption {
				captionAnchors++
				continue
			}
			valueAnchors++
		}
		if captionAnchors != 1 || valueAnchors != 1 {
			t.Errorf("%s: caption anchors=%d value anchors=%d, want 1 and 1 (field.caption, not a literal)",
				field, captionAnchors, valueAnchors)
		}
	}
}
