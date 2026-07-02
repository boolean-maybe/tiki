package theme

import (
	"fmt"
	"strings"

	gradcore "github.com/boolean-maybe/tiki/internal/gradient"
	"github.com/gdamore/tcell/v2"
)

// Paint is a render-time strategy for applying a role (optionally modified)
// to a string. Solid implementations emit a single tview color tag wrapping
// the input. Gradient implementations emit one color tag per visible rune.
// Implementations encapsulate the solid-vs-gradient branch so consumers
// remain role-blind.
type Paint interface {
	PaintString(s string) string
}

// PositionPaint yields a tcell.Color for a normalized position t in [0,1].
// Solid implementations return the same color regardless of t; gradient
// implementations interpolate. Used by screen-cell painters that need
// raw colors per column rather than a marked-up string.
type PositionPaint interface {
	ColorAt(t float64) tcell.Color
}

// solidPaint wraps a single Role.
type solidPaint struct{ role Role }

func (p solidPaint) PaintString(s string) string {
	if s == "" {
		return ""
	}
	return p.role.Tag() + s + "[-]"
}

func (p solidPaint) ColorAt(_ float64) tcell.Color { return p.role.TCell() }

// gradientAlgo derives the start/end of a gradient from a base RGB triple.
// Each modifier corresponds to one gradientAlgo.
type gradientAlgo func(base [3]int) (start, end [3]int)

// gradientPaint derives its gradient at render time from the base role's
// resolved color via algo. When gradcore.UseGradients is false, both methods
// degrade to solid behavior matching solidPaint{role: base}.
type gradientPaint struct {
	base Role
	algo gradientAlgo
}

func (p gradientPaint) PaintString(s string) string {
	if s == "" {
		return ""
	}
	if !gradcore.UseGradients.Load() {
		return p.base.Tag() + s + "[-]"
	}
	br, bg, bb := p.base.TCell().RGB()
	baseRGB := [3]int{int(br), int(bg), int(bb)}
	start, end := p.algo(baseRGB)
	return gradcore.RenderTaggedGradient(s, start, end) + "[-]"
}

func (p gradientPaint) ColorAt(t float64) tcell.Color {
	if !gradcore.UseGradients.Load() {
		return p.base.TCell()
	}
	br, bg, bb := p.base.TCell().RGB()
	baseRGB := [3]int{int(br), int(bg), int(bb)}
	start, end := p.algo(baseRGB)
	rgb := gradcore.InterpolateRGB(start, end, t)
	//nolint:gosec // G115: components are 0-255, safe int→int32 conversion
	return tcell.NewRGBColor(int32(rgb[0]), int32(rgb[1]), int32(rgb[2]))
}

// algoAccent derives a gentle gradient: start=darken(base,0.2), end=base.
// Matches the historical theme.gradientFromColor(c, 0.2) semantics
// byte-for-byte.
func algoAccent(base [3]int) (start, end [3]int) {
	return gradcore.DeriveDarkened(base, 0.2)
}

// algoLift derives a vibrant gradient: start=base, end=boost(base,1.6).
// Matches the historical GradientFromColorVibrant(c, 1.6) semantics
// byte-for-byte. Note the ordering is the inverse of algoAccent: lift's
// vivid endpoint is the *end* of the gradient, not the start.
func algoLift(base [3]int) (start, end [3]int) {
	return gradcore.DeriveVibrant(base, 1.6)
}

// paintFor returns the Paint matching base+modifier. modifier=="" → solid.
// Reports false for unknown modifiers.
func paintFor(base Role, modifier string) (Paint, bool) {
	switch modifier {
	case "":
		return solidPaint{role: base}, true
	case "accent":
		return gradientPaint{base: base, algo: algoAccent}, true
	case "lift":
		return gradientPaint{base: base, algo: algoLift}, true
	}
	return nil, false
}

// PaintForRolePosition returns a PositionPaint for base+modifier. Mirrors
// PaintResolver semantics for screen-cell consumers. Reports ok=false for
// nil base or unknown modifier.
func PaintForRolePosition(base Role, modifier string) (PositionPaint, bool) {
	if base == nil {
		return nil, false
	}
	return paintForPosition(base, modifier)
}

// paintForPosition is paintFor's sibling for screen-cell consumers.
// Same modifier vocabulary; returns PositionPaint instead of Paint.
func paintForPosition(base Role, modifier string) (PositionPaint, bool) {
	switch modifier {
	case "":
		return solidPaint{role: base}, true
	case "accent":
		return gradientPaint{base: base, algo: algoAccent}, true
	case "lift":
		return gradientPaint{base: base, algo: algoLift}, true
	}
	return nil, false
}

// KnownModifierNames returns every modifier accepted by paintFor (excluding
// the empty string which represents "no modifier"). Used by workflow load-time
// validation so the validator and resolver stay in sync.
func KnownModifierNames() []string {
	return []string{"accent", "lift"}
}

// IsKnownModifier reports whether name is one of the modifier suffixes
// accepted in `<role.modifier>` markup (currently `accent`, `lift`).
func IsKnownModifier(name string) bool {
	for _, m := range KnownModifierNames() {
		if m == name {
			return true
		}
	}
	return false
}

// SplitRoleModifier applies the last-dot disambiguation rule: when the
// suffix following the last dot is a known modifier (per
// KnownModifierNames), returns (prefix, suffix). Otherwise returns
// (token, ""). Used by every package that parses `<role.modifier>`
// markup so the rule stays consistent across surfaces.
func SplitRoleModifier(token string) (role, modifier string) {
	dot := strings.LastIndexByte(token, '.')
	if dot < 0 {
		return token, ""
	}
	suffix := token[dot+1:]
	if IsKnownModifier(suffix) {
		return token[:dot], suffix
	}
	return token, ""
}

// init asserts the disambiguation invariant: no role name (canonical or
// legacy alias) may end in a known modifier suffix. Violation would make
// the last-dot split in workflow.scanVisual ambiguous.
func init() {
	for _, role := range KnownRoleNames() {
		for _, mod := range KnownModifierNames() {
			if strings.HasSuffix(role, "."+mod) {
				panic(fmt.Sprintf("theme: role name %q ends in modifier %q — disambiguation invariant violated", role, mod))
			}
		}
	}
}
