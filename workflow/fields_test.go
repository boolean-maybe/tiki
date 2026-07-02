package workflow

import (
	"testing"

	"github.com/boolean-maybe/ruki/keyword"
)

func TestFieldDef_DisplayCaption(t *testing.T) {
	withCaption := FieldDef{Name: "status", Caption: "Status:"}
	if got := withCaption.DisplayCaption(); got != "Status:" {
		t.Errorf("DisplayCaption with caption = %q, want %q", got, "Status:")
	}
	noCaption := FieldDef{Name: "status"}
	if got := noCaption.DisplayCaption(); got != "status" {
		t.Errorf("DisplayCaption without caption = %q, want %q (field name)", got, "status")
	}
}

func TestValueType_IsList(t *testing.T) {
	listTypes := []ValueType{TypeListString, TypeListRef}
	for _, lt := range listTypes {
		if !lt.IsList() {
			t.Errorf("IsList(%v) = false, want true", lt)
		}
	}
	nonList := []ValueType{TypeString, TypeInt, TypeDate, TypeTimestamp, TypeBool, TypeID, TypeRef, TypeRecurrence, TypeEnum}
	for _, nt := range nonList {
		if nt.IsList() {
			t.Errorf("IsList(%v) = true, want false", nt)
		}
	}
}

func TestSystemField(t *testing.T) {
	tests := []struct {
		name   string
		want   ValueType
		wantOK bool
	}{
		{"id", TypeID, true},
		{"title", TypeString, true},
		{"description", TypeString, true},
		{"createdBy", TypeString, true},
		{"createdAt", TypeTimestamp, true},
		{"updatedAt", TypeTimestamp, true},
		{"filepath", TypeString, true},
		{"status", 0, false}, // workflow field, not system
		{"type", 0, false},   // workflow field, not system
		{"nonexistent", 0, false},
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

func TestIsSystemField(t *testing.T) {
	for _, name := range []string{"id", "title", "description", "createdBy", "createdAt", "updatedAt", "filepath", "body"} {
		if !IsSystemField(name) {
			t.Errorf("IsSystemField(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"status", "type", "priority", "points", "tags", "anything"} {
		if IsSystemField(name) {
			t.Errorf("IsSystemField(%q) = true, want false", name)
		}
	}
}

func TestSystemFields(t *testing.T) {
	fields := SystemFields()
	if len(fields) != 7 {
		t.Fatalf("expected 7 system fields, got %d", len(fields))
	}
	// verify it returns a copy
	fields[0].Name = "modified"
	original, _ := Field("id")
	if original.Name == "modified" {
		t.Error("SystemFields() should return a copy")
	}
}

func TestValidateFieldName_RejectsKeywords(t *testing.T) {
	for _, kw := range keyword.List() {
		t.Run(kw, func(t *testing.T) {
			if err := ValidateFieldName(kw); err == nil {
				t.Errorf("expected error for reserved keyword %q", kw)
			}
		})
	}
}

func TestValidateFieldName_RejectsInvalidIdentifiers(t *testing.T) {
	for _, name := range []string{"customer-id", "customer id", "9score", "a.b", ""} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFieldName(name); err == nil {
				t.Errorf("expected error for invalid identifier %q", name)
			}
		})
	}
}

func TestValidateFieldName_AcceptsValidIdentifiers(t *testing.T) {
	for _, name := range []string{"_private", "score2", "myField", "A", "_", "status", "type"} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFieldName(name); err != nil {
				t.Errorf("unexpected error for %q: %v", name, err)
			}
		})
	}
}

func TestRegisterWorkflowFields_StatusIsOrdinaryEnum(t *testing.T) {
	t.Cleanup(func() { ClearWorkflowFields() })

	defs := []FieldDef{
		{
			Name: "status",
			Type: TypeEnum,
			EnumValues: []EnumValue{
				{Value: "open", Label: "Open", Visual: "📂", Default: true},
				{Value: "closed", Label: "Closed", Visual: "🔒"},
			},
		},
	}
	if err := RegisterWorkflowFields(defs); err != nil {
		t.Fatalf("register: %v", err)
	}

	f, ok := Field("status")
	if !ok {
		t.Fatal("status not found after register")
	}
	if !f.Custom {
		t.Error("status.Custom should be true")
	}
	if f.EnumDefault() != "open" {
		t.Errorf("EnumDefault = %q, want open", f.EnumDefault())
	}
	if !f.IsValidEnum("open") || !f.IsValidEnum("closed") {
		t.Error("expected open/closed to be valid enum values")
	}
	if f.IsValidEnum("nonexistent") {
		t.Error("nonexistent should not be valid")
	}
	if got := f.EnumDisplay("open"); got != "📂" {
		t.Errorf("EnumDisplay(open) = %q, want %q", got, "📂")
	}
	if got := f.AllowedValues(); len(got) != 2 || got[0] != "open" || got[1] != "closed" {
		t.Errorf("AllowedValues = %v", got)
	}
}

func TestRegisterWorkflowFields_RejectsSystemFieldCollision(t *testing.T) {
	t.Cleanup(func() { ClearWorkflowFields() })

	for _, name := range []string{"id", "title", "description", "createdAt", "createdBy", "updatedAt", "filepath", "body"} {
		t.Run(name, func(t *testing.T) {
			err := RegisterWorkflowFields([]FieldDef{{Name: name, Type: TypeString}})
			if err == nil {
				t.Errorf("expected error for system field collision %q", name)
			}
		})
	}
}

func TestRegisterWorkflowFields_CaseInsensitiveSystemCollision(t *testing.T) {
	t.Cleanup(func() { ClearWorkflowFields() })

	for _, name := range []string{"Title", "ID", "CreatedAt", "BODY"} {
		t.Run(name, func(t *testing.T) {
			if err := RegisterWorkflowFields([]FieldDef{{Name: name, Type: TypeString}}); err == nil {
				t.Errorf("expected error for case-insensitive collision %q", name)
			}
		})
	}
}

func TestRegisterWorkflowFields_EnumRejectsEmptyValues(t *testing.T) {
	t.Cleanup(func() { ClearWorkflowFields() })

	if err := RegisterWorkflowFields([]FieldDef{{Name: "severity", Type: TypeEnum}}); err == nil {
		t.Fatal("expected error for enum without values")
	}
}

func TestRegisterWorkflowFields_EnumRejectsMultipleDefaults(t *testing.T) {
	t.Cleanup(func() { ClearWorkflowFields() })

	defs := []FieldDef{{
		Name: "status",
		Type: TypeEnum,
		EnumValues: []EnumValue{
			{Value: "a", Default: true},
			{Value: "b", Default: true},
		},
	}}
	if err := RegisterWorkflowFields(defs); err == nil {
		t.Fatal("expected error for multiple defaults")
	}
}

// TestRegisterWorkflowFields_EnumRejectsCaseInsensitiveDuplicates pins
// that the duplicate-key check matches runtime case-insensitivity. Both
// enumRank (ruki/executor.go) and coerceCustomValue
// (store/tikistore/persistence.go) use strings.EqualFold to match enum
// values, so a workflow declaring both "high" and "HIGH" would silently
// have the second value masked at runtime. Validation must reject this.
func TestRegisterWorkflowFields_EnumRejectsCaseInsensitiveDuplicates(t *testing.T) {
	tests := []struct {
		name   string
		values []EnumValue
	}{
		{
			name: "exact-case duplicate",
			values: []EnumValue{
				{Value: "high"},
				{Value: "high"},
			},
		},
		{
			name: "case-only duplicate",
			values: []EnumValue{
				{Value: "high"},
				{Value: "HIGH"},
			},
		},
		{
			name: "mixed-case-only duplicate",
			values: []EnumValue{
				{Value: "Medium"},
				{Value: "medium"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { ClearWorkflowFields() })
			err := RegisterWorkflowFields([]FieldDef{{
				Name:       "priority",
				Type:       TypeEnum,
				EnumValues: tt.values,
			}})
			if err == nil {
				t.Fatal("expected duplicate-value rejection")
			}
		})
	}
}

func TestRegisterWorkflowFields_DefensiveCopy(t *testing.T) {
	t.Cleanup(func() { ClearWorkflowFields() })

	defs := []FieldDef{{
		Name: "severity",
		Type: TypeEnum,
		EnumValues: []EnumValue{
			{Value: "low", Label: "Low"},
			{Value: "high", Label: "High"},
		},
	}}
	if err := RegisterWorkflowFields(defs); err != nil {
		t.Fatalf("register: %v", err)
	}

	// mutate the input — should not affect registry
	defs[0].EnumValues[0].Value = "MUTATED"

	f, _ := Field("severity")
	if f.EnumValues[0].Value != "low" {
		t.Errorf("input mutation leaked: %q", f.EnumValues[0].Value)
	}
}

func TestClearWorkflowFields(t *testing.T) {
	if err := RegisterWorkflowFields([]FieldDef{{Name: "notes", Type: TypeString}}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := Field("notes"); !ok {
		t.Fatal("field not found after register")
	}

	ClearWorkflowFields()
	if _, ok := Field("notes"); ok {
		t.Error("field still found after clear")
	}
}

func TestEnumParseDisplay(t *testing.T) {
	fd := FieldDef{
		Name: "status",
		Type: TypeEnum,
		EnumValues: []EnumValue{
			{Value: "open", Label: "Open", Visual: "📂"},
			{Value: "closed", Label: "Closed"},
		},
	}
	if got, ok := fd.EnumParseDisplay("📂"); !ok || got != "open" {
		t.Errorf("EnumParseDisplay(📂) = (%q, %v)", got, ok)
	}
	if got, ok := fd.EnumParseDisplay("Closed"); !ok || got != "closed" {
		t.Errorf("EnumParseDisplay(Closed) = (%q, %v)", got, ok)
	}
	if _, ok := fd.EnumParseDisplay("Bogus"); ok {
		t.Error("expected EnumParseDisplay to fail on unknown display")
	}
}
