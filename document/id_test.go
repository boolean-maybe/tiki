package document

import "testing"

func TestIsValidID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"ABC123", true},
		{"000000", true},
		{"ZZZZZZ", true},
		{"abc123", false}, // lowercase not accepted
		{"ABC12", false},  // too short
		{"ABC1234", false},
		{"ABC12!", false},
		{"", false},
		{"TIKI-ABC123", false}, // legacy form is NOT a valid bare ID
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := IsValidID(tt.id); got != tt.want {
				t.Errorf("IsValidID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestNewID(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		id := NewID()
		if !IsValidID(id) {
			t.Fatalf("NewID() returned invalid id %q", id)
		}
		if _, dup := seen[id]; dup {
			// Collisions are theoretically possible but should be vanishingly rare
			// at 36^6; treat as a generator quality signal, not a hard failure.
			t.Logf("collision after %d ids: %q (acceptable but rare)", i, id)
		}
		seen[id] = struct{}{}
	}
}

func TestNormalizeID(t *testing.T) {
	if got := NormalizeID("  abc123  "); got != "ABC123" {
		t.Errorf("NormalizeID trimmed+upper want ABC123, got %q", got)
	}
}

func TestIndex(t *testing.T) {
	ix := NewIndex()

	if err := ix.Register("ABC123", "/tmp/a.md"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if !ix.Contains("abc123") {
		t.Error("Contains should be case-insensitive via NormalizeID")
	}
	if got := ix.PathFor("abc123"); got != "/tmp/a.md" {
		t.Errorf("PathFor want /tmp/a.md, got %q", got)
	}

	// Same id, same path is idempotent.
	if err := ix.Register("ABC123", "/tmp/a.md"); err != nil {
		t.Errorf("re-register same path should be ok, got %v", err)
	}

	// Same id, different path is an error.
	if err := ix.Register("ABC123", "/tmp/b.md"); err == nil {
		t.Error("expected duplicate-id error when re-registering at different path")
	}

	ix.Unregister("ABC123")
	if ix.Contains("ABC123") {
		t.Error("Unregister did not remove id")
	}
}
