package teststatuses

import (
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/theme"
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
				{Value: "inbox", Label: "Inbox", Visual: "📥", Default: true},
				{Value: "ready", Label: "Ready", Visual: "📋"},
				{Value: "inProgress", Label: "In Progress", Visual: "⚙️"},
				{Value: "done", Label: "Done", Visual: "✅"},
			},
		},
		{
			Name: "type",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "story", Label: "Story", Visual: "🌀", Default: true},
				{Value: "bug", Label: "Bug", Visual: "💥"},
				{Value: "spike", Label: "Spike", Visual: "🔍"},
				{Value: "project", Label: "Project", Visual: "🗂️"},
			},
		},
		{
			Name: "priority",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "high", Label: "High", Visual: "🔴"},
				{Value: "medium-high", Label: "Medium High", Visual: "🟠"},
				{Value: "medium", Label: "Medium", Visual: "🟡", Default: true},
				{Value: "medium-low", Label: "Medium Low", Visual: "🟢"},
				{Value: "low", Label: "Low", Visual: "🔵"},
			},
		},
		{
			Name: "points",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "11", Label: "11", Visual: "<accent>❚❚❚❚❚❚❚❚❚❚❚"},
				{Value: "7", Label: "7", Visual: "<accent>❚❚❚❚❚❚❚<muted>❘❘❘❘"},
				{Value: "3", Label: "3", Visual: "<accent>❚❚❚<muted>❘❘❘❘❘❘❘❘", Default: true},
				{Value: "1", Label: "1", Visual: "<accent>❚<muted>❘❘❘❘❘❘❘❘❘❘"},
			},
		},
		{Name: "escalations", Type: workflow.TypeInt, DefaultValue: 0},
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
// the bundled kanban.yaml. Also installs the default "dark" theme so
// theme.Roles() doesn't panic in tests that pull role-derived strings.
// Call from init() in each package's testinit_test.go.
func Init() {
	config.ResetWorkflowFieldsForTest(CanonicalFields())
	theme.SetTheme(theme.LoadByName("dark"))
}
