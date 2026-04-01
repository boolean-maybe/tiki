package task

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
)

func TestNormalizeType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Type
	}{
		// Valid types
		{name: "story", input: "story", expected: TypeStory},
		{name: "bug", input: "bug", expected: TypeBug},
		{name: "spike", input: "spike", expected: TypeSpike},
		{name: "epic", input: "epic", expected: TypeEpic},
		{name: "feature -> story", input: "feature", expected: TypeStory},
		{name: "task -> story", input: "task", expected: TypeStory},
		// Case variations
		{name: "Story capitalized", input: "Story", expected: TypeStory},
		{name: "BUG uppercase", input: "BUG", expected: TypeBug},
		{name: "SPIKE uppercase", input: "SPIKE", expected: TypeSpike},
		{name: "EPIC uppercase", input: "EPIC", expected: TypeEpic},
		{name: "FEATURE uppercase", input: "FEATURE", expected: TypeStory},
		// Unknown defaults to story
		{name: "unknown type", input: "unknown", expected: TypeStory},
		{name: "empty string", input: "", expected: TypeStory},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeType(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeType(%q) = %q, want %q", tt.input, result, tt.expected)
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

// TestTypeHelpers_FallbackWithoutConfig verifies that all type helpers work
// when the config registry has not been initialized (fallback to defaults).
func TestTypeHelpers_FallbackWithoutConfig(t *testing.T) {
	config.ClearStatusRegistry()
	t.Cleanup(func() {
		// restore for other tests in the package
		config.ResetStatusRegistry(testStatusDefs())
	})

	t.Run("NormalizeType", func(t *testing.T) {
		if got := NormalizeType("bug"); got != TypeBug {
			t.Errorf("NormalizeType(%q) = %q, want %q", "bug", got, TypeBug)
		}
		if got := NormalizeType("feature"); got != TypeStory {
			t.Errorf("NormalizeType(%q) = %q, want %q (alias)", "feature", got, TypeStory)
		}
		if got := NormalizeType("unknown"); got != TypeStory {
			t.Errorf("NormalizeType(%q) = %q, want %q (fallback)", "unknown", got, TypeStory)
		}
	})

	t.Run("ParseType", func(t *testing.T) {
		typ, ok := ParseType("epic")
		if !ok || typ != TypeEpic {
			t.Errorf("ParseType(%q) = (%q, %v), want (%q, true)", "epic", typ, ok, TypeEpic)
		}
		typ, ok = ParseType("nonsense")
		if ok {
			t.Errorf("ParseType(%q) returned ok=true for unknown type", "nonsense")
		}
		if typ != TypeStory {
			t.Errorf("ParseType(%q) fallback = %q, want %q", "nonsense", typ, TypeStory)
		}
	})

	t.Run("TypeLabel", func(t *testing.T) {
		if got := TypeLabel(TypeBug); got != "Bug" {
			t.Errorf("TypeLabel(%q) = %q, want %q", TypeBug, got, "Bug")
		}
	})

	t.Run("TypeDisplay", func(t *testing.T) {
		if got := TypeDisplay(TypeSpike); got != "Spike 🔍" {
			t.Errorf("TypeDisplay(%q) = %q, want %q", TypeSpike, got, "Spike 🔍")
		}
	})
}

// testStatusDefs returns the standard test status definitions.
func testStatusDefs() []config.StatusDef {
	return []config.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "in_progress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "review", Label: "Review", Emoji: "👀", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	}
}
