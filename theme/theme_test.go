package theme

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestRolesPanicsBeforeSetTheme(t *testing.T) {
	// reset
	globalTheme.Store(nil)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic, got none")
		}
	}()
	_ = Roles()
}

func TestSetThemeAndRoles(t *testing.T) {
	red := newColorRole(NewColor(tcell.ColorRed))
	th := &Theme{textPrimary: red}
	SetTheme(th)
	if got := Roles().TextPrimary(); got != red {
		t.Errorf("Roles().TextPrimary() = %v, want %v", got, red)
	}
	// reset for other tests
	globalTheme.Store(nil)
}
