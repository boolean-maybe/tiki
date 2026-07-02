package theme

import "testing"

func TestColorHex(t *testing.T) {
	c := NewColorHex("#ff0000")
	if got, want := c.Hex(), "#ff0000"; got != want {
		t.Errorf("Hex() = %q, want %q", got, want)
	}
}

func TestColorDefault(t *testing.T) {
	c := DefaultColor()
	if !c.IsDefault() {
		t.Errorf("DefaultColor().IsDefault() = false, want true")
	}
	if got, want := c.Hex(), "-"; got != want {
		t.Errorf("Hex() = %q, want %q", got, want)
	}
}

func TestColorTag(t *testing.T) {
	c := NewColorHex("#ff0000")
	if got, want := c.Tag().String(), "[#ff0000]"; got != want {
		t.Errorf("Tag().String() = %q, want %q", got, want)
	}
}

func TestColorTagBold(t *testing.T) {
	c := NewColorHex("#ff0000")
	// Note: bg slot renders as "-" when no bg is set — matches the existing
	// config.Color tag format, required for byte-identical output post-refactor.
	if got, want := c.Tag().Bold().String(), "[#ff0000:-:b]"; got != want {
		t.Errorf("Tag().Bold().String() = %q, want %q", got, want)
	}
}
