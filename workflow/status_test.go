package workflow

import "testing"

func defaultTestStatuses() []StatusDef {
	return []StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "in_progress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "review", Label: "Review", Emoji: "👀", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	}
}

func mustBuildStatusRegistry(t *testing.T, defs []StatusDef) *StatusRegistry {
	t.Helper()
	reg, err := NewStatusRegistry(defs)
	if err != nil {
		t.Fatalf("NewStatusRegistry: %v", err)
	}
	return reg
}

func TestNormalizeStatusKey(t *testing.T) {
	tests := []struct {
		input string
		want  StatusKey
	}{
		{"backlog", "backlog"},
		{"BACKLOG", "backlog"},
		{"In-Progress", "in_progress"},
		{"in progress", "in_progress"},
		{"  DONE  ", "done"},
		{"In_Review", "in_review"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := NormalizeStatusKey(tt.input); got != tt.want {
				t.Errorf("NormalizeStatusKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewStatusRegistry_DefaultStatuses(t *testing.T) {
	reg := mustBuildStatusRegistry(t, defaultTestStatuses())

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

func TestNewStatusRegistry_CustomStatuses(t *testing.T) {
	custom := []StatusDef{
		{Key: "new", Label: "New", Emoji: "🆕", Default: true},
		{Key: "wip", Label: "Work In Progress", Emoji: "🔧", Active: true},
		{Key: "closed", Label: "Closed", Emoji: "🔒", Done: true},
	}
	reg := mustBuildStatusRegistry(t, custom)

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

func TestStatusRegistry_IsValid(t *testing.T) {
	reg := mustBuildStatusRegistry(t, defaultTestStatuses())

	tests := []struct {
		key  string
		want bool
	}{
		{"backlog", true},
		{"ready", true},
		{"in_progress", true},
		{"In-Progress", true},
		{"review", true},
		{"done", true},
		{"unknown", false},
		{"", false},
		{"todo", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := reg.IsValid(tt.key); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestStatusRegistry_IsActive(t *testing.T) {
	reg := mustBuildStatusRegistry(t, defaultTestStatuses())

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

func TestStatusRegistry_IsDone(t *testing.T) {
	reg := mustBuildStatusRegistry(t, defaultTestStatuses())

	if !reg.IsDone("done") {
		t.Error("expected 'done' to be marked as done")
	}
	if reg.IsDone("backlog") {
		t.Error("expected 'backlog' to not be marked as done")
	}
}

func TestStatusRegistry_Lookup(t *testing.T) {
	reg := mustBuildStatusRegistry(t, defaultTestStatuses())

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

func TestStatusRegistry_Keys(t *testing.T) {
	reg := mustBuildStatusRegistry(t, defaultTestStatuses())

	keys := reg.Keys()
	expected := []StatusKey{"backlog", "ready", "in_progress", "review", "done"}

	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, key := range keys {
		if key != expected[i] {
			t.Errorf("keys[%d] = %q, want %q", i, key, expected[i])
		}
	}
}

func TestStatusRegistry_NormalizesKeys(t *testing.T) {
	custom := []StatusDef{
		{Key: "In-Progress", Label: "In Progress", Default: true},
		{Key: "  DONE  ", Label: "Done", Done: true},
	}
	reg := mustBuildStatusRegistry(t, custom)

	if !reg.IsValid("in_progress") {
		t.Error("expected 'in_progress' to be valid after normalization")
	}
	if !reg.IsValid("done") {
		t.Error("expected 'done' to be valid after normalization")
	}
}

func TestNewStatusRegistry_EmptyKey(t *testing.T) {
	defs := []StatusDef{
		{Key: "", Label: "No Key"},
	}
	_, err := NewStatusRegistry(defs)
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestNewStatusRegistry_DuplicateKey(t *testing.T) {
	defs := []StatusDef{
		{Key: "ready", Label: "Ready", Default: true},
		{Key: "ready", Label: "Ready 2"},
	}
	_, err := NewStatusRegistry(defs)
	if err == nil {
		t.Error("expected error for duplicate key")
	}
}

func TestNewStatusRegistry_Empty(t *testing.T) {
	_, err := NewStatusRegistry(nil)
	if err == nil {
		t.Error("expected error for empty statuses")
	}
}

func TestNewStatusRegistry_DefaultFallsToFirst(t *testing.T) {
	defs := []StatusDef{
		{Key: "alpha", Label: "Alpha"},
		{Key: "beta", Label: "Beta"},
	}
	reg, err := NewStatusRegistry(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.DefaultKey() != "alpha" {
		t.Errorf("expected default to fall back to first status 'alpha', got %q", reg.DefaultKey())
	}
}

func TestStatusRegistry_AllReturnsCopy(t *testing.T) {
	reg := mustBuildStatusRegistry(t, defaultTestStatuses())

	all := reg.All()
	all[0].Key = "mutated"

	// internal state must be unchanged
	keys := reg.Keys()
	if keys[0] != "backlog" {
		t.Errorf("All() mutation leaked into registry: first key = %q, want %q", keys[0], "backlog")
	}
}

func TestNewStatusRegistry_MultipleDoneWarns(t *testing.T) {
	defs := []StatusDef{
		{Key: "alpha", Label: "Alpha", Default: true, Done: true},
		{Key: "beta", Label: "Beta", Done: true},
	}
	reg, err := NewStatusRegistry(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// first done wins
	if reg.DoneKey() != "alpha" {
		t.Errorf("expected done key 'alpha', got %q", reg.DoneKey())
	}
}

func TestNewStatusRegistry_MultipleDefaultWarns(t *testing.T) {
	defs := []StatusDef{
		{Key: "alpha", Label: "Alpha", Default: true},
		{Key: "beta", Label: "Beta", Default: true},
	}
	reg, err := NewStatusRegistry(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// first default wins
	if reg.DefaultKey() != "alpha" {
		t.Errorf("expected default key 'alpha', got %q", reg.DefaultKey())
	}
}

func TestNewStatusRegistry_NoDoneKey(t *testing.T) {
	defs := []StatusDef{
		{Key: "alpha", Label: "Alpha", Default: true},
		{Key: "beta", Label: "Beta"},
	}
	reg, err := NewStatusRegistry(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.DoneKey() != "" {
		t.Errorf("expected empty done key, got %q", reg.DoneKey())
	}
	// IsDone should return false for all statuses
	if reg.IsDone("alpha") {
		t.Error("expected alpha to not be done")
	}
}

func TestStatusKeyConstants(t *testing.T) {
	if StatusBacklog != "backlog" {
		t.Errorf("StatusBacklog = %q", StatusBacklog)
	}
	if StatusReady != "ready" {
		t.Errorf("StatusReady = %q", StatusReady)
	}
	if StatusInProgress != "in_progress" {
		t.Errorf("StatusInProgress = %q", StatusInProgress)
	}
	if StatusReview != "review" {
		t.Errorf("StatusReview = %q", StatusReview)
	}
	if StatusDone != "done" {
		t.Errorf("StatusDone = %q", StatusDone)
	}
}
