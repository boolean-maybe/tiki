package workflow

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/theme"
)

// fakePaint is a test-only Paint that wraps text in a single tag.
type fakePaint struct{ tag string }

func (p fakePaint) PaintString(s string) string {
	if s == "" {
		return ""
	}
	return p.tag + s + "[-]"
}

// gradientFakePaint emits one tag per rune so tests can distinguish solid
// vs gradient output without exercising real gradient math.
type gradientFakePaint struct{ tag string }

func (p gradientFakePaint) PaintString(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		b.WriteString(p.tag)
		b.WriteRune(r)
	}
	b.WriteString("[-]")
	return b.String()
}

// paintResolver mimics the shape of theme.Theme.PaintResolver but uses
// fakes. Accepts only roles in workflow.ValidRoles plus the modifier set
// theme.KnownModifierNames returns.
func paintResolver(role, modifier string) (theme.Paint, bool) {
	if _, ok := ValidRoles[role]; !ok {
		return nil, false
	}
	switch modifier {
	case "":
		return fakePaint{tag: "[#" + role + "]"}, true
	case "accent", "lift":
		return gradientFakePaint{tag: "[#" + role + ":" + modifier + "]"}, true
	}
	return nil, false
}

func TestExpandVisual_BareGlyphPassthrough(t *testing.T) {
	got, err := ExpandVisual("📥", paintResolver)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "📥" {
		t.Errorf("got %q, want %q", got, "📥")
	}
}

func TestExpandVisual_Empty(t *testing.T) {
	got, err := ExpandVisual("", paintResolver)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExpandVisual_SingleRoleSolid(t *testing.T) {
	got, err := ExpandVisual("<danger>!!!", paintResolver)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := "[#danger]!!![-]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandVisual_RoleWithAccentModifier(t *testing.T) {
	got, err := ExpandVisual("<text.muted.accent>AB", paintResolver)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// gradientFakePaint emits one tag per rune.
	if !strings.Contains(got, "[#text.muted:accent]A") || !strings.Contains(got, "[#text.muted:accent]B") {
		t.Errorf("expected per-rune accent tags in %q", got)
	}
	if !strings.HasSuffix(got, "[-]") {
		t.Errorf("missing trailing reset in %q", got)
	}
}

func TestExpandVisual_RoleWithLiftModifier(t *testing.T) {
	got, err := ExpandVisual("<accent.action.lift>X", paintResolver)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(got, "[#accent.action:lift]X") {
		t.Errorf("expected lift tag in %q", got)
	}
}

func TestExpandVisual_DottedRoleWithoutModifier(t *testing.T) {
	// "text.muted" is a dotted role; the suffix is not a modifier so the
	// whole token must be treated as the role name.
	got, err := ExpandVisual("<text.muted>x", paintResolver)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := "[#text.muted]x[-]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandVisual_MultipleTokensMixed(t *testing.T) {
	got, err := ExpandVisual("<danger>!<muted>?", paintResolver)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := "[#danger]![-][#muted]?[-]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandVisual_LiteralAngleEscape(t *testing.T) {
	got, err := ExpandVisual("a<<b", paintResolver)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "a<b" {
		t.Errorf("got %q, want %q", got, "a<b")
	}
}

func TestExpandVisual_UnknownRoleErrors(t *testing.T) {
	if _, err := ExpandVisual("<nosuchrole>x", paintResolver); err == nil {
		t.Error("want error for unknown role, got nil")
	}
}

func TestExpandVisual_UnknownModifierErrors(t *testing.T) {
	if _, err := ExpandVisual("<danger.bogus>x", paintResolver); err == nil {
		t.Error("want error for unknown modifier, got nil")
	}
}

func TestValidateVisualMarkup_KnownModifiers(t *testing.T) {
	if err := ValidateVisualMarkup("<danger.accent>x"); err != nil {
		t.Errorf("unexpected err for <danger.accent>: %v", err)
	}
	if err := ValidateVisualMarkup("<danger.lift>x"); err != nil {
		t.Errorf("unexpected err for <danger.lift>: %v", err)
	}
}

func TestValidateVisualMarkup_UnknownModifier(t *testing.T) {
	if err := ValidateVisualMarkup("<danger.bogus>x"); err == nil {
		t.Error("want error for unknown modifier")
	}
}
