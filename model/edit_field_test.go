package model

import "testing"

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
		{"Recurrence to Description", EditFieldRecurrence, EditFieldDescription},
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
		{"Description to Recurrence", EditFieldDescription, EditFieldRecurrence},
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
	// Test complete forward navigation (stops at end, no wrap)
	field := EditFieldTitle
	expectedOrder := []EditField{
		EditFieldStatus,
		EditFieldType,
		EditFieldPriority,
		EditFieldPoints,
		EditFieldAssignee,
		EditFieldDue,
		EditFieldRecurrence,
		EditFieldDescription,
		EditFieldDescription, // stays at end
	}

	for i, expected := range expectedOrder {
		field = NextField(field)
		if field != expected {
			t.Errorf("Forward navigation step %d: got %v, want %v", i, field, expected)
		}
	}

	// Test complete backward navigation (stops at beginning, no wrap)
	field = EditFieldDescription
	expectedOrderReverse := []EditField{
		EditFieldRecurrence,
		EditFieldDue,
		EditFieldAssignee,
		EditFieldPoints,
		EditFieldPriority,
		EditFieldType,
		EditFieldStatus,
		EditFieldTitle,
		EditFieldTitle, // stays at beginning
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
		{"recurrence to description (no skip)", EditFieldRecurrence, skipDue, EditFieldDescription},
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
		{"Status is not editable yet", EditFieldStatus, false},
		{"Priority is editable", EditFieldPriority, true},
		{"Assignee is editable", EditFieldAssignee, true},
		{"Points is editable", EditFieldPoints, true},
		{"Due is editable", EditFieldDue, true},
		{"Recurrence is editable", EditFieldRecurrence, true},
		{"Description is editable", EditFieldDescription, true},
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
