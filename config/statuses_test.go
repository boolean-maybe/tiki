package config

import (
	"testing"
)

func defaultTestStatuses() []StatusDef {
	return []StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "in_progress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "review", Label: "Review", Emoji: "👀", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	}
}

func setupTestRegistry(t *testing.T, defs []StatusDef) {
	t.Helper()
	ResetStatusRegistry(defs)
	t.Cleanup(func() { ClearStatusRegistry() })
}

func TestBuildRegistry_DefaultStatuses(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	if len(reg.All()) != 5 {
		t.Fatalf("expected 5 statuses, got %d", len(reg.All()))
	}

	if reg.DefaultKey() != "backlog" {
		t.Errorf("expected default key 'backlog', got %q", reg.DefaultKey())
	}

	if reg.DoneKey() != "done" {
		t.Errorf("expected done key 'done', got %q", reg.DoneKey())
	}
}

func TestBuildRegistry_CustomStatuses(t *testing.T) {
	custom := []StatusDef{
		{Key: "new", Label: "New", Emoji: "🆕", Default: true},
		{Key: "wip", Label: "Work In Progress", Emoji: "🔧", Active: true},
		{Key: "closed", Label: "Closed", Emoji: "🔒", Done: true},
	}
	setupTestRegistry(t, custom)
	reg := GetStatusRegistry()

	if len(reg.All()) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(reg.All()))
	}
	if reg.DefaultKey() != "new" {
		t.Errorf("expected default key 'new', got %q", reg.DefaultKey())
	}
	if reg.DoneKey() != "closed" {
		t.Errorf("expected done key 'closed', got %q", reg.DoneKey())
	}
}

func TestRegistry_IsValid(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	tests := []struct {
		key  string
		want bool
	}{
		{"backlog", true},
		{"ready", true},
		{"in_progress", true},
		{"In-Progress", true}, // normalization
		{"review", true},
		{"done", true},
		{"unknown", false},
		{"", false},
		{"todo", false}, // no aliases
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := reg.IsValid(tt.key); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestRegistry_IsActive(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	tests := []struct {
		key  string
		want bool
	}{
		{"backlog", false},
		{"ready", true},
		{"in_progress", true},
		{"review", true},
		{"done", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := reg.IsActive(tt.key); got != tt.want {
				t.Errorf("IsActive(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestRegistry_Lookup(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	def, ok := reg.Lookup("ready")
	if !ok {
		t.Fatal("expected to find 'ready'")
	}
	if def.Label != "Ready" {
		t.Errorf("expected label 'Ready', got %q", def.Label)
	}
	if def.Emoji != "📋" {
		t.Errorf("expected emoji '📋', got %q", def.Emoji)
	}

	_, ok = reg.Lookup("nonexistent")
	if ok {
		t.Error("expected Lookup to return false for nonexistent key")
	}
}

func TestRegistry_Keys(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	keys := reg.Keys()
	expected := []string{"backlog", "ready", "in_progress", "review", "done"}

	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, key := range keys {
		if key != expected[i] {
			t.Errorf("keys[%d] = %q, want %q", i, key, expected[i])
		}
	}
}

func TestRegistry_NormalizesKeys(t *testing.T) {
	custom := []StatusDef{
		{Key: "In-Progress", Label: "In Progress", Default: true},
		{Key: "  DONE  ", Label: "Done", Done: true},
	}
	setupTestRegistry(t, custom)
	reg := GetStatusRegistry()

	if !reg.IsValid("in_progress") {
		t.Error("expected 'in_progress' to be valid after normalization")
	}
	if !reg.IsValid("done") {
		t.Error("expected 'done' to be valid after normalization")
	}
}

func TestBuildRegistry_EmptyKey(t *testing.T) {
	defs := []StatusDef{
		{Key: "", Label: "No Key"},
	}
	_, err := buildRegistry(defs)
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestBuildRegistry_DuplicateKey(t *testing.T) {
	defs := []StatusDef{
		{Key: "ready", Label: "Ready", Default: true},
		{Key: "ready", Label: "Ready 2"},
	}
	_, err := buildRegistry(defs)
	if err == nil {
		t.Error("expected error for duplicate key")
	}
}

func TestBuildRegistry_Empty(t *testing.T) {
	_, err := buildRegistry(nil)
	if err == nil {
		t.Error("expected error for empty statuses")
	}
}

func TestBuildRegistry_DefaultFallsToFirst(t *testing.T) {
	defs := []StatusDef{
		{Key: "alpha", Label: "Alpha"},
		{Key: "beta", Label: "Beta"},
	}
	reg, err := buildRegistry(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.DefaultKey() != "alpha" {
		t.Errorf("expected default to fall back to first status 'alpha', got %q", reg.DefaultKey())
	}
}

func TestNormalizeStatusKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"backlog", "backlog"},
		{"BACKLOG", "backlog"},
		{"In-Progress", "in_progress"},
		{"in progress", "in_progress"},
		{"  DONE  ", "done"},
		{"In_Review", "in_review"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := NormalizeStatusKey(tt.input); got != tt.want {
				t.Errorf("NormalizeStatusKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRegistry_IsDone(t *testing.T) {
	setupTestRegistry(t, defaultTestStatuses())
	reg := GetStatusRegistry()

	if !reg.IsDone("done") {
		t.Error("expected 'done' to be marked as done")
	}
	if reg.IsDone("backlog") {
		t.Error("expected 'backlog' to not be marked as done")
	}
}
