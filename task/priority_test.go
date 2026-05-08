package task

import (
	"testing"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
)

// withCanonicalFields restores the canonical workflow catalog before each
// priority test. Sibling tests (notably collections_test) clear the workflow
// registry mid-suite, and since priority lookups read live workflow state,
// any test that depends on the priority enum being registered must re-seed
// the catalog rather than relying on package-init alone.
func withCanonicalFields(t *testing.T) {
	t.Helper()
	teststatuses.Init()
}

func TestPriorityDisplay(t *testing.T) {
	withCanonicalFields(t)
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"high", PriorityHigh, "High 🔴"},
		{"medium-high", PriorityMediumHigh, "Medium High 🟠"},
		{"medium", PriorityMedium, "Medium 🟡"},
		{"medium-low", PriorityMediumLow, "Medium Low 🟢"},
		{"low", PriorityLow, "Low 🔵"},
		{"unknown returns key verbatim", "bogus", "bogus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PriorityDisplay(tt.key); got != tt.expected {
				t.Errorf("PriorityDisplay(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestPriorityLabel(t *testing.T) {
	withCanonicalFields(t)
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"high", PriorityHigh, "🔴"},
		{"medium-high", PriorityMediumHigh, "🟠"},
		{"medium", PriorityMedium, "🟡"},
		{"medium-low", PriorityMediumLow, "🟢"},
		{"low", PriorityLow, "🔵"},
		{"unknown", "nope", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PriorityLabel(tt.key); got != tt.want {
				t.Errorf("PriorityLabel(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestPriorityFromDisplay(t *testing.T) {
	withCanonicalFields(t)
	tests := []struct {
		name    string
		display string
		want    string
		ok      bool
	}{
		{"high", "High 🔴", PriorityHigh, true},
		{"medium-high", "Medium High 🟠", PriorityMediumHigh, true},
		{"medium", "Medium 🟡", PriorityMedium, true},
		{"low", "Low 🔵", PriorityLow, true},
		{"unknown", "Bogus", "", false},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PriorityFromDisplay(tt.display)
			if ok != tt.ok {
				t.Errorf("PriorityFromDisplay(%q) ok = %v, want %v", tt.display, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("PriorityFromDisplay(%q) = %q, want %q", tt.display, got, tt.want)
			}
		})
	}
}

func TestAllPriorityDisplayValues(t *testing.T) {
	withCanonicalFields(t)
	got := AllPriorityDisplayValues()
	want := []string{"High 🔴", "Medium High 🟠", "Medium 🟡", "Medium Low 🟢", "Low 🔵"}
	if len(got) != len(want) {
		t.Fatalf("AllPriorityDisplayValues() len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("AllPriorityDisplayValues()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAllPriorities(t *testing.T) {
	withCanonicalFields(t)
	got := AllPriorities()
	want := []string{PriorityHigh, PriorityMediumHigh, PriorityMedium, PriorityMediumLow, PriorityLow}
	if len(got) != len(want) {
		t.Fatalf("AllPriorities() len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("AllPriorities()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDefaultPriority(t *testing.T) {
	withCanonicalFields(t)
	if got := DefaultPriority(); got != PriorityMedium {
		t.Errorf("DefaultPriority() = %q, want %q", got, PriorityMedium)
	}
}

func TestNormalizePriority(t *testing.T) {
	withCanonicalFields(t)
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"already canonical", "high", PriorityHigh},
		{"uppercase folds", "HIGH", PriorityHigh},
		{"mixed case", "Medium-High", PriorityMediumHigh},
		{"underscore separator", "medium_high", PriorityMediumHigh},
		{"space separator", "medium high", PriorityMediumHigh},
		{"alias high-medium", "high-medium", PriorityMediumHigh},
		{"alias low-medium", "low-medium", PriorityMediumLow},
		{"unknown returns empty", "bogus", ""},
		{"empty returns empty", "", ""},
		{"whitespace returns empty", "   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePriority(tt.input); got != tt.want {
				t.Errorf("NormalizePriority(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidPriority(t *testing.T) {
	withCanonicalFields(t)
	tests := []struct {
		key  string
		want bool
	}{
		{PriorityHigh, true},
		{PriorityMedium, true},
		{PriorityLow, true},
		{"bogus", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := IsValidPriority(tt.key); got != tt.want {
				t.Errorf("IsValidPriority(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
