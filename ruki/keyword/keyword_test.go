package keyword

import (
	"regexp"
	"testing"
)

func TestIsReserved(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"select", true},
		{"SELECT", true},
		{"SeLeCt", true},
		{"where", true},
		{"and", true},
		{"old", true},
		{"new", true},
		{"title", false},
		{"priority", false},
		{"foobar", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsReserved(tt.name); got != tt.want {
			t.Errorf("IsReserved(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestList_ReturnsCopy(t *testing.T) {
	list := List()
	if len(list) != len(reserved) {
		t.Fatalf("List() returned %d keywords, want %d", len(list), len(reserved))
	}

	// mutate returned slice — internal state must be unaffected
	list[0] = "MUTATED"
	fresh := List()
	if fresh[0] == "MUTATED" {
		t.Fatal("mutating List() result affected internal state")
	}
}

func TestPattern(t *testing.T) {
	pat := Pattern()

	// must be a valid regex
	re, err := regexp.Compile(pat)
	if err != nil {
		t.Fatalf("Pattern() is not valid regex: %v", err)
	}

	// must match all reserved keywords
	for _, kw := range reserved {
		if !re.MatchString(kw) {
			t.Errorf("Pattern() does not match keyword %q", kw)
		}
	}

	// must not match non-keywords
	if re.MatchString("foobar") {
		t.Error("Pattern() should not match 'foobar'")
	}
}
