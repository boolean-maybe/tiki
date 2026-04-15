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

func TestValidateFieldName_RejectsInvalidIdentifiers(t *testing.T) {
	for _, name := range []string{"customer-id", "customer id", "9score", "a.b", ""} {
		t.Run(name, func(t *testing.T) {
			err := ValidateFieldName(name)
			if err == nil {
				t.Errorf("expected error for invalid identifier %q", name)
			}
		})
	}
}

func TestValidateFieldName_AcceptsValidIdentifiers(t *testing.T) {
	for _, name := range []string{"_private", "score2", "myField", "A", "_"} {
		t.Run(name, func(t *testing.T) {
			err := ValidateFieldName(name)
			if err != nil {
				t.Errorf("unexpected error for %q: %v", name, err)
			}
		})
	}
}

func TestRegisterCustomFields(t *testing.T) {
	t.Cleanup(func() { ClearCustomFields() })

	defs := []FieldDef{
		{Name: "notes", Type: TypeString},
		{Name: "score", Type: TypeInt},
		{Name: "active", Type: TypeBool},
		{Name: "startedAt", Type: TypeTimestamp},
		{Name: "severity", Type: TypeEnum, AllowedValues: []string{"low", "medium", "high"}},
		{Name: "labels", Type: TypeListString},
		{Name: "related", Type: TypeListRef},
	}

	if err := RegisterCustomFields(defs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify Field() finds each custom field
	for _, d := range defs {
		f, ok := Field(d.Name)
		if !ok {
			t.Errorf("Field(%q) not found", d.Name)
			continue
		}
		if f.Type != d.Type {
			t.Errorf("Field(%q).Type = %v, want %v", d.Name, f.Type, d.Type)
		}
		if !f.Custom {
			t.Errorf("Field(%q).Custom = false, want true", d.Name)
		}
	}

	// verify Fields() includes custom fields
	all := Fields()
	if len(all) != 15+len(defs) {
		t.Errorf("Fields() length = %d, want %d", len(all), 15+len(defs))
	}

	// verify enum AllowedValues
	sev, _ := Field("severity")
	if len(sev.AllowedValues) != 3 || sev.AllowedValues[0] != "low" {
		t.Errorf("severity.AllowedValues = %v, want [low medium high]", sev.AllowedValues)
	}
}

func TestRegisterCustomFields_Collisions(t *testing.T) {
	t.Cleanup(func() { ClearCustomFields() })

	// each built-in field name should be rejected
	for _, name := range []string{"id", "title", "status", "priority", "tags", "due"} {
		t.Run(name, func(t *testing.T) {
			err := RegisterCustomFields([]FieldDef{{Name: name, Type: TypeString}})
			if err == nil {
				t.Errorf("expected error for collision with built-in %q", name)
			}
		})
	}
}

func TestRegisterCustomFields_CaseInsensitiveBuiltinCollision(t *testing.T) {
	t.Cleanup(func() { ClearCustomFields() })

	for _, name := range []string{"Title", "STATUS", "DependsOn", "DEPENDSON", "CreatedAt", "CREATEDAT"} {
		t.Run(name, func(t *testing.T) {
			err := RegisterCustomFields([]FieldDef{{Name: name, Type: TypeString}})
			if err == nil {
				t.Errorf("expected error for case-insensitive collision with built-in for %q", name)
			}
		})
	}
}

func TestRegisterCustomFields_CaseInsensitiveBatchCollision(t *testing.T) {
	t.Cleanup(func() { ClearCustomFields() })

	err := RegisterCustomFields([]FieldDef{
		{Name: "myField", Type: TypeString},
		{Name: "MyField", Type: TypeInt},
	})
	if err == nil {
		t.Fatal("expected error for case-insensitive collision between custom fields")
	}
}

func TestRegisterCustomFields_ReservedKeyword(t *testing.T) {
	t.Cleanup(func() { ClearCustomFields() })

	for _, name := range []string{"select", "where", "and", "order"} {
		t.Run(name, func(t *testing.T) {
			err := RegisterCustomFields([]FieldDef{{Name: name, Type: TypeString}})
			if err == nil {
				t.Errorf("expected error for reserved keyword %q", name)
			}
		})
	}
}

func TestRegisterCustomFields_TrueFalseRejected(t *testing.T) {
	t.Cleanup(func() { ClearCustomFields() })

	for _, name := range []string{"true", "false", "True", "FALSE"} {
		t.Run(name, func(t *testing.T) {
			err := RegisterCustomFields([]FieldDef{{Name: name, Type: TypeString}})
			if err == nil {
				t.Errorf("expected error for boolean literal name %q", name)
			}
		})
	}
}

func TestRegisterCustomFields_EnumRequiresValues(t *testing.T) {
	t.Cleanup(func() { ClearCustomFields() })

	err := RegisterCustomFields([]FieldDef{{Name: "severity", Type: TypeEnum}})
	if err == nil {
		t.Fatal("expected error for enum without values")
	}
}

func TestClearCustomFields(t *testing.T) {
	t.Cleanup(func() { ClearCustomFields() })

	err := RegisterCustomFields([]FieldDef{{Name: "notes", Type: TypeString}})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := Field("notes"); !ok {
		t.Fatal("field not found after register")
	}

	ClearCustomFields()

	if _, ok := Field("notes"); ok {
		t.Error("field still found after clear")
	}

	// Fields() should return only built-ins
	if len(Fields()) != 15 {
		t.Errorf("Fields() = %d, want 15 after clear", len(Fields()))
	}
}

func TestRegisterCustomFields_DefensiveCopy(t *testing.T) {
	t.Cleanup(func() { ClearCustomFields() })

	inputVals := []string{"low", "high"}
	defs := []FieldDef{{Name: "severity", Type: TypeEnum, AllowedValues: inputVals}}

	if err := RegisterCustomFields(defs); err != nil {
		t.Fatalf("register: %v", err)
	}

	// mutate the input slice — should not affect registry
	inputVals[0] = "MUTATED"
	defs[0].Name = "MUTATED"

	f, ok := Field("severity")
	if !ok {
		t.Fatal("field not found")
	}
	if f.AllowedValues[0] != "low" {
		t.Errorf("input mutation leaked: AllowedValues[0] = %q, want %q", f.AllowedValues[0], "low")
	}

	// mutate the returned AllowedValues — should not affect registry
	f.AllowedValues[0] = "MUTATED"
	f2, _ := Field("severity")
	if f2.AllowedValues[0] != "low" {
		t.Errorf("returned mutation leaked: AllowedValues[0] = %q, want %q", f2.AllowedValues[0], "low")
	}
}
