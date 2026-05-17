package model

import (
	"reflect"
	"testing"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

func TestNextField(t *testing.T) {
	tests := []struct {
		name     string
		current  EditField
		expected EditField
	}{
		{"Title to Status", EditFieldTitle, EditFieldStatus},
		{"Status to Type", EditFieldStatus, EditFieldType},
		{"Type to Priority", EditFieldType, EditFieldPriority},
		{"Priority to Points", EditFieldPriority, EditFieldPoints},
		{"Points to Assignee", EditFieldPoints, EditFieldAssignee},
		{"Assignee to Due", EditFieldAssignee, EditFieldDue},
		{"Due to Recurrence", EditFieldDue, EditFieldRecurrence},
		{"Recurrence to Tags", EditFieldRecurrence, EditFieldTags},
		{"Tags to Description", EditFieldTags, EditFieldDescription},
		{"Description stays at Description (no wrap)", EditFieldDescription, EditFieldDescription},
		{"Unknown field defaults to Title", EditField("unknown"), EditFieldTitle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NextField(tt.current)
			if result != tt.expected {
				t.Errorf("NextField(%v) = %v, want %v", tt.current, result, tt.expected)
			}
		})
	}
}

func TestPrevField(t *testing.T) {
	tests := []struct {
		name     string
		current  EditField
		expected EditField
	}{
		{"Title stays at Title (no wrap)", EditFieldTitle, EditFieldTitle},
		{"Status to Title", EditFieldStatus, EditFieldTitle},
		{"Type to Status", EditFieldType, EditFieldStatus},
		{"Priority to Type", EditFieldPriority, EditFieldType},
		{"Points to Priority", EditFieldPoints, EditFieldPriority},
		{"Assignee to Points", EditFieldAssignee, EditFieldPoints},
		{"Due to Assignee", EditFieldDue, EditFieldAssignee},
		{"Recurrence to Due", EditFieldRecurrence, EditFieldDue},
		{"Tags to Recurrence", EditFieldTags, EditFieldRecurrence},
		{"Description to Tags", EditFieldDescription, EditFieldTags},
		{"Unknown field defaults to Title", EditField("unknown"), EditFieldTitle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrevField(tt.current)
			if result != tt.expected {
				t.Errorf("PrevField(%v) = %v, want %v", tt.current, result, tt.expected)
			}
		})
	}
}

func TestFieldCycling(t *testing.T) {
	field := EditFieldTitle
	expectedOrder := []EditField{
		EditFieldStatus,
		EditFieldType,
		EditFieldPriority,
		EditFieldPoints,
		EditFieldAssignee,
		EditFieldDue,
		EditFieldRecurrence,
		EditFieldTags,
		EditFieldDescription,
		EditFieldDescription,
	}

	for i, expected := range expectedOrder {
		field = NextField(field)
		if field != expected {
			t.Errorf("Forward navigation step %d: got %v, want %v", i, field, expected)
		}
	}

	field = EditFieldDescription
	expectedOrderReverse := []EditField{
		EditFieldTags,
		EditFieldRecurrence,
		EditFieldDue,
		EditFieldAssignee,
		EditFieldPoints,
		EditFieldPriority,
		EditFieldType,
		EditFieldStatus,
		EditFieldTitle,
		EditFieldTitle,
	}

	for i, expected := range expectedOrderReverse {
		field = PrevField(field)
		if field != expected {
			t.Errorf("Backward navigation step %d: got %v, want %v", i, field, expected)
		}
	}
}

func TestNextFieldSkipping(t *testing.T) {
	skipDue := func(f EditField) bool { return f == EditFieldDue }

	tests := []struct {
		name     string
		current  EditField
		skip     func(EditField) bool
		expected EditField
	}{
		{"assignee skips due to recurrence", EditFieldAssignee, skipDue, EditFieldRecurrence},
		{"due itself not relevant (would not be focused)", EditFieldDue, skipDue, EditFieldRecurrence},
		{"recurrence to tags (no skip)", EditFieldRecurrence, skipDue, EditFieldTags},
		{"tags to description", EditFieldTags, skipDue, EditFieldDescription},
		{"description stays (end of list)", EditFieldDescription, skipDue, EditFieldDescription},
		{"title to status (no skip involved)", EditFieldTitle, skipDue, EditFieldStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextFieldSkipping(tt.current, tt.skip)
			if got != tt.expected {
				t.Errorf("NextFieldSkipping(%v) = %v, want %v", tt.current, got, tt.expected)
			}
		})
	}
}

func TestPrevFieldSkipping(t *testing.T) {
	skipDue := func(f EditField) bool { return f == EditFieldDue }

	tests := []struct {
		name     string
		current  EditField
		skip     func(EditField) bool
		expected EditField
	}{
		{"recurrence skips due to assignee", EditFieldRecurrence, skipDue, EditFieldAssignee},
		{"assignee to points (no skip)", EditFieldAssignee, skipDue, EditFieldPoints},
		{"title stays (start of list)", EditFieldTitle, skipDue, EditFieldTitle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrevFieldSkipping(tt.current, tt.skip)
			if got != tt.expected {
				t.Errorf("PrevFieldSkipping(%v) = %v, want %v", tt.current, got, tt.expected)
			}
		})
	}
}

func TestIsEditableField(t *testing.T) {
	tests := []struct {
		name     string
		field    EditField
		expected bool
	}{
		{"Title is editable", EditFieldTitle, true},
		{"Status is editable", EditFieldStatus, true},
		{"Type is editable", EditFieldType, true},
		{"Priority is editable", EditFieldPriority, true},
		{"Assignee is editable", EditFieldAssignee, true},
		{"Points is editable", EditFieldPoints, true},
		{"Due is editable", EditFieldDue, true},
		{"Recurrence is editable", EditFieldRecurrence, true},
		{"Tags is editable", EditFieldTags, true},
		{"Description is editable", EditFieldDescription, true},
		{"Unknown field is not editable", EditField("bogus"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEditableField(tt.field)
			if result != tt.expected {
				t.Errorf("IsEditableField(%v) = %v, want %v", tt.field, result, tt.expected)
			}
		})
	}
}

func TestFieldLabel(t *testing.T) {
	tests := []struct {
		name     string
		field    EditField
		expected string
	}{
		{"Title label", EditFieldTitle, "Title"},
		{"Status label", EditFieldStatus, "Status"},
		{"Priority label", EditFieldPriority, "Priority"},
		{"Assignee label", EditFieldAssignee, "Assignee"},
		{"Points label", EditFieldPoints, "Story Points"},
		{"Due label", EditFieldDue, "Due"},
		{"Recurrence label", EditFieldRecurrence, "Recurrence"},
		{"Tags label", EditFieldTags, "Tags"},
		{"Description label", EditFieldDescription, "Description"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FieldLabel(tt.field)
			if result != tt.expected {
				t.Errorf("FieldLabel(%v) = %v, want %v", tt.field, result, tt.expected)
			}
		})
	}
}

func TestMetadataToEditFieldOrder(t *testing.T) {
	tests := []struct {
		name     string
		metadata []string
		expected []EditField
	}{
		{
			name:     "default 8-item metadata",
			metadata: []string{tikipkg.FieldStatus, tikipkg.FieldType, tikipkg.FieldPriority, tikipkg.FieldPoints, tikipkg.FieldAssignee, tikipkg.FieldDue, tikipkg.FieldRecurrence, tikipkg.FieldTags},
			expected: []EditField{EditFieldStatus, EditFieldType, EditFieldPriority, EditFieldPoints, EditFieldAssignee, EditFieldDue, EditFieldRecurrence, EditFieldTags},
		},
		{
			name:     "subset preserves order",
			metadata: []string{tikipkg.FieldStatus, tikipkg.FieldTags},
			expected: []EditField{EditFieldStatus, EditFieldTags},
		},
		{
			name:     "read-only descriptors are skipped",
			metadata: []string{tikipkg.FieldStatus, "createdBy", "createdAt", "updatedAt", tikipkg.FieldType},
			expected: []EditField{EditFieldStatus, EditFieldType},
		},
		{
			name:     "title in grid",
			metadata: []string{"title", tikipkg.FieldStatus, tikipkg.FieldType},
			expected: []EditField{EditFieldTitle, EditFieldStatus, EditFieldType},
		},
		{
			name:     "empty input → empty output",
			metadata: nil,
			expected: []EditField{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MetadataToEditFieldOrder(tt.metadata)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("MetadataToEditFieldOrder(%v) = %v, want %v", tt.metadata, got, tt.expected)
			}
		})
	}
}

// TestMetadataToEditFieldOrder_WorkflowEnums pins that workflow-declared
// enum fields without a static EditField constant participate in the
// navigation order using their field name as the EditField identity.
// Without this, custom enums like severity or environment never reached
// the focused-editor path in the full TikiEditView.
func TestMetadataToEditFieldOrder_WorkflowEnums(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{
			Name:       "severity",
			Type:       workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{{Value: "low"}, {Value: "high"}},
		},
		{Name: "score", Type: workflow.TypeInt},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	got := MetadataToEditFieldOrder([]string{
		tikipkg.FieldStatus, "severity", "score", tikipkg.FieldPriority,
	})
	want := []EditField{EditFieldStatus, EditField("severity"), EditFieldPriority}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MetadataToEditFieldOrder = %v, want %v (severity should appear; score is non-enum and should be skipped)", got, want)
	}
}

func TestNextFieldInOrder_RespectsCustomOrder(t *testing.T) {
	custom := []EditField{EditFieldTitle, EditFieldTags, EditFieldDescription}
	tests := []struct {
		name     string
		current  EditField
		expected EditField
	}{
		{"Title to Tags (custom order)", EditFieldTitle, EditFieldTags},
		{"Tags to Description (custom order)", EditFieldTags, EditFieldDescription},
		{"Description stays at Description", EditFieldDescription, EditFieldDescription},
		{"Status not in custom order falls back to first", EditFieldStatus, EditFieldTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextFieldInOrder(tt.current, custom, nil)
			if got != tt.expected {
				t.Errorf("NextFieldInOrder(%v, custom) = %v, want %v", tt.current, got, tt.expected)
			}
		})
	}
}

func TestPrevFieldInOrder_RespectsCustomOrder(t *testing.T) {
	custom := []EditField{EditFieldTitle, EditFieldTags, EditFieldDescription}
	tests := []struct {
		name     string
		current  EditField
		expected EditField
	}{
		{"Description to Tags (custom order)", EditFieldDescription, EditFieldTags},
		{"Tags to Title (custom order)", EditFieldTags, EditFieldTitle},
		{"Title stays at Title", EditFieldTitle, EditFieldTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrevFieldInOrder(tt.current, custom, nil)
			if got != tt.expected {
				t.Errorf("PrevFieldInOrder(%v, custom) = %v, want %v", tt.current, got, tt.expected)
			}
		})
	}
}
