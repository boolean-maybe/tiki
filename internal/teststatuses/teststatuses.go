package teststatuses

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/workflow"
)

// CanonicalFields returns the canonical test workflow field set (status, type,
// priority, points, tags, dependsOn, due, recurrence, assignee).
func CanonicalFields() []workflow.FieldDef {
	return []workflow.FieldDef{
		{
			Name: "status",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
				{Value: "ready", Label: "Ready", Emoji: "📋"},
				{Value: "inProgress", Label: "In Progress", Emoji: "⚙️"},
				{Value: "review", Label: "Review", Emoji: "👀"},
				{Value: "done", Label: "Done", Emoji: "✅"},
			},
		},
		{
			Name: "type",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "story", Label: "Story", Emoji: "🌀", Default: true},
				{Value: "bug", Label: "Bug", Emoji: "💥"},
				{Value: "spike", Label: "Spike", Emoji: "🔍"},
				{Value: "epic", Label: "Epic", Emoji: "🗂️"},
			},
		},
		{
			Name: "priority",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "high", Label: "High", Emoji: "🔴"},
				{Value: "medium-high", Label: "Medium High", Emoji: "🟠"},
				{Value: "medium", Label: "Medium", Emoji: "🟡", Default: true},
				{Value: "medium-low", Label: "Medium Low", Emoji: "🟢"},
				{Value: "low", Label: "Low", Emoji: "🔵"},
			},
		},
		{Name: "points", Type: workflow.TypeInt, DefaultValue: 1},
		{Name: "tags", Type: workflow.TypeListString, DefaultValue: []string{"idea"}},
		{Name: "dependsOn", Type: workflow.TypeListRef},
		{Name: "due", Type: workflow.TypeDate},
		{Name: "recurrence", Type: workflow.TypeRecurrence},
		{Name: "assignee", Type: workflow.TypeString},
	}
}

// InitWith registers the canonical test fields plus any extras (typically
// custom fields used by a single test). Use when a test needs additional
// fields beyond the canonical set without losing status/type.
func InitWith(extra []workflow.FieldDef) error {
	all := append(CanonicalFields(), extra...)
	if err := workflow.RegisterWorkflowFields(all); err != nil {
		return err
	}
	config.MarkWorkflowFieldsLoadedForTest()
	return nil
}

// Init registers the canonical test workflow field set: status, type,
// priority, points, tags, dependsOn, due, recurrence, assignee — matching
// the bundled kanban.yaml. Call from init() in each package's
// testinit_test.go.
func Init() {
	config.ResetWorkflowFieldsForTest(CanonicalFields())
}
