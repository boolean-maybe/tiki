package workflow

import "testing"

func mustBuildTypeRegistry(t *testing.T, defs []TypeDef) *TypeRegistry {
	t.Helper()
	reg, err := NewTypeRegistry(defs)
	if err != nil {
		t.Fatalf("NewTypeRegistry: %v", err)
	}
	return reg
}

func TestNormalizeTypeKey(t *testing.T) {
	tests := []struct {
		input string
		want  TaskType
	}{
		{"story", "story"},
		{"Story", "story"},
		{"BUG", "bug"},
		{"SPIKE", "spike"},
		{"in_progress", "inprogress"},
		{"some-type", "sometype"},
		{"  EPIC  ", "epic"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := NormalizeTypeKey(tt.input); got != tt.want {
				t.Errorf("NormalizeTypeKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTypeRegistry_ParseType(t *testing.T) {
	reg := mustBuildTypeRegistry(t, DefaultTypeDefs())

	tests := []struct {
		name   string
		input  string
		want   TaskType
		wantOK bool
	}{
		{"story", "story", TypeStory, true},
		{"bug", "bug", TypeBug, true},
		{"spike", "spike", TypeSpike, true},
		{"epic", "epic", TypeEpic, true},
		{"case insensitive", "Story", TypeStory, true},
		{"uppercase", "BUG", TypeBug, true},
		{"unknown returns empty", "unknown", "", false},
		{"empty returns empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := reg.ParseType(tt.input)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("ParseType(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestTypeRegistry_ParseDisplay(t *testing.T) {
	reg := mustBuildTypeRegistry(t, DefaultTypeDefs())

	tests := []struct {
		name   string
		input  string
		want   TaskType
		wantOK bool
	}{
		{"story display", "Story 🌀", TypeStory, true},
		{"bug display", "Bug 💥", TypeBug, true},
		{"spike display", "Spike 🔍", TypeSpike, true},
		{"epic display", "Epic 🗂️", TypeEpic, true},
		{"unknown returns empty", "Unknown", "", false},
		{"label only", "Bug", "", false},
		{"empty returns empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := reg.ParseDisplay(tt.input)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("ParseDisplay(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestTypeRegistry_TypeLabel(t *testing.T) {
	reg := mustBuildTypeRegistry(t, DefaultTypeDefs())

	tests := []struct {
		input TaskType
		want  string
	}{
		{TypeStory, "Story"},
		{TypeBug, "Bug"},
		{TypeSpike, "Spike"},
		{TypeEpic, "Epic"},
		{TaskType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := reg.TypeLabel(tt.input); got != tt.want {
				t.Errorf("TypeLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTypeRegistry_TypeEmoji(t *testing.T) {
	reg := mustBuildTypeRegistry(t, DefaultTypeDefs())

	tests := []struct {
		input TaskType
		want  string
	}{
		{TypeStory, "🌀"},
		{TypeBug, "💥"},
		{TypeSpike, "🔍"},
		{TypeEpic, "🗂️"},
		{TaskType("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := reg.TypeEmoji(tt.input); got != tt.want {
				t.Errorf("TypeEmoji(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTypeRegistry_TypeDisplay(t *testing.T) {
	reg := mustBuildTypeRegistry(t, DefaultTypeDefs())

	tests := []struct {
		input TaskType
		want  string
	}{
		{TypeStory, "Story 🌀"},
		{TypeBug, "Bug 💥"},
		{TypeSpike, "Spike 🔍"},
		{TypeEpic, "Epic 🗂️"},
		{TaskType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := reg.TypeDisplay(tt.input); got != tt.want {
				t.Errorf("TypeDisplay(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTypeRegistry_DefaultType(t *testing.T) {
	reg := mustBuildTypeRegistry(t, DefaultTypeDefs())
	if got := reg.DefaultType(); got != TypeStory {
		t.Errorf("DefaultType() = %q, want %q", got, TypeStory)
	}

	// custom registry: first type is the default
	custom := mustBuildTypeRegistry(t, []TypeDef{
		{Key: "task", Label: "Task"},
		{Key: "bug", Label: "Bug"},
	})
	if got := custom.DefaultType(); got != "task" {
		t.Errorf("DefaultType() = %q, want %q", got, "task")
	}
}

func TestTypeRegistry_Keys(t *testing.T) {
	reg := mustBuildTypeRegistry(t, DefaultTypeDefs())

	keys := reg.Keys()
	expected := []TaskType{TypeStory, TypeBug, TypeSpike, TypeEpic}

	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, key := range keys {
		if key != expected[i] {
			t.Errorf("keys[%d] = %q, want %q", i, key, expected[i])
		}
	}
}

func TestTypeRegistry_IsValid(t *testing.T) {
	reg := mustBuildTypeRegistry(t, DefaultTypeDefs())

	if !reg.IsValid(TypeStory) {
		t.Error("expected story to be valid")
	}
	if reg.IsValid("unknown") {
		t.Error("expected unknown to not be valid")
	}
}

func TestTypeRegistry_LookupNormalizesInput(t *testing.T) {
	reg := mustBuildTypeRegistry(t, DefaultTypeDefs())

	tests := []struct {
		name  string
		input TaskType
		want  bool
	}{
		{"lowercase", "story", true},
		{"uppercase", "STORY", true},
		{"mixed case", "Bug", true},
		{"unknown", "nope", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := reg.Lookup(tt.input)
			if ok != tt.want {
				t.Errorf("Lookup(%q) ok = %v, want %v", tt.input, ok, tt.want)
			}
		})
	}

	if label := reg.TypeLabel("BUG"); label != "Bug" {
		t.Errorf("TypeLabel(BUG) = %q, want %q", label, "Bug")
	}
	if emoji := reg.TypeEmoji("EPIC"); emoji != "🗂️" {
		t.Errorf("TypeEmoji(EPIC) = %q, want %q", emoji, "🗂️")
	}
	if !reg.IsValid("SPIKE") {
		t.Error("expected IsValid(SPIKE) to be true after normalization")
	}
}

func TestNewTypeRegistry_EmptyKey(t *testing.T) {
	defs := []TypeDef{{Key: "", Label: "No Key"}}
	_, err := NewTypeRegistry(defs)
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestNewTypeRegistry_DuplicateKey(t *testing.T) {
	defs := []TypeDef{
		{Key: "story", Label: "Story"},
		{Key: "story", Label: "Story 2"},
	}
	_, err := NewTypeRegistry(defs)
	if err == nil {
		t.Error("expected error for duplicate key")
	}
}

func TestNewTypeRegistry_NonCanonicalKey(t *testing.T) {
	defs := []TypeDef{
		{Key: "Story", Label: "Story"},
	}
	_, err := NewTypeRegistry(defs)
	if err == nil {
		t.Fatal("expected error for non-canonical key")
	}
	if got := err.Error(); got != `type key "Story" is not canonical; use "story"` {
		t.Errorf("got error %q", got)
	}
}

func TestNewTypeRegistry_LabelDefaultsToKey(t *testing.T) {
	reg := mustBuildTypeRegistry(t, []TypeDef{
		{Key: "task", Emoji: "📋"},
	})
	if got := reg.TypeLabel("task"); got != "task" {
		t.Errorf("expected label to default to key, got %q", got)
	}
}

func TestNewTypeRegistry_EmptyWhitespaceLabel(t *testing.T) {
	_, err := NewTypeRegistry([]TypeDef{
		{Key: "task", Label: "  "},
	})
	if err == nil {
		t.Error("expected error for whitespace-only label")
	}
}

func TestNewTypeRegistry_EmojiTrimmed(t *testing.T) {
	reg := mustBuildTypeRegistry(t, []TypeDef{
		{Key: "task", Label: "Task", Emoji: " 🔧 "},
	})
	if got := reg.TypeEmoji("task"); got != "🔧" {
		t.Errorf("expected trimmed emoji, got %q", got)
	}
}

func TestNewTypeRegistry_DuplicateDisplay(t *testing.T) {
	_, err := NewTypeRegistry([]TypeDef{
		{Key: "task", Label: "Item", Emoji: "📋"},
		{Key: "work", Label: "Item", Emoji: "📋"},
	})
	if err == nil {
		t.Error("expected error for duplicate display")
	}
}

func TestNewTypeRegistry_DuplicateDisplayLabelOnly(t *testing.T) {
	// duplicate label with no emoji — display is just the label
	_, err := NewTypeRegistry([]TypeDef{
		{Key: "task", Label: "Item"},
		{Key: "work", Label: "Item"},
	})
	if err == nil {
		t.Error("expected error for duplicate label-only display")
	}
}

func TestNewTypeRegistry_Empty(t *testing.T) {
	_, err := NewTypeRegistry(nil)
	if err == nil {
		t.Error("expected error for empty type definitions")
	}
}

func TestTypeRegistry_AllReturnsCopy(t *testing.T) {
	reg := mustBuildTypeRegistry(t, DefaultTypeDefs())

	all := reg.All()
	all[0].Key = "mutated"

	// internal state must be unchanged
	keys := reg.Keys()
	if keys[0] != TypeStory {
		t.Errorf("All() mutation leaked into registry: first key = %q, want %q", keys[0], TypeStory)
	}
}

func TestTypeRegistry_ExplicitDefault(t *testing.T) {
	reg := mustBuildTypeRegistry(t, []TypeDef{
		{Key: "story", Label: "Story"},
		{Key: "bug", Label: "Bug", Default: true},
		{Key: "spike", Label: "Spike"},
	})
	if got := reg.DefaultType(); got != "bug" {
		t.Errorf("DefaultType() = %q, want %q", got, "bug")
	}
}

func TestTypeRegistry_MultipleDefaultsRejected(t *testing.T) {
	_, err := NewTypeRegistry([]TypeDef{
		{Key: "story", Label: "Story", Default: true},
		{Key: "bug", Label: "Bug", Default: true},
	})
	if err == nil {
		t.Fatal("expected error for multiple default types")
	}
}

func TestTypeRegistry_NoExplicitDefaultUsesFirst(t *testing.T) {
	reg := mustBuildTypeRegistry(t, []TypeDef{
		{Key: "spike", Label: "Spike"},
		{Key: "bug", Label: "Bug"},
	})
	if got := reg.DefaultType(); got != "spike" {
		t.Errorf("DefaultType() = %q, want first type %q", got, "spike")
	}
}

func TestTypeConstants(t *testing.T) {
	if TypeStory != "story" {
		t.Errorf("TypeStory = %q", TypeStory)
	}
	if TypeBug != "bug" {
		t.Errorf("TypeBug = %q", TypeBug)
	}
	if TypeSpike != "spike" {
		t.Errorf("TypeSpike = %q", TypeSpike)
	}
	if TypeEpic != "epic" {
		t.Errorf("TypeEpic = %q", TypeEpic)
	}
}
