package task

import (
	"os"
	"testing"

	"github.com/boolean-maybe/tiki/config"
)

func TestMain(m *testing.M) {
	config.ResetStatusRegistry(defaultTestStatusDefs())
	os.Exit(m.Run())
}

func defaultTestStatusDefs() []config.StatusDef {
	return []config.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "inProgress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "review", Label: "Review", Emoji: "👀", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	}
}

func TestParseType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType Type
		wantOK   bool
	}{
		{"valid story", "story", TypeStory, true},
		{"valid bug", "bug", TypeBug, true},
		{"valid spike", "spike", TypeSpike, true},
		{"valid epic", "epic", TypeEpic, true},
		{"case insensitive", "Story", TypeStory, true},
		{"uppercase", "BUG", TypeBug, true},
		{"unknown returns empty", "nonsense", "", false},
		{"empty returns empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseType(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ParseType(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.wantType {
				t.Errorf("ParseType(%q) = %q, want %q", tt.input, got, tt.wantType)
			}
		})
	}
}

func TestTypeLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    Type
		expected string
	}{
		{name: "story label", input: TypeStory, expected: "Story"},
		{name: "bug label", input: TypeBug, expected: "Bug"},
		{name: "spike label", input: TypeSpike, expected: "Spike"},
		{name: "epic label", input: TypeEpic, expected: "Epic"},
		{name: "unknown type", input: Type("unknown"), expected: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TypeLabel(tt.input)
			if result != tt.expected {
				t.Errorf("TypeLabel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTypeEmoji(t *testing.T) {
	tests := []struct {
		name     string
		input    Type
		expected string
	}{
		{name: "story emoji", input: TypeStory, expected: "🌀"},
		{name: "bug emoji", input: TypeBug, expected: "💥"},
		{name: "spike emoji", input: TypeSpike, expected: "🔍"},
		{name: "epic emoji", input: TypeEpic, expected: "🗂️"},
		{name: "unknown type", input: Type("unknown"), expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TypeEmoji(tt.input)
			if result != tt.expected {
				t.Errorf("TypeEmoji(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTypeDisplay(t *testing.T) {
	tests := []struct {
		name     string
		input    Type
		expected string
	}{
		{name: "story display", input: TypeStory, expected: "Story 🌀"},
		{name: "bug display", input: TypeBug, expected: "Bug 💥"},
		{name: "spike display", input: TypeSpike, expected: "Spike 🔍"},
		{name: "epic display", input: TypeEpic, expected: "Epic 🗂️"},
		{name: "unknown type", input: Type("unknown"), expected: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TypeDisplay(tt.input)
			if result != tt.expected {
				t.Errorf("TypeDisplay(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDisplay(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType Type
		wantOK   bool
	}{
		{"story display", "Story 🌀", TypeStory, true},
		{"bug display", "Bug 💥", TypeBug, true},
		{"spike display", "Spike 🔍", TypeSpike, true},
		{"epic display", "Epic 🗂️", TypeEpic, true},
		{"unknown display", "Unknown 🤷", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseDisplay(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ParseDisplay(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.wantType {
				t.Errorf("ParseDisplay(%q) = %q, want %q", tt.input, got, tt.wantType)
			}
		})
	}
}

func TestAllTypes(t *testing.T) {
	types := AllTypes()
	if len(types) == 0 {
		t.Fatal("AllTypes() returned empty list")
	}
	found := make(map[Type]bool)
	for _, tp := range types {
		found[tp] = true
	}
	for _, want := range []Type{TypeStory, TypeBug, TypeSpike, TypeEpic} {
		if !found[want] {
			t.Errorf("AllTypes() missing %q", want)
		}
	}
}

func TestDefaultType(t *testing.T) {
	if got := DefaultType(); got != TypeStory {
		t.Errorf("DefaultType() = %q, want %q", got, TypeStory)
	}
}

// TestPreInitPanics verifies that type helpers fail before registries are loaded.
func TestPreInitPanics(t *testing.T) {
	config.ClearStatusRegistry()
	t.Cleanup(func() {
		config.ResetStatusRegistry(defaultTestStatusDefs())
	})

	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected panic before registry init", name)
			}
		}()
		fn()
	}

	assertPanics("ParseType", func() { ParseType("story") })
	assertPanics("AllTypes", func() { AllTypes() })
	assertPanics("DefaultType", func() { DefaultType() })
	assertPanics("TypeLabel", func() { TypeLabel(TypeStory) })
	assertPanics("ParseDisplay", func() { ParseDisplay("Story 🌀") })
}
