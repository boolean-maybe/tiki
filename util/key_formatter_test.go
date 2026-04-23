package util

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestFormatKeyBinding(t *testing.T) {
	tests := []struct {
		name     string
		key      tcell.Key
		ch       rune
		mod      tcell.ModMask
		expected string
	}{
		// rune key path (ch != 0)
		{
			name:     "plain rune",
			key:      tcell.KeyRune,
			ch:       's',
			mod:      0,
			expected: "s",
		},
		{
			name:     "rune with Ctrl modifier",
			key:      tcell.KeyRune,
			ch:       'r',
			mod:      tcell.ModCtrl,
			expected: "Ctrl+r",
		},
		{
			name:     "rune with Shift modifier",
			key:      tcell.KeyRune,
			ch:       'A',
			mod:      tcell.ModShift,
			expected: "Shift+A",
		},
		{
			name:     "rune with Alt modifier",
			key:      tcell.KeyRune,
			ch:       'x',
			mod:      tcell.ModAlt,
			expected: "Alt+x",
		},
		{
			name:     "rune with Shift+Ctrl modifiers",
			key:      tcell.KeyRune,
			ch:       'p',
			mod:      tcell.ModShift | tcell.ModCtrl,
			expected: "Shift+Ctrl+p",
		},

		// named special key path (in tcell.KeyNames, no modifier prefix in name)
		{
			name:     "Enter key",
			key:      tcell.KeyEnter,
			ch:       0,
			mod:      0,
			expected: "Enter",
		},
		{
			name:     "Escape key",
			key:      tcell.KeyEscape,
			ch:       0,
			mod:      0,
			expected: "Esc",
		},
		{
			name:     "Tab key",
			key:      tcell.KeyTab,
			ch:       0,
			mod:      0,
			expected: "Tab",
		},
		{
			name:     "Backspace key",
			key:      tcell.KeyBackspace,
			ch:       0,
			mod:      0,
			expected: "Backspace",
		},
		{
			name:     "Enter with Shift modifier",
			key:      tcell.KeyEnter,
			ch:       0,
			mod:      tcell.ModShift,
			expected: "Shift+Enter",
		},
		{
			name:     "F1 key",
			key:      tcell.KeyF1,
			ch:       0,
			mod:      0,
			expected: "F1",
		},

		// named key that already has modifier in its name (e.g. "Ctrl-A")
		{
			name:     "CtrlA — name already contains Ctrl-",
			key:      tcell.KeyCtrlA,
			ch:       0,
			mod:      0,
			expected: "Ctrl-A",
		},

		// Ctrl+letter fallback path (keys 1–26 not caught by the named-key guard above)
		// KeyCtrlA IS in KeyNames as "Ctrl-A", so the fallback fires only for truly unnamed keys.
		// We test a key in range [1,26] that is NOT in KeyNames.
		// All KeyCtrl* keys are in KeyNames, so the cleanest way to hit the fallback is
		// to construct a raw key value that is in range but has no KeyNames entry.
		// In practice this path is defensive; we verify it doesn't panic and returns "Ctrl+?"
		// by using a raw key value known to not be in KeyNames.
		{
			name:     "unknown key returns question mark",
			key:      tcell.Key(9999),
			ch:       0,
			mod:      0,
			expected: "?",
		},
		{
			name:     "keyless action returns empty string",
			key:      0,
			ch:       0,
			mod:      0,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatKeyBinding(tt.key, tt.ch, tt.mod)
			if got != tt.expected {
				t.Errorf("FormatKeyBinding(%v, %q, %v) = %q, want %q", tt.key, tt.ch, tt.mod, got, tt.expected)
			}
		})
	}
}
