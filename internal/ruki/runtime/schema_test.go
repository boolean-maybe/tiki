package runtime

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/ruki"
	"github.com/boolean-maybe/tiki/workflow"
)

func initTestRegistries() {
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "inProgress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
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
		{"status", ruki.ValueStatus},
		{"type", ruki.ValueTaskType},
		{"tags", ruki.ValueListString},
		{"dependsOn", ruki.ValueListRef},
		{"due", ruki.ValueDate},
		{"priority", ruki.ValueInt},
		{"createdAt", ruki.ValueTimestamp},
		{"updatedAt", ruki.ValueTimestamp},
		{"recurrence", ruki.ValueRecurrence},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := s.Field(tt.name)
			if !ok {
				t.Fatalf("Field(%q) not found", tt.name)
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

func TestSchemaNormalizeStatus(t *testing.T) {
	initTestRegistries()
	s := NewSchema()

	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{"done", "done", true},
		{"backlog", "backlog", true},
		{"inProgress", "inProgress", true},
		{"unknown_status", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := s.NormalizeStatus(tt.input)
			if ok != tt.wantOK {
				t.Errorf("NormalizeStatus(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("NormalizeStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSchemaNormalizeType(t *testing.T) {
	initTestRegistries()
	s := NewSchema()

	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{"story", "story", true},
		{"bug", "bug", true},
		{"feature", "story", true},       // alias
		{"task", "story", true},          // alias
		{"unknown_type", "story", false}, // falls back to first type
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := s.NormalizeType(tt.input)
			if ok != tt.wantOK {
				t.Errorf("NormalizeType(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("NormalizeType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMapValueTypeCompleteness(t *testing.T) {
	// verify every workflow type maps to a distinct ruki type
	seen := make(map[ruki.ValueType]workflow.ValueType)
	types := []workflow.ValueType{
		workflow.TypeString, workflow.TypeInt, workflow.TypeDate,
		workflow.TypeTimestamp, workflow.TypeDuration, workflow.TypeBool,
		workflow.TypeID, workflow.TypeRef, workflow.TypeRecurrence,
		workflow.TypeListString, workflow.TypeListRef,
		workflow.TypeStatus, workflow.TypeTaskType,
	}

	for _, wt := range types {
		rv := mapValueType(wt)
		if prev, exists := seen[rv]; exists {
			t.Errorf("mapValueType(%d) and mapValueType(%d) both map to ruki.ValueType %d", prev, wt, rv)
		}
		seen[rv] = wt
	}
}

func TestMapValueTypeUnknownFallback(t *testing.T) {
	got := mapValueType(workflow.ValueType(999))
	if got != ruki.ValueString {
		t.Errorf("expected fallback to ValueString, got %d", got)
	}
}
