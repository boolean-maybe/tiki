package runtime

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/workflow"
)

func initTestRegistries() {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{Name: "status", Type: workflow.TypeEnum, EnumValues: []workflow.EnumValue{
			{Value: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
			{Value: "ready", Label: "Ready", Emoji: "📋"},
			{Value: "inProgress", Label: "In Progress", Emoji: "⚙️"},
			{Value: "done", Label: "Done", Emoji: "✅"},
		}},
		{Name: "type", Type: workflow.TypeEnum, EnumValues: []workflow.EnumValue{
			{Value: "story", Label: "Story", Default: true},
			{Value: "bug", Label: "Bug"},
			{Value: "spike", Label: "Spike"},
			{Value: "epic", Label: "Epic"},
		}},
		{Name: "priority", Type: workflow.TypeEnum, EnumValues: []workflow.EnumValue{
			{Value: "high", Label: "High", Emoji: "🔴"},
			{Value: "medium-high", Label: "Medium High", Emoji: "🟠"},
			{Value: "medium", Label: "Medium", Emoji: "🟡", Default: true},
			{Value: "medium-low", Label: "Medium Low", Emoji: "🟢"},
			{Value: "low", Label: "Low", Emoji: "🔵"},
		}},
		{Name: "points", Type: workflow.TypeInt},
		{Name: "tags", Type: workflow.TypeListString},
		{Name: "dependsOn", Type: workflow.TypeListRef},
		{Name: "due", Type: workflow.TypeDate},
		{Name: "recurrence", Type: workflow.TypeRecurrence},
		{Name: "assignee", Type: workflow.TypeString},
	})
}

func TestSchemaFieldMapping(t *testing.T) {
	initTestRegistries()
	s := NewSchema()

	tests := []struct {
		name     string
		wantType ruki.ValueType
	}{
		{"id", ruki.ValueID},
		{"title", ruki.ValueString},
		{"status", ruki.ValueEnum},
		{"type", ruki.ValueEnum},
		{"tags", ruki.ValueListString},
		{"dependsOn", ruki.ValueListRef},
		{"due", ruki.ValueDate},
		{"priority", ruki.ValueEnum},
		{"createdAt", ruki.ValueTimestamp},
		{"updatedAt", ruki.ValueTimestamp},
		{"recurrence", ruki.ValueRecurrence},
		{"filepath", ruki.ValueString},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := s.Field(tt.name)
			if !ok {
				t.Fatalf("Field(%q) not found", tt.name)
				return
			}
			if spec.Type != tt.wantType {
				t.Errorf("Field(%q).Type = %d, want %d", tt.name, spec.Type, tt.wantType)
			}
			if spec.Name != tt.name {
				t.Errorf("Field(%q).Name = %q, want %q", tt.name, spec.Name, tt.name)
			}
		})
	}
}

func TestSchemaUnknownField(t *testing.T) {
	initTestRegistries()
	s := NewSchema()

	_, ok := s.Field("nonexistent")
	if ok {
		t.Error("Field(nonexistent) should return false")
	}
}

func TestSchemaEnumAllowedValues(t *testing.T) {
	initTestRegistries()
	s := NewSchema()

	status, ok := s.Field("status")
	if !ok {
		t.Fatal("status field not found")
	}
	wantStatus := []string{"backlog", "ready", "inProgress", "done"}
	if !equalStrings(status.AllowedValues, wantStatus) {
		t.Fatalf("status AllowedValues = %v, want %v", status.AllowedValues, wantStatus)
	}

	typ, ok := s.Field("type")
	if !ok {
		t.Fatal("type field not found")
	}
	wantType := []string{"story", "bug", "spike", "epic"}
	if !equalStrings(typ.AllowedValues, wantType) {
		t.Fatalf("type AllowedValues = %v, want %v", typ.AllowedValues, wantType)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMapValueTypeCompleteness(t *testing.T) {
	// status, task type, and custom enum all share TypeEnum, which maps to ValueEnum.
	types := []workflow.ValueType{
		workflow.TypeString, workflow.TypeInt, workflow.TypeDate,
		workflow.TypeTimestamp, workflow.TypeDuration, workflow.TypeBool,
		workflow.TypeID, workflow.TypeRef, workflow.TypeRecurrence,
		workflow.TypeListString, workflow.TypeListRef,
		workflow.TypeEnum,
	}

	for _, wt := range types {
		if got := mapValueType(wt); got == ruki.ValueString && wt != workflow.TypeString {
			t.Errorf("mapValueType(%d) fell back to ValueString", wt)
		}
	}
}

func TestMapValueTypeUnknownFallback(t *testing.T) {
	got := mapValueType(workflow.ValueType(999))
	if got != ruki.ValueString {
		t.Errorf("expected fallback to ValueString, got %d", got)
	}
}
