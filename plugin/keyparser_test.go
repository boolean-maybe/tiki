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

func TestNormalizeParsedKey(t *testing.T) {
	tests := []struct {
		name    string
		key     tcell.Key
		r       rune
		mod     tcell.ModMask
		wantKey tcell.Key
		wantR   rune
		wantMod tcell.ModMask
	}{
		{"Shift-X drops ModShift", tcell.KeyRune, 'X', tcell.ModShift, tcell.KeyRune, 'X', 0},
		{"plain X unchanged", tcell.KeyRune, 'X', 0, tcell.KeyRune, 'X', 0},
		{"lowercase x unchanged", tcell.KeyRune, 'x', 0, tcell.KeyRune, 'x', 0},
		{"Shift-F3 kept", tcell.KeyF3, 0, tcell.ModShift, tcell.KeyF3, 0, tcell.ModShift},
		{"Ctrl-U unchanged", tcell.KeyCtrlU, 0, tcell.ModCtrl, tcell.KeyCtrlU, 0, tcell.ModCtrl},
		{"Alt-M unchanged", tcell.KeyRune, 'M', tcell.ModAlt, tcell.KeyRune, 'M', tcell.ModAlt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotR, gotMod := normalizeParsedKey(tt.key, tt.r, tt.mod)
			if gotKey != tt.wantKey || gotR != tt.wantR || gotMod != tt.wantMod {
				t.Errorf("normalizeParsedKey(%v, %q, %v) = (%v, %q, %v), want (%v, %q, %v)",
					tt.key, tt.r, tt.mod, gotKey, gotR, gotMod, tt.wantKey, tt.wantR, tt.wantMod)
			}
		})
	}
}

func TestFormatKeyStr(t *testing.T) {
	tests := []struct {
		name string
		key  tcell.Key
		r    rune
		mod  tcell.ModMask
		want string
	}{
		{"plain rune", tcell.KeyRune, 'A', 0, "A"},
		{"lowercase rune", tcell.KeyRune, 'b', 0, "b"},
		{"Ctrl-U with ModCtrl", tcell.KeyCtrlU, 0, tcell.ModCtrl, "Ctrl-U"},
		{"Ctrl-U without ModCtrl", tcell.KeyCtrlU, 0, 0, "Ctrl-U"},
		{"Alt-M", tcell.KeyRune, 'M', tcell.ModAlt, "Alt-M"},
		{"Shift-F3", tcell.KeyF3, 0, tcell.ModShift, "Shift-F3"},
		{"F5", tcell.KeyF5, 0, 0, "F5"},
		{"F1", tcell.KeyF1, 0, 0, "F1"},
		{"F12", tcell.KeyF12, 0, 0, "F12"},
		{"Ctrl-F1", tcell.KeyF1, 0, tcell.ModCtrl, "Ctrl-F1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatKeyStr(tt.key, tt.r, tt.mod)
			if got != tt.want {
				t.Errorf("formatKeyStr(%v, %q, %v) = %q, want %q", tt.key, tt.r, tt.mod, got, tt.want)
			}
		})
	}
}

func TestParseCanonicalKey(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantKey    tcell.Key
		wantRune   rune
		wantMod    tcell.ModMask
		wantKeyStr string
		wantErr    bool
	}{
		{"plain A", "A", tcell.KeyRune, 'A', 0, "A", false},
		{"plain b", "b", tcell.KeyRune, 'b', 0, "b", false},
		{"Ctrl-U", "Ctrl-U", tcell.KeyCtrlU, 0, tcell.ModCtrl, "Ctrl-U", false},
		{"ctrl-u lowercase", "ctrl-u", tcell.KeyCtrlU, 0, tcell.ModCtrl, "Ctrl-U", false},
		{"Alt-M", "Alt-M", tcell.KeyRune, 'M', tcell.ModAlt, "Alt-M", false},
		{"alt-m lowercase", "alt-m", tcell.KeyRune, 'M', tcell.ModAlt, "Alt-M", false},
		{"Shift-x normalizes", "Shift-x", tcell.KeyRune, 'X', 0, "X", false},
		{"Shift-X normalizes", "Shift-X", tcell.KeyRune, 'X', 0, "X", false},
		{"X same as Shift-X", "X", tcell.KeyRune, 'X', 0, "X", false},
		{"x stays distinct", "x", tcell.KeyRune, 'x', 0, "x", false},
		{"F5", "F5", tcell.KeyF5, 0, 0, "F5", false},
		{"Shift-F3", "Shift-F3", tcell.KeyF3, 0, tcell.ModShift, "Shift-F3", false},
		{"empty returns zeros", "", 0, 0, 0, "", false},
		{"invalid multi-char", "AB", 0, 0, 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, r, mod, keyStr, err := parseCanonicalKey(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key != tt.wantKey || r != tt.wantRune || mod != tt.wantMod || keyStr != tt.wantKeyStr {
				t.Errorf("parseCanonicalKey(%q) = (%v, %q, %v, %q), want (%v, %q, %v, %q)",
					tt.input, key, r, mod, keyStr, tt.wantKey, tt.wantRune, tt.wantMod, tt.wantKeyStr)
			}
		})
	}

	// verify case-insensitive input yields identical canonical output
	t.Run("case-insensitive Ctrl", func(t *testing.T) {
		_, _, _, ks1, _ := parseCanonicalKey("Ctrl-U")
		_, _, _, ks2, _ := parseCanonicalKey("ctrl-u")
		_, _, _, ks3, _ := parseCanonicalKey("CTRL-U")
		if ks1 != ks2 || ks2 != ks3 {
			t.Errorf("expected identical KeyStr, got %q, %q, %q", ks1, ks2, ks3)
		}
	})

	// verify Shift-x, Shift-X, and X all produce the same KeyStr
	t.Run("Shift-letter aliases produce same KeyStr", func(t *testing.T) {
		_, _, _, ks1, _ := parseCanonicalKey("Shift-x")
		_, _, _, ks2, _ := parseCanonicalKey("Shift-X")
		_, _, _, ks3, _ := parseCanonicalKey("X")
		if ks1 != ks2 || ks2 != ks3 {
			t.Errorf("expected identical KeyStr, got %q, %q, %q", ks1, ks2, ks3)
		}
	})

	// verify x is distinct from X
	t.Run("lowercase x distinct from X", func(t *testing.T) {
		_, _, _, ksLower, _ := parseCanonicalKey("x")
		_, _, _, ksUpper, _ := parseCanonicalKey("X")
		if ksLower == ksUpper {
			t.Error("expected 'x' and 'X' to have different KeyStr")
		}
	})

	// verify non-printable standalone rune is rejected
	t.Run("non-printable standalone rune rejected", func(t *testing.T) {
		_, _, _, _, err := parseCanonicalKey("\x01")
		if err == nil {
			t.Fatal("expected error for non-printable rune")
		}
	})
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
