package task

import "testing"

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
