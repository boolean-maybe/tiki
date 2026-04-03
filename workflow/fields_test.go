package workflow

import (
	"testing"

	"github.com/boolean-maybe/tiki/ruki/keyword"
)

func TestField(t *testing.T) {
	tests := []struct {
		name   string
		want   ValueType
		wantOK bool
	}{
		{"id", TypeID, true},
		{"title", TypeString, true},
		{"description", TypeString, true},
		{"status", TypeStatus, true},
		{"type", TypeTaskType, true},
		{"tags", TypeListString, true},
		{"dependsOn", TypeListRef, true},
		{"due", TypeDate, true},
		{"recurrence", TypeRecurrence, true},
		{"assignee", TypeString, true},
		{"priority", TypeInt, true},
		{"points", TypeInt, true},
		{"createdBy", TypeString, true},
		{"createdAt", TypeTimestamp, true},
		{"updatedAt", TypeTimestamp, true},
		{"nonexistent", 0, false},
		{"comments", 0, false},    // excluded from DSL catalog
		{"loadedMtime", 0, false}, // excluded from DSL catalog
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, ok := Field(tt.name)
			if ok != tt.wantOK {
				t.Errorf("Field(%q) ok = %v, want %v", tt.name, ok, tt.wantOK)
				return
			}
			if ok && f.Type != tt.want {
				t.Errorf("Field(%q).Type = %v, want %v", tt.name, f.Type, tt.want)
			}
		})
	}
}

func TestFields(t *testing.T) {
	fields := Fields()
	if len(fields) != 15 {
		t.Fatalf("expected 15 fields, got %d", len(fields))
	}

	// verify it returns a copy
	fields[0].Name = "modified"
	original, _ := Field("id")
	if original.Name == "modified" {
		t.Error("Fields() should return a copy, not a reference to the internal slice")
	}
}

func TestDateVsTimestamp(t *testing.T) {
	due, _ := Field("due")
	if due.Type != TypeDate {
		t.Errorf("due should be TypeDate, got %v", due.Type)
	}

	createdAt, _ := Field("createdAt")
	if createdAt.Type != TypeTimestamp {
		t.Errorf("createdAt should be TypeTimestamp, got %v", createdAt.Type)
	}

	updatedAt, _ := Field("updatedAt")
	if updatedAt.Type != TypeTimestamp {
		t.Errorf("updatedAt should be TypeTimestamp, got %v", updatedAt.Type)
	}
}

func TestValidateFieldName_RejectsKeywords(t *testing.T) {
	for _, kw := range keyword.List() {
		t.Run(kw, func(t *testing.T) {
			err := ValidateFieldName(kw)
			if err == nil {
				t.Errorf("expected error for reserved keyword %q", kw)
			}
		})
	}
}

func TestValidateFieldName_CaseInsensitive(t *testing.T) {
	for _, name := range []string{"SELECT", "Where", "AND", "Order", "NEW"} {
		t.Run(name, func(t *testing.T) {
			err := ValidateFieldName(name)
			if err == nil {
				t.Errorf("expected error for %q", name)
			}
		})
	}
}

func TestValidateFieldName_AcceptsNonKeywords(t *testing.T) {
	for _, name := range []string{"title", "status", "selectAll", "newsletter", "priority", "dependsOn"} {
		t.Run(name, func(t *testing.T) {
			err := ValidateFieldName(name)
			if err != nil {
				t.Errorf("unexpected error for %q: %v", name, err)
			}
		})
	}
}
