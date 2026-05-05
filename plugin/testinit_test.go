package plugin

import (
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
)

func init() {
	teststatuses.Init()
	// The view layer publishes the renderable-metadata field set at its own
	// init. Plugin-package tests don't import view/taskdetail, so seed the
	// canonical schema-known fields here so detail metadata validation can
	// run without dragging the view package into the plugin import graph.
	for _, name := range []string{
		tikipkg.FieldStatus,
		tikipkg.FieldType,
		tikipkg.FieldPriority,
		tikipkg.FieldPoints,
		tikipkg.FieldAssignee,
		tikipkg.FieldDue,
		tikipkg.FieldRecurrence,
		tikipkg.FieldTags,
		tikipkg.FieldDependsOn,
	} {
		RegisterRenderableMetadataField(name)
	}
}
