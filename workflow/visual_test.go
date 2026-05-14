package workflow

import (
	"strings"
	"testing"
)

// tagResolver returns a deterministic tag per known role so tests can assert
// on the exact substitution. Unknown roles report false.
func tagResolver(role string) (string, bool) {
	if _, ok := ValidRoles[role]; !ok {
		return "", false
	}
	return "[" + role + "]", true
}

func TestExpandVisual_BareGlyphPassthrough(t *testing.T) {
	got, err := ExpandVisual("📥", tagResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "📥" {
		t.Errorf("got %q, want %q", got, "📥")
	}
}

func TestExpandVisual_Empty(t *testing.T) {
	got, err := ExpandVisual("", tagResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestExpandVisual_SingleRole(t *testing.T) {
	got, err := ExpandVisual("{danger}!!!", tagResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "[danger]!!![-]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandVisual_MultipleRoles(t *testing.T) {
	got, err := ExpandVisual("{accent}❚❚❚{muted}❘❘", tagResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "[accent]❚❚❚[muted]❘❘[-]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandVisual_LiteralBraceEscape(t *testing.T) {
	got, err := ExpandVisual("a{{b", tagResolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "a{b" {
		t.Errorf("got %q, want %q", got, "a{b")
	}
}

func TestExpandVisual_UnknownRoleErrors(t *testing.T) {
	_, err := ExpandVisual("{nosuchrole}x", tagResolver)
	if err == nil {
		t.Fatal("expected error for unknown role")
	}
	if !strings.Contains(err.Error(), "nosuchrole") {
		t.Errorf("error %q does not mention role name", err.Error())
	}
}

func TestExpandVisual_UnclosedBraceErrors(t *testing.T) {
	_, err := ExpandVisual("a{danger", tagResolver)
	if err == nil {
		t.Fatal("expected error for unclosed brace")
	}
	if !strings.Contains(err.Error(), "unclosed") {
		t.Errorf("error %q does not mention unclosed", err.Error())
	}
}

func TestExpandVisual_EmptyRoleErrors(t *testing.T) {
	_, err := ExpandVisual("a{}b", tagResolver)
	if err == nil {
		t.Fatal("expected error for empty role")
	}
}

func TestValidateVisualMarkup_AcceptsBareGlyph(t *testing.T) {
	if err := ValidateVisualMarkup("📥"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateVisualMarkup_AcceptsEmpty(t *testing.T) {
	if err := ValidateVisualMarkup(""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateVisualMarkup_AcceptsKnownRoles(t *testing.T) {
	for role := range ValidRoles {
		t.Run(role, func(t *testing.T) {
			if err := ValidateVisualMarkup("{" + role + "}x"); err != nil {
				t.Errorf("role %q rejected: %v", role, err)
			}
		})
	}
}

func TestValidateVisualMarkup_RejectsUnknownRole(t *testing.T) {
	err := ValidateVisualMarkup("{purple}x")
	if err == nil {
		t.Fatal("expected error for unknown role")
	}
	if !strings.Contains(err.Error(), "purple") {
		t.Errorf("error %q does not mention bad role", err.Error())
	}
}

func TestValidateVisualMarkup_RejectsUnclosedBrace(t *testing.T) {
	if err := ValidateVisualMarkup("{danger"); err == nil {
		t.Error("expected error for unclosed brace")
	}
}
