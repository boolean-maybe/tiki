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
		{"Priority to Assignee", EditFieldPriority, EditFieldAssignee},
		{"Assignee to Points", EditFieldAssignee, EditFieldPoints},
		{"Points to Description", EditFieldPoints, EditFieldDescription},
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
		{"Assignee to Priority", EditFieldAssignee, EditFieldPriority},
		{"Points to Assignee", EditFieldPoints, EditFieldAssignee},
		{"Description to Points", EditFieldDescription, EditFieldPoints},
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
		EditFieldAssignee,
		EditFieldPoints,
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
		EditFieldPoints,
		EditFieldAssignee,
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
