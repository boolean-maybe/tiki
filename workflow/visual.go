package workflow

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/theme"
)

// ValidRoles is the closed vocabulary of semantic color roles accepted by
// workflow `visual:` markup. Membership is checked at workflow load time;
// unknown roles produce a load error. The runtime binding from role name to
// concrete color lives in theme.Theme.ResolveByName.
//
// Derived from theme.KnownRoleNames() so the load-time validator and the
// runtime resolver always agree on which names are accepted.
var ValidRoles = func() map[string]struct{} {
	m := make(map[string]struct{}, len(theme.KnownRoleNames()))
	for _, name := range theme.KnownRoleNames() {
		m[name] = struct{}{}
	}
	return m
}()

// ValidateVisualMarkup checks a `visual:` string for syntactic and role
// errors without resolving roles to colors. Returns nil for the empty string
// or strings with no markup tokens. Used by the workflow loader to surface
// typos in workflow.yaml at startup, long before any render call.
func ValidateVisualMarkup(markup string) error {
	if markup == "" {
		return nil
	}
	_, err := scanVisual(markup, func(role string) (string, bool) {
		if _, ok := ValidRoles[role]; !ok {
			return "", false
		}
		return "", true
	})
	return err
}

// ExpandVisual converts `<role>` markup into tview color tags using the
// caller-supplied resolver. The resolver returns the tview tag prefix for a
// role (e.g. "[#ff4444]") and reports false for unknown roles. The result
// ends with "[-]" when any role token was emitted, otherwise the original
// markup is returned unchanged (fast path for bare glyphs like "📥").
//
// Escape rule: literal "<" is written as "<<".
func ExpandVisual(markup string, resolve func(role string) (tag string, ok bool)) (string, error) {
	if markup == "" {
		return "", nil
	}
	if !strings.ContainsRune(markup, '<') {
		return markup, nil
	}
	return scanVisual(markup, resolve)
}

// scanVisual is the shared tokenizer used by both ValidateVisualMarkup and
// ExpandVisual. The resolver's tag return value is appended to the output;
// validation mode passes an empty tag and only cares about the bool.
func scanVisual(markup string, resolve func(role string) (tag string, ok bool)) (string, error) {
	var out strings.Builder
	out.Grow(len(markup) + 16)
	emittedTag := false

	i := 0
	for i < len(markup) {
		c := markup[i]
		if c != '<' {
			out.WriteByte(c)
			i++
			continue
		}
		// '<<' → literal '<'
		if i+1 < len(markup) && markup[i+1] == '<' {
			out.WriteByte('<')
			i += 2
			continue
		}
		// Find matching '>'.
		close := strings.IndexByte(markup[i+1:], '>')
		if close < 0 {
			return "", fmt.Errorf("unclosed `<` in visual markup %q", markup)
		}
		role := markup[i+1 : i+1+close]
		if role == "" {
			return "", fmt.Errorf("empty role name in visual markup %q", markup)
		}
		tag, ok := resolve(role)
		if !ok {
			return "", fmt.Errorf("unknown visual role %q in markup %q", role, markup)
		}
		if tag != "" {
			out.WriteString(tag)
			emittedTag = true
		}
		i += 1 + close + 1
	}

	if emittedTag {
		out.WriteString("[-]")
	}
	return out.String(), nil
}
