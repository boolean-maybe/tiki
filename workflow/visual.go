package workflow

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/theme"
)

// ValidRoles is the closed vocabulary of semantic color role names accepted
// in `<role>` markup. Derived from theme.KnownRoleNames so the load-time
// validator and the runtime resolver always agree.
var ValidRoles = func() map[string]struct{} {
	m := make(map[string]struct{}, len(theme.KnownRoleNames()))
	for _, name := range theme.KnownRoleNames() {
		m[name] = struct{}{}
	}
	return m
}()

// ValidateVisualMarkup checks a `visual:` (or any role-markup) string for
// syntactic and vocabulary errors without resolving colors. Returns nil for
// the empty string or strings with no markup tokens. Used by the workflow
// loader to surface typos in workflow.yaml at startup.
//
// Grammar: `<role>` or `<role.modifier>`. The modifier (if present) must be
// a member of theme.KnownModifierNames. The role must be a member of
// ValidRoles. The literal-`<` escape is `<<`.
func ValidateVisualMarkup(markup string) error {
	if markup == "" {
		return nil
	}
	i := 0
	for i < len(markup) {
		c := markup[i]
		if c != '<' {
			i++
			continue
		}
		if i+1 < len(markup) && markup[i+1] == '<' {
			i += 2
			continue
		}
		closeIdx := strings.IndexByte(markup[i+1:], '>')
		if closeIdx < 0 {
			return fmt.Errorf("unclosed `<` in visual markup %q", markup)
		}
		token := markup[i+1 : i+1+closeIdx]
		if token == "" {
			return fmt.Errorf("empty role name in visual markup %q", markup)
		}
		role, modifier := theme.SplitRoleModifier(token)
		if _, ok := ValidRoles[role]; !ok {
			return fmt.Errorf("unknown visual role %q in markup %q", role, markup)
		}
		if modifier != "" {
			if !theme.IsKnownModifier(modifier) {
				return fmt.Errorf("unknown visual modifier %q in markup %q", modifier, markup)
			}
		}
		i += 1 + closeIdx + 1
	}
	return nil
}

// ExpandVisual converts `<role>` and `<role.modifier>` markup into tview
// color-tagged output using the caller-supplied resolver. The resolver
// returns a Paint that knows how to render the text span following its
// token; ExpandVisual writes the resolver's output verbatim. Reports an
// error for unknown role names, unknown modifiers, unclosed `<`, or empty
// role names.
//
// Disambiguation: when a token contains dots, the suffix following the
// last dot is checked against theme.KnownModifierNames; if it is a known
// modifier, the prefix is the role and the suffix is the modifier.
// Otherwise the entire token is the role name. This rule keeps existing
// dotted role names (e.g. "text.muted") working unchanged.
//
// Escape: literal `<` is written as `<<`.
func ExpandVisual(markup string, resolve func(role, modifier string) (theme.Paint, bool)) (string, error) {
	if markup == "" {
		return "", nil
	}
	if !strings.ContainsRune(markup, '<') {
		return markup, nil
	}
	return scanVisual(markup, resolve)
}

// scanVisual tokenizes role markup and, for each token, hands the run of
// text that follows the token (up to the next token or end of input) to
// the Paint returned by the resolver. The resolver receives (role,
// modifier) pairs and returns the Paint to apply.
func scanVisual(markup string, resolve func(role, modifier string) (theme.Paint, bool)) (string, error) {
	var out strings.Builder
	out.Grow(len(markup) + 32)

	i := 0
	// pending paint applies to the next plain-text run.
	var pending theme.Paint
	pendingRunStart := -1

	flushRun := func(end int) {
		if pendingRunStart < 0 {
			return
		}
		text := markup[pendingRunStart:end]
		if pending != nil {
			out.WriteString(pending.PaintString(text))
		} else {
			// text before any token (or after `<<` with no preceding token):
			// emit verbatim. The validator path no longer reaches scanVisual
			// at all, so this branch is reached only by ExpandVisual.
			out.WriteString(text)
		}
		pending = nil
		pendingRunStart = -1
	}

	for i < len(markup) {
		c := markup[i]
		if c != '<' {
			if pendingRunStart < 0 {
				pendingRunStart = i
			}
			i++
			continue
		}
		// `<<` → literal `<`
		if i+1 < len(markup) && markup[i+1] == '<' {
			// End the current run at this `<`, emit a literal `<`, continue.
			flushRun(i)
			out.WriteByte('<')
			i += 2
			pendingRunStart = -1
			continue
		}
		// End the previous run (if any) at this `<`.
		flushRun(i)

		closeIdx := strings.IndexByte(markup[i+1:], '>')
		if closeIdx < 0 {
			return "", fmt.Errorf("unclosed `<` in visual markup %q", markup)
		}
		token := markup[i+1 : i+1+closeIdx]
		if token == "" {
			return "", fmt.Errorf("empty role name in visual markup %q", markup)
		}
		role, modifier := theme.SplitRoleModifier(token)
		paint, ok := resolve(role, modifier)
		if !ok {
			if modifier != "" {
				return "", fmt.Errorf("unknown visual role/modifier %q.%q in markup %q", role, modifier, markup)
			}
			return "", fmt.Errorf("unknown visual role %q in markup %q", role, markup)
		}
		pending = paint
		pendingRunStart = -1
		i += 1 + closeIdx + 1
	}
	flushRun(len(markup))
	return out.String(), nil
}
