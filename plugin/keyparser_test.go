package plugin

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestParseKey(t *testing.T) {
	t.Run("single rune upper", func(t *testing.T) {
		k, r, m, err := parseKey("B")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyRune || r != 'B' || m != 0 {
			t.Fatalf("Expected (KeyRune,'B',0), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("single rune lower preserved", func(t *testing.T) {
		k, r, m, err := parseKey("b")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyRune || r != 'b' || m != 0 {
			t.Fatalf("Expected (KeyRune,'b',0), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("ctrl dash upper", func(t *testing.T) {
		k, r, m, err := parseKey("Ctrl-U")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyCtrlU || r != 0 || m != tcell.ModCtrl {
			t.Fatalf("Expected (KeyCtrlU,0,ModCtrl), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("ctrl dash lower", func(t *testing.T) {
		k, r, m, err := parseKey("ctrl-u")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyCtrlU || r != 0 || m != tcell.ModCtrl {
			t.Fatalf("Expected (KeyCtrlU,0,ModCtrl), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("alt dash upper", func(t *testing.T) {
		k, r, m, err := parseKey("Alt-M")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyRune || r != 'M' || m != tcell.ModAlt {
			t.Fatalf("Expected (KeyRune,'M',ModAlt), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("alt dash lower", func(t *testing.T) {
		k, r, m, err := parseKey("alt-m")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyRune || r != 'M' || m != tcell.ModAlt {
			t.Fatalf("Expected (KeyRune,'M',ModAlt), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("shift dash upper", func(t *testing.T) {
		k, r, m, err := parseKey("Shift-F")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyRune || r != 'F' || m != tcell.ModShift {
			t.Fatalf("Expected (KeyRune,'F',ModShift), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("shift dash lower", func(t *testing.T) {
		k, r, m, err := parseKey("shift-f")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyRune || r != 'F' || m != tcell.ModShift {
			t.Fatalf("Expected (KeyRune,'F',ModShift), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("function key F1", func(t *testing.T) {
		k, r, m, err := parseKey("F1")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyF1 || r != 0 || m != 0 {
			t.Fatalf("Expected (KeyF1,0,0), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("function key F12", func(t *testing.T) {
		k, r, m, err := parseKey("F12")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyF12 || r != 0 || m != 0 {
			t.Fatalf("Expected (KeyF12,0,0), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("function key f5 lowercase", func(t *testing.T) {
		k, r, m, err := parseKey("f5")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyF5 || r != 0 || m != 0 {
			t.Fatalf("Expected (KeyF5,0,0), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("ctrl-F1", func(t *testing.T) {
		k, r, m, err := parseKey("Ctrl-F1")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyF1 || r != 0 || m != tcell.ModCtrl {
			t.Fatalf("Expected (KeyF1,0,ModCtrl), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("alt-F2", func(t *testing.T) {
		k, r, m, err := parseKey("Alt-F2")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyF2 || r != 0 || m != tcell.ModAlt {
			t.Fatalf("Expected (KeyF2,0,ModAlt), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("shift-F10", func(t *testing.T) {
		k, r, m, err := parseKey("Shift-F10")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if k != tcell.KeyF10 || r != 0 || m != tcell.ModShift {
			t.Fatalf("Expected (KeyF10,0,ModShift), got (%v,%q,%v)", k, r, m)
		}
	})

	t.Run("invalid F0", func(t *testing.T) {
		_, _, _, err := parseKey("F0")
		if err == nil {
			t.Fatalf("Expected error for F0, got nil")
		}
	})

	t.Run("invalid F13", func(t *testing.T) {
		_, _, _, err := parseKey("F13")
		if err == nil {
			t.Fatalf("Expected error for F13, got nil")
		}
	})

	t.Run("invalid ctrl-1", func(t *testing.T) {
		_, _, _, err := parseKey("Ctrl-1")
		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("invalid alt-1", func(t *testing.T) {
		_, _, _, err := parseKey("Alt-1")
		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("invalid shift-1", func(t *testing.T) {
		_, _, _, err := parseKey("Shift-1")
		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("invalid multi rune", func(t *testing.T) {
		_, _, _, err := parseKey("AB")
		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})
}

func TestParseFunctionKey(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKey   tcell.Key
		wantValid bool
	}{
		{"F1", "F1", tcell.KeyF1, true},
		{"F12", "F12", tcell.KeyF12, true},
		{"F5", "F5", tcell.KeyF5, true},
		{"not F key", "G1", 0, false},
		{"F0 invalid", "F0", 0, false},
		{"F13 invalid", "F13", 0, false},
		{"empty after F", "F", 0, false},
		{"no F prefix", "1", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotValid := parseFunctionKey(tt.input)
			if gotKey != tt.wantKey || gotValid != tt.wantValid {
				t.Errorf("parseFunctionKey(%q) = (%v, %v), want (%v, %v)",
					tt.input, gotKey, gotValid, tt.wantKey, tt.wantValid)
			}
		})
	}
}

func TestKeyName(t *testing.T) {
	tests := []struct {
		name string
		key  tcell.Key
		r    rune
		want string
	}{
		{"rune key", tcell.KeyRune, 'A', "A"},
		{"rune lowercase", tcell.KeyRune, 'b', "b"},
		{"special char", tcell.KeyRune, '?', "?"},
		{"F1 key", tcell.KeyF1, 0, "F1"},
		{"Ctrl-R key", tcell.KeyCtrlR, 0, "Ctrl-R"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyName(tt.key, tt.r)
			if got != tt.want {
				t.Errorf("keyName(%v, %q) = %q, want %q", tt.key, tt.r, got, tt.want)
			}
		})
	}
}
