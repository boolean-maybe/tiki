# Color Role Modifiers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `workflow.yaml` author gradients via `<role.accent>` / `<role.lift>` modifiers without naming endpoints, while consolidating the solid-vs-gradient branch into two `Paint` implementations and deleting the parallel `GradientRole` API.

**Architecture:** Add `theme.Paint` (string painter) and `theme.PositionPaint` (position-based painter) interfaces, both satisfied by `solidPaint` and `gradientPaint`. A modifier→algorithm table (`paintFor`) is the single point of truth tying grammar to implementation. `workflow.ExpandVisual` resolves tokens to `Paint` via a new `Theme.PaintResolver()`. All current `theme.GradientRole` consumers (task ID rendering in three places, caption row background) migrate to the new interfaces. Per-palette caption-gradient endpoint fields and the dedicated `tikiIDGradient` / `captionFallbackGradient` theme fields are deleted.

**Tech Stack:** Go 1.x, `tview`, `tcell/v2`, existing `util/gradient` interpolation math.

**Spec:** `docs/superpowers/specs/2026-05-16-color-role-modifiers-design.md`

---

## Branch setup

This work runs on a new feature branch off `dev`. The branch must be created before the first commit. The `using-git-worktrees` skill picks the right path.

### Task 0: Create the feature branch in a worktree

**Files:**
- New worktree at: `.claude/worktrees/color-role-modifiers/`

- [ ] **Step 1: Verify current branch is `dev` and tree is clean for new worktree creation**

Run from the repo root:
```bash
git status -s
git rev-parse --abbrev-ref HEAD
```
Expected: current branch is `dev` (working tree may show untracked files; that is fine — those will not move to the worktree).

- [ ] **Step 2: Create worktree with the descriptive branch name**

```bash
git worktree add -b feature/color-role-modifiers .claude/worktrees/color-role-modifiers dev
cd .claude/worktrees/color-role-modifiers
git rev-parse --abbrev-ref HEAD
```
Expected: `feature/color-role-modifiers`.

- [ ] **Step 3: Confirm no `claude/` branch name slipped in**

```bash
git rev-parse --abbrev-ref HEAD | grep -v '^claude/' && echo OK
```
Expected: prints `feature/color-role-modifiers` then `OK`.

All subsequent tasks operate inside the worktree at `.claude/worktrees/color-role-modifiers/`. Paths below are relative to that worktree.

---

## File structure

The change is concentrated in `theme/` (new file + edits to existing files), with thin migration edits in three view files and one component file. Two helpers (`renderTaskIDGradient` in `view/task_box.go` and `renderTikiIDGradient` in `view/taskdetail/render_helpers.go`) are byte-identical duplicates and get consolidated into one helper that uses `Paint`.

**New files:**
- `theme/paint.go` — `Paint`, `PositionPaint` interfaces; `solidPaint`, `gradientPaint`, `gradientAlgo` types; `paintFor`, `KnownModifierNames`, modifier algorithms.
- `theme/paint_test.go` — unit tests for solid/gradient paint behavior and `paintFor` lookup.

**Modified — theme package:**
- `theme/role.go` — DELETE `GradientRole` interface, `gradientRole` type, `newGradientRole` (lines 20-27, 58-100).
- `theme/theme.go` — DELETE `tikiIDGradient`, `captionFallbackGradient` fields and their getters (lines 33-34, 73-74). Update doc.
- `theme/binding.go` — DELETE all `tikiIDGradient: newGradientRole(...)` and `captionFallbackGradient: newGradientRole(...)` initialization blocks across 14 themes (~28 entries). ADD a new `tikiID` solid `Role` field initialized per theme from the palette color today's gradient is built from (DeepSkyBlue/Cyan/Frost1/etc.). DELETE the unused `gradientFromColor` helper (lines 9-27 already marked `//nolint:unused`).
- `theme/theme.go` — ADD `tikiID Role` field + `TikiID() Role` getter.
- `theme/resolve.go` — REPLACE `RoleResolver()` with `PaintResolver()` returning the modifier-aware signature. ADD a `"tiki.id"` case to `ResolveByName` so the new `tikiID` field is namable from `workflow.yaml`. ADD startup invariant: assert no canonical role name ends in a known modifier suffix.
- `theme/palettes/*.go` (14 files) — DELETE the `CaptionFallbackStart` / `CaptionFallbackEnd` fields and their initializer values. Remove unused-field assertions from corresponding `_test.go` files.
- `theme/snapshot_test.go` — UPDATE to drop gradient-row keys; ADD per-rune golden assertions for `gradientPaint.PaintString` on `tiki.id.accent` and `accent.action.lift`.
- `theme/binding_test.go` — DROP `TikiIDGradient is nil` assertion; ADD `TikiID()` non-nil assertion.
- `theme/internal_smoke_test.go` — DELETE the `gradientRole` exercise (lines 50-64); ADD an equivalent `gradientPaint` exercise.
- `theme/resolve_test.go` — REPLACE `TestRoleResolverClosure` with `TestPaintResolverClosure` covering `(role,"")`, `(role,"accent")`, `(role,"lift")`, and unknown-modifier failure.

**Modified — workflow package:**
- `workflow/visual.go` — CHANGE resolver signature in `ExpandVisual` from `func(role string) (tag string, ok bool)` to `func(role, modifier string) (Paint, bool)` (where `Paint` is `theme.Paint`). UPDATE `scanVisual` to split the token on the last dot when the suffix is a known modifier. CHANGE `ValidRoles` companion to also gate modifiers.
- `workflow/visual_test.go` — UPDATE existing tests to use a `Paint`-returning resolver helper; ADD new tests for `<role.accent>`, `<role.lift>`, unknown-modifier failure, last-dot disambiguation.

**Modified — view package:**
- `view/task_box.go` — DELETE local `renderTaskIDGradient`; CALL the new shared helper.
- `view/gradient_caption_row.go` — REPLACE `gradient theme.Gradient` field with `paint theme.PositionPaint`. DELETE `computeCaptionGradient` and the `useVibrantPluginGradient` / `vibrantBoost` constants. UPDATE `NewGradientCaptionRow` to accept the per-plugin base `theme.Role` instead of a `theme.Color`; resolve `.lift` paint internally.
- `view/tiki_plugin_view.go` and `view/doki_plugin_view.go` — UPDATE callers to pass a `theme.Role` instead of a `theme.Color` background.
- `view/taskdetail/render_helpers.go` — DELETE local `renderTikiIDGradient`; CALL the new shared helper. UPDATE `expandFieldText` and `RenderTitleText` to use `PaintResolver` instead of `RoleResolver`.

**New shared helper:**
- `view/render/tikiid.go` — `RenderTikiIDPaint(id string, roles *theme.Theme) string` consolidating the three duplicates. Internally resolves `tiki.id.accent` via `roles.PaintResolver()` and returns the painted string.

**Modified — component package:**
- `component/task_list.go` — REPLACE `IDGradient theme.Gradient` / `IDFallback theme.Color` in `TaskRowColors` with `IDPaint theme.Paint`. UPDATE `DefaultTaskRowColors` to call `roles.PaintResolver()("tiki.id", "accent")`. UPDATE `RenderTaskRow` to call `colors.IDPaint.PaintString(tk.ID)`. UPDATE `SetIDColors` → `SetIDPaint(p theme.Paint)`.
- `component/task_list_test.go` — UPDATE the `TestSetIDColors` test.

**Modified — documentation:**
- `CLAUDE.md` — ADD "Color roles & modifiers" section.
- `theme/doc.go` — DOCUMENT `Paint`, `PositionPaint`, modifier vocabulary.
- `workflow/visual.go` doc comments — DOCUMENT new grammar.

---

## Algorithm notes (read once before starting)

Two gradient algorithms map to the two modifiers. **Both reuse existing math** to keep output byte-identical to today's renderers:

- **`.accent`** → derives from the base color by **lightening** toward white by ratio `0.2`. Implementation = `util/gradient.GradientFromColor(base, 0.2, fallback)`. Per-rune emission = `util/gradient.RenderGradientText`. The "fallback" parameter is irrelevant in the new model — if base is `(0,0,0)`, return a solid `solidPaint` instead. Rationale: matches the public `GradientFromColor` semantics already in `util/gradient`, which is what `view/gradient_caption_row.go:165` already uses. (Note: today's `theme/binding.go` had a separate **darken**-flavored helper used by `tikiIDGradient`, but `util/gradient.GradientFromColor` is the load-bearing public one and produces the same visual feel for our base colors — the snapshot tests in Task 14 lock the exact bytes.)

- **`.lift`** → derives from the base color by **saturation boost** of `1.6`. Implementation = `util/gradient.GradientFromColorVibrant(base, 1.6, fallback)`. Same per-rune emission. Same `(0,0,0)` → solid fallback rule.

**Disabled-gradient degradation:** when `theme.UseGradients.Load()` is false, `gradientPaint.PaintString(s)` returns `[#rrggbb]s[-]` using `base.TCell().RGB()`, and `gradientPaint.ColorAt(t)` returns `base.TCell()` regardless of `t`. This matches the spec's "base role is its own fallback" rule.

**Last-dot split disambiguation:** when `scanVisual` sees `<x.y.z>`, it tries the suffix `z` against `KnownModifierNames()`; if `z` is a known modifier, base=`x.y`, modifier=`z`; otherwise base=`x.y.z`, modifier=`""`. A startup invariant asserts that no canonical role name in `KnownRoleNames()` ends with `.accent` or `.lift`.

---

## Task 1: Add `Paint` and `PositionPaint` interfaces with a solid implementation

**Files:**
- Create: `theme/paint.go`
- Create: `theme/paint_test.go`

- [ ] **Step 1: Write the failing tests**

Create `theme/paint_test.go`:

```go
package theme

import (
	"strings"
	"testing"
)

func TestSolidPaint_PaintString(t *testing.T) {
	role := newColorRole(NewColorHex("#ff0000"))
	p := solidPaint{role: role}
	got := p.PaintString("hello")
	want := role.Tag() + "hello[-]"
	if got != want {
		t.Errorf("solidPaint.PaintString = %q, want %q", got, want)
	}
}

func TestSolidPaint_PaintString_Empty(t *testing.T) {
	role := newColorRole(NewColorHex("#ff0000"))
	p := solidPaint{role: role}
	got := p.PaintString("")
	if got != "" {
		t.Errorf("solidPaint.PaintString(\"\") = %q, want empty", got)
	}
}

func TestSolidPaint_ColorAt(t *testing.T) {
	role := newColorRole(NewColorHex("#abcdef"))
	p := solidPaint{role: role}
	for _, ti := range []float64{0, 0.5, 1, -1, 2} {
		if got := p.ColorAt(ti); got != role.TCell() {
			t.Errorf("solidPaint.ColorAt(%v) = %v, want %v", ti, got, role.TCell())
		}
	}
}

func TestSolidPaint_SatisfiesBothInterfaces(t *testing.T) {
	role := newColorRole(NewColorHex("#123456"))
	var _ Paint = solidPaint{role: role}
	var _ PositionPaint = solidPaint{role: role}
}

// Sanity check that the per-rune emission of gradientPaint uses bracketed
// tview tags rather than ANSI escapes, even before paintFor exists.
func TestGradientPaint_PaintString_PerRuneTags(t *testing.T) {
	base := newColorRole(NewColorHex("#0080ff"))
	p := gradientPaint{base: base, algo: algoAccent}
	UseGradients.Store(true)
	defer UseGradients.Store(false)
	got := p.PaintString("ab")
	// Expect 2 `[#rrggbb]` openers (one per rune) and a single trailing `[-]`.
	openCount := strings.Count(got, "[#")
	if openCount != 2 {
		t.Errorf("gradientPaint.PaintString: got %d color tags, want 2; output=%q", openCount, got)
	}
	if !strings.HasSuffix(got, "[-]") {
		t.Errorf("gradientPaint.PaintString: missing trailing reset; output=%q", got)
	}
}

func TestGradientPaint_DisabledGradient_DegradesToSolid(t *testing.T) {
	base := newColorRole(NewColorHex("#0080ff"))
	p := gradientPaint{base: base, algo: algoAccent}
	UseGradients.Store(false)
	defer UseGradients.Store(false)
	got := p.PaintString("x")
	want := base.Tag() + "x[-]"
	if got != want {
		t.Errorf("gradientPaint.PaintString (gradients off) = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
go test ./theme/ -run 'Paint' -v
```
Expected: FAIL with `undefined: solidPaint`, `undefined: gradientPaint`, `undefined: Paint`, `undefined: PositionPaint`, `undefined: algoAccent`.

- [ ] **Step 3: Implement `theme/paint.go`**

Create `theme/paint.go`:

```go
package theme

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/util/gradient"
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

// gradientAlgo derives a Gradient from a base color. Each modifier
// corresponds to one gradientAlgo.
type gradientAlgo func(base tcell.Color) Gradient

// gradientPaint derives its gradient at render time from the base role's
// resolved color via algo. When theme.UseGradients is false, both methods
// degrade to solid behavior matching solidPaint{role: base}.
type gradientPaint struct {
	base Role
	algo gradientAlgo
}

func (p gradientPaint) PaintString(s string) string {
	if s == "" {
		return ""
	}
	if !UseGradients.Load() {
		return p.base.Tag() + s + "[-]"
	}
	g := p.algo(p.base.TCell())
	br, bg, bb := p.base.TCell().RGB()
	fallback := NewColorRGB(br, bg, bb)
	return gradient.RenderAdaptiveGradientText(s, g, fallback) + "[-]"
}

func (p gradientPaint) ColorAt(t float64) tcell.Color {
	if !UseGradients.Load() {
		return p.base.TCell()
	}
	g := p.algo(p.base.TCell())
	return gradient.InterpolateColor(g, t)
}

// algoAccent derives a gentle gradient by lightening the base by 20%.
// Matches util/gradient.GradientFromColor(base, 0.2, ...) semantics.
func algoAccent(base tcell.Color) Gradient {
	r, g, b := base.RGB()
	c := NewColorRGB(r, g, b)
	// Use util/gradient's helper directly so behavior tracks any future
	// fix to the lighten algorithm. Pass a zero-fallback — gradientPaint
	// already handles UseGradients=false and (0,0,0) bases.
	return gradient.GradientFromColor(c, 0.2, Gradient{Start: [3]int{int(r), int(g), int(b)}, End: [3]int{int(r), int(g), int(b)}})
}

// algoLift derives a vibrant gradient by boosting saturation 1.6x.
// Matches util/gradient.GradientFromColorVibrant(base, 1.6, ...) semantics.
func algoLift(base tcell.Color) Gradient {
	r, g, b := base.RGB()
	c := NewColorRGB(r, g, b)
	return gradient.GradientFromColorVibrant(c, 1.6, Gradient{Start: [3]int{int(r), int(g), int(b)}, End: [3]int{int(r), int(g), int(b)}})
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

// init asserts the disambiguation invariant: no canonical role name may
// end in a known modifier suffix. Violation would make the last-dot split
// in workflow.scanVisual ambiguous.
func init() {
	for _, role := range canonicalRoleNames {
		for _, mod := range KnownModifierNames() {
			if strings.HasSuffix(role, "."+mod) {
				panic(fmt.Sprintf("theme: canonical role %q ends in modifier %q — disambiguation invariant violated", role, mod))
			}
		}
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
go test ./theme/ -run 'Paint' -v
```
Expected: PASS for all `TestSolidPaint_*`, `TestGradientPaint_*`.

- [ ] **Step 5: Run linter**

```bash
golangci-lint run ./theme/...
```
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add theme/paint.go theme/paint_test.go
git commit -m "theme: add Paint and PositionPaint interfaces with solid/gradient impls"
```

---

## Task 2: Add `tikiID` role and `TikiID()` getter to Theme

**Files:**
- Modify: `theme/theme.go`
- Modify: `theme/binding.go`
- Modify: `theme/resolve.go`
- Modify: `theme/binding_test.go`

- [ ] **Step 1: Write the failing test**

Append to `theme/binding_test.go`:

```go
func TestTikiIDRolePresent(t *testing.T) {
	for _, name := range []string{"dark", "light", "dracula", "tokyo-night", "gruvbox-dark", "catppuccin-mocha", "solarized-dark", "nord", "monokai", "one-dark", "catppuccin-latte", "solarized-light", "gruvbox-light", "github-light"} {
		th := LoadByName(name)
		if th.TikiID() == nil {
			t.Errorf("%s: TikiID() is nil", name)
		}
	}
}

func TestResolveByName_TikiID(t *testing.T) {
	th := LoadByName("dark")
	r, ok := th.ResolveByName("tiki.id")
	if !ok || r == nil {
		t.Fatalf("ResolveByName(\"tiki.id\") = (%v, %v), want non-nil + true", r, ok)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./theme/ -run 'TikiID' -v
```
Expected: FAIL with `th.TikiID undefined`.

- [ ] **Step 3: Add the `tikiID Role` field and getter**

In `theme/theme.go`, locate the `Theme` struct (line 15). Add `tikiID Role` to the field list — group with `// Tiki-specific`:

```go
// Tiki-specific
tikiID Role
```

In the getters section (around line 41), add:

```go
func (t *Theme) TikiID() Role { return t.tikiID }
```

In `theme/resolve.go`, in the `ResolveByName` switch (after the `accent.*` cases, around line 45), add:

```go
case "tiki.id":
	return t.tikiID, true
```

In the `canonicalRoleNames` slice (line 124), add `"tiki.id"` to the appropriate group:

```go
"highlight", "accent.action", "accent.tag", "tiki.id",
```

- [ ] **Step 4: Initialize `tikiID` in every theme binding**

In `theme/binding.go`, for each `bind*()` function, add a `tikiID:` line using the same palette color the function currently passes to `gradientFromColor`/`roleOf` for `tikiIDGradient`. The mapping by theme is:

| Theme | Palette field |
|-------|---------------|
| `bindDark` | `p.DeepSkyBlue` |
| `bindLight` | `p.DeepSkyBlue` |
| `bindDracula` | `p.Cyan` |
| `bindNord` | `p.Frost1` |
| `bindGruvboxDark` | `p.NeutralBlue` |
| `bindGruvboxLight` | `p.NeutralBlue` |
| `bindTokyoNight` | `p.Cyan` |
| `bindMonokai` | `p.Cyan` |
| `bindOneDark` | `p.Blue` |
| `bindCatppuccinMocha` | `p.Sky` |
| `bindCatppuccinLatte` | `p.Sky` |
| `bindSolarizedDark` | `p.Blue` |
| `bindSolarizedLight` | `p.Blue` |
| `bindGithubLight` | `p.BlueAccent` |

In each binding, add a line like (using `bindDark` as an example):

```go
tikiID: roleOf(p.DeepSkyBlue),
```

Place this line adjacent to the existing `tikiIDGradient:` initialization. Both will coexist temporarily; the old field is deleted in Task 5.

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./theme/ -run 'TikiID' -v
```
Expected: PASS.

- [ ] **Step 6: Confirm all existing theme tests still pass**

```bash
go test ./theme/...
```
Expected: PASS.

- [ ] **Step 7: Run linter**

```bash
golangci-lint run ./theme/...
```
Expected: 0 issues.

- [ ] **Step 8: Commit**

```bash
git add theme/theme.go theme/binding.go theme/resolve.go theme/binding_test.go
git commit -m "theme: add tikiID solid role and tiki.id resolver entry"
```

---

## Task 3: Add `PaintResolver` method to Theme

**Files:**
- Modify: `theme/resolve.go`
- Modify: `theme/resolve_test.go`

- [ ] **Step 1: Write the failing test**

Add to `theme/resolve_test.go`:

```go
func TestPaintResolver_SolidRole(t *testing.T) {
	th := LoadByName("dark")
	resolver := th.PaintResolver()
	p, ok := resolver("status.danger", "")
	if !ok || p == nil {
		t.Fatalf("PaintResolver(status.danger, \"\") = (%v, %v); want non-nil + true", p, ok)
	}
	want := th.StatusDanger().Tag() + "x[-]"
	if got := p.PaintString("x"); got != want {
		t.Errorf("PaintString(\"x\") = %q, want %q", got, want)
	}
}

func TestPaintResolver_GradientModifier(t *testing.T) {
	UseGradients.Store(true)
	defer UseGradients.Store(false)
	th := LoadByName("dark")
	resolver := th.PaintResolver()
	for _, modifier := range []string{"accent", "lift"} {
		p, ok := resolver("tiki.id", modifier)
		if !ok || p == nil {
			t.Errorf("PaintResolver(tiki.id, %q) = (%v, %v); want non-nil + true", modifier, p, ok)
			continue
		}
		// At least 2 color tags for a 2-rune string under gradient mode.
		got := p.PaintString("AB")
		if !strings.Contains(got, "[#") {
			t.Errorf("modifier %q: expected per-rune color tags in %q", modifier, got)
		}
	}
}

func TestPaintResolver_UnknownRole(t *testing.T) {
	th := LoadByName("dark")
	resolver := th.PaintResolver()
	if _, ok := resolver("nosuchrole", ""); ok {
		t.Error("PaintResolver returned ok=true for unknown role")
	}
}

func TestPaintResolver_UnknownModifier(t *testing.T) {
	th := LoadByName("dark")
	resolver := th.PaintResolver()
	if _, ok := resolver("status.danger", "nosuchmod"); ok {
		t.Error("PaintResolver returned ok=true for unknown modifier")
	}
}
```

Note: this test imports `strings`; the existing `resolve_test.go` may already import it. If not, add the import.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./theme/ -run 'PaintResolver' -v
```
Expected: FAIL with `th.PaintResolver undefined`.

- [ ] **Step 3: Add `PaintResolver` to `theme/resolve.go`**

Add after the existing `RoleResolver` method (which will be deleted in Task 6):

```go
// PaintResolver returns a closure that maps (role, modifier) to a Paint.
// Used by workflow.ExpandVisual at render time. modifier is the optional
// suffix following the last dot in `<role.modifier>` markup; pass "" for
// solid (no modifier). Reports ok=false for unknown role or modifier names.
func (t *Theme) PaintResolver() func(role, modifier string) (Paint, bool) {
	return func(role, modifier string) (Paint, bool) {
		base, ok := t.ResolveByName(role)
		if !ok {
			return nil, false
		}
		return paintFor(base, modifier)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./theme/ -run 'PaintResolver' -v
```
Expected: PASS.

- [ ] **Step 5: Run linter**

```bash
golangci-lint run ./theme/...
```
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add theme/resolve.go theme/resolve_test.go
git commit -m "theme: add PaintResolver returning Paint for (role, modifier)"
```

---

## Task 4: Teach `workflow.ExpandVisual` the modifier grammar

**Files:**
- Modify: `workflow/visual.go`
- Modify: `workflow/visual_test.go`

This task changes the resolver signature. It is the riskiest single edit because many tests and callers consume the old `func(role string) (tag string, ok bool)`. We migrate everything in one task to avoid leaving the API in a half-broken state.

- [ ] **Step 1: Write the failing tests**

Replace the body of `workflow/visual_test.go` with the following. The new helper accepts the new resolver shape; tests cover bare roles, modifiers, the last-dot disambiguation rule, and validation.

```go
package workflow

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/theme"
)

// solidPaintFromTag is a test-only Paint that wraps text in a fake tag.
// We use a function-typed Paint so tests do not depend on theme internals.
type fakePaint struct{ tag string }

func (p fakePaint) PaintString(s string) string {
	if s == "" {
		return ""
	}
	return p.tag + s + "[-]"
}

// gradientFakePaint emits one tag per rune so tests can distinguish
// solid vs gradient output without exercising real gradient math.
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

// paintResolver mimics the shape of theme.Theme.PaintResolver but uses fakes.
// Accepts only roles in workflow.ValidRoles plus the modifier set
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./workflow/ -run 'Visual\|Expand' -v
```
Expected: FAIL — many "undefined" errors and the existing `ExpandVisual`/`ValidateVisualMarkup` signatures don't match.

- [ ] **Step 3: Replace `workflow/visual.go` with the modifier-aware implementation**

Replace the entire file content with:

```go
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

// validModifiers mirrors theme.KnownModifierNames in map form.
var validModifiers = func() map[string]struct{} {
	m := make(map[string]struct{}, len(theme.KnownModifierNames()))
	for _, name := range theme.KnownModifierNames() {
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
	_, err := scanVisual(markup, func(role, modifier string) (theme.Paint, bool) {
		if _, ok := ValidRoles[role]; !ok {
			return nil, false
		}
		if modifier != "" {
			if _, ok := validModifiers[modifier]; !ok {
				return nil, false
			}
		}
		return nil, true
	})
	return err
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
			if pendingRunStart < 0 {
				pendingRunStart = i
			}
			// Replace the '<<' pair with a single '<' by flushing up to here,
			// emitting a literal '<', and continuing past the pair.
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
		role, modifier := splitRoleModifier(token)
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

// splitRoleModifier applies the last-dot disambiguation rule. If the last
// dot-separated segment is a known modifier, returns (prefix, suffix).
// Otherwise returns (token, "").
func splitRoleModifier(token string) (role, modifier string) {
	dot := strings.LastIndexByte(token, '.')
	if dot < 0 {
		return token, ""
	}
	suffix := token[dot+1:]
	if _, ok := validModifiers[suffix]; ok {
		return token[:dot], suffix
	}
	return token, ""
}
```

- [ ] **Step 4: Run workflow tests to verify they pass**

```bash
go test ./workflow/ -run 'Visual\|Expand' -v
```
Expected: PASS for all new and renamed tests.

- [ ] **Step 5: Run linter (workflow + theme dependency)**

```bash
golangci-lint run ./workflow/... ./theme/...
```
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add workflow/visual.go workflow/visual_test.go
git commit -m "workflow: teach ExpandVisual the .accent/.lift modifier grammar"
```

---

## Task 5: Migrate the shared tiki-ID renderer to use Paint

**Files:**
- Create: `view/render/tikiid.go`
- Create: `view/render/tikiid_test.go`

- [ ] **Step 1: Write the failing test**

Create `view/render/tikiid_test.go`:

```go
package render

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/theme"
)

func TestRenderTikiIDPaint_SolidWhenGradientsDisabled(t *testing.T) {
	theme.SetTheme(theme.LoadByName("dark"))
	theme.UseGradients.Store(false)
	defer theme.UseGradients.Store(false)
	got := RenderTikiIDPaint("K3X9M2", theme.Roles())
	if !strings.HasPrefix(got, "[#") || !strings.HasSuffix(got, "[-]") {
		t.Errorf("expected `[#...]...[-]` shape, got %q", got)
	}
	// solid mode emits exactly one color tag
	if strings.Count(got, "[#") != 1 {
		t.Errorf("expected exactly 1 color tag in solid mode, got %d in %q", strings.Count(got, "[#"), got)
	}
}

func TestRenderTikiIDPaint_GradientWhenEnabled(t *testing.T) {
	theme.SetTheme(theme.LoadByName("dark"))
	theme.UseGradients.Store(true)
	defer theme.UseGradients.Store(false)
	got := RenderTikiIDPaint("AB", theme.Roles())
	// gradient mode emits one tag per rune (RenderGradientText behavior)
	if strings.Count(got, "[#") < 2 {
		t.Errorf("expected per-rune color tags in gradient mode, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./view/render/...
```
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement `view/render/tikiid.go`**

```go
// Package render holds small string-building helpers shared between
// view subpackages, kept here to avoid duplication.
package render

import "github.com/boolean-maybe/tiki/theme"

// RenderTikiIDPaint renders a tiki ID using the theme's tiki.id role with
// the .accent modifier — a subtle gradient on capable terminals, a solid
// color on lesser ones. Returns empty when id is empty. Falls back to a
// plain echo if the resolver fails (defensive; should never happen since
// tiki.id is a canonical role).
func RenderTikiIDPaint(id string, roles *theme.Theme) string {
	if id == "" {
		return ""
	}
	resolver := roles.PaintResolver()
	paint, ok := resolver("tiki.id", "accent")
	if !ok {
		return id
	}
	return paint.PaintString(id)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./view/render/...
```
Expected: PASS.

- [ ] **Step 5: Run linter**

```bash
golangci-lint run ./view/render/...
```
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add view/render/tikiid.go view/render/tikiid_test.go
git commit -m "view/render: add RenderTikiIDPaint consolidating tiki-id gradient renderers"
```

---

## Task 6: Migrate `view/task_box.go`, `view/taskdetail/render_helpers.go`, and `component/task_list.go` to the new helper

**Files:**
- Modify: `view/task_box.go`
- Modify: `view/taskdetail/render_helpers.go`
- Modify: `view/taskdetail/task_edit_view.go`
- Modify: `view/taskdetail/configurable_detail_view.go`
- Modify: `component/task_list.go`
- Modify: `component/task_list_test.go`

This task migrates the three byte-identical `renderTikiIDGradient` / `renderTaskIDGradient` callers and the `TaskList` ID-gradient plumbing to use the shared `Paint`-based helper.

- [ ] **Step 1: Update `view/task_box.go`**

In `view/task_box.go`, locate `renderTaskIDGradient` (around line 151). Replace the function with a call to the shared helper. Top of file: ensure the import `"github.com/boolean-maybe/tiki/view/render"` is present and drop the unused `"github.com/boolean-maybe/tiki/util/gradient"` import if no other usage remains in the file (run `goimports` after).

Replace the function body:

```go
func renderTaskIDGradient(id string, roles *theme.Theme) string {
	return render.RenderTikiIDPaint(id, roles)
}
```

Or — if `renderTaskIDGradient` has only one caller — inline the call. Use `grep -n "renderTaskIDGradient" view/task_box.go` to decide.

- [ ] **Step 2: Update `view/taskdetail/render_helpers.go`**

Replace `renderTikiIDGradient` (lines 20-33) with:

```go
func renderTikiIDGradient(id string, roles *theme.Theme) string {
	return render.RenderTikiIDPaint(id, roles)
}
```

Add `"github.com/boolean-maybe/tiki/view/render"` to the import block. Remove unused `"github.com/boolean-maybe/tiki/util/gradient"` import if no other code in the file uses it.

Also in the same file: `expandFieldText` (line 49) calls `roles.RoleResolver()`. Update it to use a `Paint`-resolver adapter:

```go
func expandFieldText(raw string, roles *theme.Theme) string {
	escaped := tview.Escape(raw)
	if roles == nil {
		return escaped
	}
	expanded, err := workflow.ExpandVisual(escaped, roles.PaintResolver())
	if err != nil {
		return escaped
	}
	return expanded
}
```

And in `RenderTitleText` (line 142-144), update the role-tag lookup to use `PaintResolver`:

```go
if role != "" && ctx.Roles != nil {
	resolver := ctx.Roles.PaintResolver()
	if paint, ok := resolver(role, ""); ok {
		// Apply the solid paint as a tag-prefix style. PaintString wraps the
		// whole title; here we only want the opening tag, so resolve the
		// underlying Role directly:
		if r, rok := ctx.Roles.ResolveByName(role); rok {
			titleTag = r.Tag()
		} else {
			titleTag = ctx.Roles.TextPrimary().BoldTag()
		}
		_ = paint // keep symmetric with new resolver wiring
	} else {
		titleTag = ctx.Roles.TextPrimary().BoldTag()
	}
}
```

Note: `RenderTitleText` previously emitted a bare opening tag rather than wrapping. The new code preserves that exact contract by falling back to `ResolveByName` for the tag. The `_ = paint` line documents intent and will be removed in cleanup; do not optimize it away yet — the explicit form makes the test diff in Step 4 easier to read.

- [ ] **Step 3: Update `component/task_list.go`**

Replace the `TaskRowColors` struct (lines 17-25) with:

```go
type TaskRowColors struct {
	IDPaint            theme.Paint
	TitleColor         theme.Color
	SelectionFg        theme.Color
	SelectionBg        theme.Color
	StatusDoneColor    theme.Color
	StatusPendingColor theme.Color
}
```

Replace `DefaultTaskRowColors` (lines 28-43):

```go
func DefaultTaskRowColors() TaskRowColors {
	roles := theme.Roles()
	idPaint, _ := roles.PaintResolver()("tiki.id", "accent")
	return TaskRowColors{
		IDPaint:            idPaint,
		TitleColor:         theme.NewColor(roles.TextSecondary().TCell()),
		SelectionFg:        theme.NewColor(roles.TextPrimary().TCell()),
		SelectionBg:        theme.NewColor(roles.SurfaceSelection().TCell()),
		StatusDoneColor:    theme.NewColor(roles.TextLabel().TCell()),
		StatusPendingColor: theme.NewColor(roles.TextPrimary().TCell()),
	}
}
```

Replace `RenderTaskRow` (line 49) — change the `idText` line:

```go
idText := colors.IDPaint.PaintString(tk.ID)
```

In the `TaskList` struct (lines 86-100), replace the `idGradient` and `idFallback` fields with:

```go
idPaint theme.Paint
```

In `NewTaskList` (line 102), update the field assignment:

```go
return &TaskList{
	Box:                tview.NewBox(),
	maxVisibleRows:     maxVisibleRows,
	idPaint:            colors.IDPaint,
	titleColor:         colors.TitleColor,
	selectionColor:     colors.SelectionFg,
	selectionBgColor:   colors.SelectionBg,
	statusDoneColor:    colors.StatusDoneColor,
	statusPendingColor: colors.StatusPendingColor,
}
```

Replace `SetIDColors` (line 165) with:

```go
// SetIDPaint overrides the Paint for the ID column.
func (tl *TaskList) SetIDPaint(p theme.Paint) *TaskList {
	tl.idPaint = p
	return tl
}
```

Update `TaskList.buildRow` (lines 202-212) to use the new `IDPaint` field:

```go
func (tl *TaskList) buildRow(tk *tikipkg.Tiki, selected bool, width int) string {
	return RenderTaskRow(tk, selected, width, tl.idColumnWidth, TaskRowColors{
		IDPaint:            tl.idPaint,
		TitleColor:         tl.titleColor,
		SelectionFg:        tl.selectionColor,
		SelectionBg:        tl.selectionBgColor,
		StatusDoneColor:    tl.statusDoneColor,
		StatusPendingColor: tl.statusPendingColor,
	})
}
```

Remove the now-unused `"github.com/boolean-maybe/tiki/util/gradient"` import.

- [ ] **Step 4: Update `component/task_list_test.go`**

Replace `TestSetIDColors` (line 180):

```go
func TestSetIDPaint(t *testing.T) {
	tl := NewTaskList(5)
	role := theme.Roles().TextValue()
	_ = role // satisfy import while keeping the test independent of internal solidPaint
	// Use the resolver to obtain a known-good Paint.
	paint, ok := theme.Roles().PaintResolver()("text.value", "")
	if !ok {
		t.Fatalf("PaintResolver(text.value) returned ok=false")
	}
	result := tl.SetIDPaint(paint)
	if result != tl {
		t.Error("SetIDPaint should return self for chaining")
	}
}
```

Adjust the test file's `init` or `TestMain` to call `theme.SetTheme(theme.LoadByName("dark"))` before tests run, if not already present.

- [ ] **Step 5: Run all tests in affected packages**

```bash
go test ./component/... ./view/... ./view/taskdetail/... ./view/render/...
```
Expected: PASS.

- [ ] **Step 6: Run linter**

```bash
golangci-lint run ./component/... ./view/...
```
Expected: 0 issues.

- [ ] **Step 7: Commit**

```bash
git add view/task_box.go view/taskdetail/render_helpers.go component/task_list.go component/task_list_test.go
git commit -m "migrate tiki-id gradient consumers to Paint"
```

---

## Task 7: Migrate `GradientCaptionRow` to use PositionPaint

**Files:**
- Modify: `view/gradient_caption_row.go`
- Modify: `view/tiki_plugin_view.go`
- Modify: `view/doki_plugin_view.go`

This is the screen-cell painter migration. The widget loses its `theme.Gradient` field and `computeCaptionGradient` helper; callers pass a `theme.Role` whose `.lift` paint produces the per-column color.

- [ ] **Step 1: Update `view/gradient_caption_row.go`**

Replace the file's struct, constructor, draw loop, and helper functions:

```go
package view

import (
	"github.com/boolean-maybe/tiki/theme"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// GradientCaptionRow renders multiple lane captions with a continuous
// horizontal background gradient. The gradient is sourced from a
// PositionPaint (typically theme.Roles().PaintResolver()(role, "lift"))
// so the widget itself is unaware of gradient vs. solid behavior — that
// decision lives entirely in the Paint implementation.
type GradientCaptionRow struct {
	*tview.Box
	laneNames  []string
	laneWidths []int
	paint      theme.PositionPaint
	textColor  theme.Color
}

// NewGradientCaptionRow creates a new gradient caption row. The widget
// derives its per-column background color from `bgRole` with the `.lift`
// modifier: a vibrant gradient on capable terminals, a solid color when
// gradients are disabled. The widget itself is unaware of which case
// applies — that decision lives inside the PositionPaint implementation.
func NewGradientCaptionRow(laneNames []string, laneWidths []int, bgRole theme.Role, textColor theme.Color) *GradientCaptionRow {
	pp := theme.PaintForRolePosition(bgRole, "lift")
	if pp == nil {
		// Defensive: an unknown modifier from a refactor mistake should not
		// crash the UI. Fall back to a flat solid paint with the same role.
		pp = theme.PaintForRolePosition(bgRole, "")
	}
	return &GradientCaptionRow{
		Box:        tview.NewBox(),
		laneNames:  laneNames,
		laneWidths: laneWidths,
		paint:      pp,
		textColor:  textColor,
	}
}

// Draw renders all lane captions with a screen-wide gradient background.
func (gcr *GradientCaptionRow) Draw(screen tcell.Screen) {
	gcr.DrawForSubclass(screen, gcr)

	x, y, width, height := gcr.GetInnerRect()
	if width <= 0 || height <= 0 || len(gcr.laneNames) == 0 {
		return
	}

	numLanes := len(gcr.laneNames)
	laneStarts, laneEnds := computeLaneBoundaries(gcr.laneWidths, numLanes, width)

	laneRunes := make([][]rune, numLanes)
	for i, name := range gcr.laneNames {
		laneRunes[i] = []rune(name)
	}

	laneIndex := 0
	for col := 0; col < width; col++ {
		for laneIndex < numLanes-1 && col >= laneEnds[laneIndex] {
			laneIndex++
		}

		// Distance-from-center → t in [0,1]. Edges are t=1, center is t=0.
		var t float64
		if width > 1 {
			centerPos := float64(width) / 2.0
			d := (float64(col) - centerPos) / centerPos
			if d < 0 {
				d = -d
			}
			t = d
		}

		bgColor := gcr.paint.ColorAt(t)

		currentLaneWidth := laneEnds[laneIndex] - laneStarts[laneIndex]
		posInLane := col - laneStarts[laneIndex]

		textRunes := laneRunes[laneIndex]
		textWidth := len(textRunes)
		textStartPos := 0
		if textWidth < currentLaneWidth {
			textStartPos = (currentLaneWidth - textWidth) / 2
		}

		char := ' '
		textIndex := posInLane - textStartPos
		if textIndex >= 0 && textIndex < textWidth {
			char = textRunes[textIndex]
		}

		style := tcell.StyleDefault.Foreground(gcr.textColor.TCell()).Background(bgColor)
		for row := 0; row < height; row++ {
			screen.SetContent(x+col, y+row, char, nil, style)
		}
	}
}

// computeLaneBoundaries — UNCHANGED, copy from the existing file as-is.
func computeLaneBoundaries(weights []int, numLanes, totalWidth int) (starts []int, ends []int) {
	starts = make([]int, numLanes)
	ends = make([]int, numLanes)

	totalWeight := 0
	for i := 0; i < numLanes; i++ {
		if i < len(weights) && weights[i] > 0 {
			totalWeight += weights[i]
		} else {
			totalWeight++
		}
	}

	pos := 0
	for i := 0; i < numLanes; i++ {
		w := 1
		if i < len(weights) && weights[i] > 0 {
			w = weights[i]
		}
		starts[i] = pos
		if i == numLanes-1 {
			ends[i] = totalWidth
		} else {
			ends[i] = pos + (totalWidth*w)/totalWeight
		}
		pos = ends[i]
	}
	return starts, ends
}
```

- [ ] **Step 2: Add `PaintForRolePosition` to `theme/paint.go`**

The above file references `theme.PaintForRolePosition`. Export it. Add to `theme/paint.go`:

```go
// PaintForRolePosition returns a PositionPaint for base+modifier. Mirrors
// PaintResolver semantics for screen-cell consumers. Returns nil for
// unknown modifier or nil base.
func PaintForRolePosition(base Role, modifier string) PositionPaint {
	if base == nil {
		return nil
	}
	p, ok := paintForPosition(base, modifier)
	if !ok {
		return nil
	}
	return p
}
```

- [ ] **Step 3: Update callers**

In `view/tiki_plugin_view.go` line 79, the current call is:

```go
pv.titleBar = NewGradientCaptionRow(laneNames, laneWidths, bgColor, textColor)
```

`bgColor` is a `theme.Color`. Convert the caller to pass a `theme.Role`. The simplest path: wrap the color in a `solidPaint`-style Role via a new helper, OR (better) pass `theme.Roles().TikiID()` or the equivalent plugin-specific role. Plugin captions are colored from `theme.Roles().PluginCaptions()` — that's a `PairListRole`. Inspect the existing call site to identify what `bgColor` corresponds to in the role system. **If `bgColor` cannot be cleanly mapped to a named role, fall back to a `theme.NewColorRoleAdapter(bgColor) Role` helper added to `theme/role.go`** that constructs an unexported `colorRole` from a `theme.Color`.

Add to `theme/role.go`:

```go
// NewColorRoleAdapter wraps a theme.Color as a Role for use with the Paint
// system. Used by view code that holds a Color from older APIs and needs to
// hand it to a PositionPaint resolver.
func NewColorRoleAdapter(c Color) Role { return colorRole{c: c} }
```

Then update both `view/tiki_plugin_view.go:79` and `view/doki_plugin_view.go:139`:

```go
pv.titleBar = NewGradientCaptionRow(laneNames, laneWidths, theme.NewColorRoleAdapter(bgColor), textColor)
```

```go
dv.titleBar = NewGradientCaptionRow([]string{dv.pluginDef.Name}, nil, theme.NewColorRoleAdapter(bgColor), textColor)
```

- [ ] **Step 4: Run all view tests**

```bash
go test ./view/... ./theme/...
```
Expected: PASS.

- [ ] **Step 5: Run linter**

```bash
golangci-lint run ./view/... ./theme/...
```
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add theme/paint.go theme/role.go view/gradient_caption_row.go view/tiki_plugin_view.go view/doki_plugin_view.go
git commit -m "view: migrate GradientCaptionRow to PositionPaint"
```

---

## Task 8: Delete the old `GradientRole` interface and theme fields

**Files:**
- Modify: `theme/role.go`
- Modify: `theme/theme.go`
- Modify: `theme/binding.go`
- Modify: `theme/internal_smoke_test.go`
- Modify: `theme/snapshot_test.go`
- Modify: `theme/binding_test.go`
- Modify: `theme/doc.go`

- [ ] **Step 1: Verify zero references remain to the old API**

```bash
grep -rn "GradientRole\b\|tikiIDGradient\b\|captionFallbackGradient\b\|TikiIDGradient\b\|CaptionFallbackGradient\b\|newGradientRole\b\|gradientRole\b\|FallbackRole\b\|InterpolateTag\b\|InterpolateTCell\b" --include="*.go" .
```
Expected: matches only in `theme/role.go`, `theme/theme.go`, `theme/binding.go`, `theme/internal_smoke_test.go`, `theme/snapshot_test.go`, `theme/binding_test.go`, `theme/doc.go` (the files we are about to edit). **If any other file matches, return to the appropriate migration task and finish it before continuing.**

- [ ] **Step 2: Delete `GradientRole`, `gradientRole`, `newGradientRole` from `theme/role.go`**

Delete lines 20-27 (interface) and 58-100 (concrete type, methods, constructor). The `darkenRGB` helper (lines 9-17) is also unused — delete it. The `gradientFromColor` helper at lines 9-27 of `theme/binding.go` (NOT `theme/role.go`) is deleted in step 4.

- [ ] **Step 3: Delete fields and getters in `theme/theme.go`**

Delete the line `tikiIDGradient, captionFallbackGradient GradientRole` (around line 34) and the lines:

```go
func (t *Theme) TikiIDGradient() GradientRole          { return t.tikiIDGradient }
func (t *Theme) CaptionFallbackGradient() GradientRole { return t.captionFallbackGradient }
```

(around lines 73-74).

- [ ] **Step 4: Delete all `tikiIDGradient: newGradientRole(...)` and `captionFallbackGradient: newGradientRole(...)` blocks in `theme/binding.go`**

For each of the 14 `bind*` functions, delete the two multi-line initialization blocks. Also delete the now-unused `gradientFromColor` helper at the top of the file (lines 9-27).

The `Gradient` struct itself (in `theme/gradient.go`) stays — `util/gradient` still uses it.

- [ ] **Step 5: Update `theme/internal_smoke_test.go`**

Delete the `gradientRole` exercise block (lines 50-64). Replace with an equivalent `gradientPaint` exercise:

```go
// exercise gradientPaint via Paint and PositionPaint interfaces.
UseGradients.Store(true)
defer UseGradients.Store(false)
gp := gradientPaint{base: fg, algo: algoAccent}
if gp.PaintString("ab") == "" {
	t.Errorf("gradientPaint.PaintString empty")
}
_ = gp.ColorAt(-0.5) // exercise clamp low (gradient pkg clamps internally)
_ = gp.ColorAt(1.5)  // exercise clamp high
```

- [ ] **Step 6: Update `theme/snapshot_test.go`**

Delete the `CaptionFallbackGradient.start`, `CaptionFallbackGradient.end`, and any other gradient-key entries from `roleKeyMapping` (lines 119-120 and similar). Delete the `case "CaptionFallbackGradient":` and `case "TikiIDGradient":` blocks (around lines 182-188, 316-321, 337). Add new golden entries for gradient `Paint`:

```go
// new entries — per-rune golden bytes for gradient paint
"gradient_paint.tiki_id.accent.AB": "<computed at test time, see helper below>",
"gradient_paint.accent_action.lift.X": "<computed at test time, see helper below>",
```

For the helper, add (or extend) a test that does:

```go
func TestPaintSnapshot_GradientPaintGolden(t *testing.T) {
	UseGradients.Store(true)
	defer UseGradients.Store(false)
	th := LoadByName("dark")
	p, ok := th.PaintResolver()("tiki.id", "accent")
	if !ok {
		t.Fatal("resolver returned ok=false")
	}
	got := p.PaintString("AB")
	// Lock the exact byte sequence so future refactors can't silently drift output.
	want := goldenAccentAB // package-level const computed once and checked in
	if got != want {
		t.Errorf("gradient paint output drift:\n got=%q\nwant=%q", got, want)
	}
}
```

Compute the actual `want` string by running the test once with `want = got` substituted, observing the output, and pasting it into a package-level const. Repeat for `.lift` on a different role. (This is the "golden file" pattern — the first run captures truth, subsequent runs lock it.)

- [ ] **Step 7: Update `theme/binding_test.go`**

Delete the `TestAllThemesBoundGradients` (or similar) assertion that checks `TikiIDGradient() == nil`. The `TestTikiIDRolePresent` test from Task 2 supersedes it.

- [ ] **Step 8: Update `theme/doc.go`**

Delete the documentation references to `TikiIDGradient`, `CaptionFallbackGradient`, and `GradientRole` (lines 62-63 area). Add a new section documenting `Paint`, `PositionPaint`, `KnownModifierNames`, and the `.accent`/`.lift` vocabulary.

- [ ] **Step 9: Run all tests**

```bash
go test ./...
```
Expected: PASS for the entire test suite.

- [ ] **Step 10: Run linter**

```bash
golangci-lint run ./...
```
Expected: 0 issues.

- [ ] **Step 11: Commit**

```bash
git add theme/role.go theme/theme.go theme/binding.go theme/internal_smoke_test.go theme/snapshot_test.go theme/binding_test.go theme/doc.go
git commit -m "theme: delete GradientRole, tikiIDGradient, captionFallbackGradient"
```

---

## Task 9: Delete `CaptionFallbackStart` / `CaptionFallbackEnd` from every palette

**Files:**
- Modify: `theme/palettes/dark.go`, `light.go`, `dracula.go`, `tokyo_night.go`, `gruvbox_dark.go`, `gruvbox_light.go`, `catppuccin_mocha.go`, `catppuccin_latte.go`, `solarized_dark.go`, `solarized_light.go`, `nord.go`, `monokai.go`, `one_dark.go`, `github_light.go`
- Modify: corresponding `*_test.go` files in `theme/palettes/`

- [ ] **Step 1: Verify nothing outside `theme/palettes/` references these fields**

```bash
grep -rn "CaptionFallbackStart\|CaptionFallbackEnd" --include="*.go" .
```
Expected: matches only in `theme/palettes/*.go` and `theme/palettes/*_test.go`. The `theme/binding.go` lines that previously referenced them were deleted in Task 8.

- [ ] **Step 2: Delete the fields from each palette struct and the corresponding initializer values**

For each palette file (e.g. `theme/palettes/dark.go`):

```go
// Remove these field declarations:
CaptionFallbackStart [3]int
CaptionFallbackEnd   [3]int

// Remove the corresponding initializer entries:
CaptionFallbackStart: [3]int{...},
CaptionFallbackEnd:   [3]int{...},
```

Repeat for all 14 palettes.

- [ ] **Step 3: Update the palette unit tests**

Each `theme/palettes/*_test.go` has lines like:

```go
_ = d.CaptionFallbackStart
_ = d.CaptionFallbackEnd
```

These were field-existence assertions. Delete those two lines from each `_test.go` file.

- [ ] **Step 4: Run palette tests**

```bash
go test ./theme/palettes/...
```
Expected: PASS.

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```
Expected: PASS.

- [ ] **Step 6: Run linter**

```bash
golangci-lint run ./...
```
Expected: 0 issues.

- [ ] **Step 7: Commit**

```bash
git add theme/palettes/
git commit -m "theme/palettes: delete CaptionFallbackStart/End fields"
```

---

## Task 10: Delete the old `RoleResolver` method

**Files:**
- Modify: `theme/resolve.go`
- Modify: `theme/resolve_test.go`

- [ ] **Step 1: Verify no callers remain**

```bash
grep -rn "\.RoleResolver()\b\|RoleResolver func\b" --include="*.go" .
```
Expected: matches only in `theme/resolve.go` and `theme/resolve_test.go` (the files we are about to edit). If any other file matches, return to the relevant migration task — Task 6 should have caught all of them.

- [ ] **Step 2: Delete `Theme.RoleResolver()`**

In `theme/resolve.go`, delete the method (lines 108-119):

```go
func (t *Theme) RoleResolver() func(role string) (string, bool) {
	...
}
```

- [ ] **Step 3: Delete or rename `TestRoleResolverClosure`**

In `theme/resolve_test.go`, delete `TestRoleResolverClosure` (around line 103). The `TestPaintResolver_*` tests from Task 3 cover the same surface.

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```
Expected: PASS.

- [ ] **Step 5: Run linter**

```bash
golangci-lint run ./...
```
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add theme/resolve.go theme/resolve_test.go
git commit -m "theme: delete deprecated RoleResolver method"
```

---

## Task 11: Teach the gridlayout segment validator about modifiers

**Files:**
- Modify: `gridlayout/spec.go` (or whichever file declares the `Segment` struct used at `plugin/parser.go:370`)
- Modify: `gridlayout/parser.go` (the role parser that populates `Segment.Role`)
- Modify: `plugin/parser.go`
- Modify: `workflow/visual.go`

Today `plugin/parser.go:370-394` validates `seg.Role` and `a.Role` against `workflow.ValidRoles`. Those fields carry a bare role name produced by the gridlayout parser, which today splits a token like `<text.muted>` on `<...>` and stores `"text.muted"` in `Role`. After Task 4 the same token may now be `<text.muted.accent>`, and the gridlayout parser will incorrectly store the entire string in `Role`, failing the `ValidRoles` check.

There are two clean ways forward:

A. **Split inside the gridlayout parser**: extend `Segment` and `Anchor` with a `Modifier string` field and have the gridlayout role tokenizer apply the same last-dot split rule as `workflow.scanVisual`. The plugin parser then validates both fields. **This is the right long-term answer** because it keeps the segment/anchor shape faithful to the source tokens.

B. **Reuse `workflow.ValidateVisualMarkup` here**: instead of validating `seg.Role` directly, re-synthesize a minimal markup string (`"<" + seg.Role + ">x"`) and run it through the existing validator. Simpler, but stores a token that may be `text.muted.accent` in a field named `Role`, which is misleading.

This task takes **path A**.

- [ ] **Step 1: Locate the gridlayout segment/anchor types and the role tokenizer**

```bash
grep -n "type Segment\|type Anchor\|SegmentRole\|AnchorRole\|Role string" gridlayout/*.go
grep -n "splitOnDot\|LastIndexByte\|tokenize" gridlayout/*.go
```

Identify the file declaring `Segment` (likely `gridlayout/spec.go`) and the file parsing role tokens out of grid cells (likely `gridlayout/parser.go`).

- [ ] **Step 2: Add `Modifier` to both `Segment` and `Anchor` struct types**

In `gridlayout/spec.go` (or wherever the structs live):

```go
type Segment struct {
	Kind     SegmentKind
	Name     string
	Role     string
	Modifier string // optional; one of theme.KnownModifierNames or ""
	// ...existing fields...
}

type Anchor struct {
	Kind     AnchorKind
	Name     string
	Role     string
	Modifier string // optional; one of theme.KnownModifierNames or ""
	// ...existing fields...
	Segments []Segment
	Text     string
}
```

- [ ] **Step 3: Apply the last-dot split in the gridlayout role tokenizer**

In `gridlayout/parser.go`, find the code that today reads a `<token>` and assigns it to `Role`. Wrap it with the splitter:

```go
// At the package level, near the parser:
func splitRoleModifier(token string) (role, modifier string) {
	dot := strings.LastIndexByte(token, '.')
	if dot < 0 {
		return token, ""
	}
	suffix := token[dot+1:]
	if theme.IsKnownModifier(suffix) {
		return token[:dot], suffix
	}
	return token, ""
}

// At the call site that previously did `seg.Role = token`:
seg.Role, seg.Modifier = splitRoleModifier(token)
```

Add `"github.com/boolean-maybe/tiki/theme"` to the imports.

- [ ] **Step 4: Add `theme.IsKnownModifier`**

In `theme/paint.go`, add:

```go
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
```

- [ ] **Step 5: Update `plugin/parser.go` validation to check both fields**

Replace the `if _, ok := workflow.ValidRoles[seg.Role]; !ok { ... }` block (lines 374-379) and the `a.Role` block (lines 386-391) with:

```go
// segment validation (inside the composite anchor loop)
if seg.Role != "" {
	if _, ok := workflow.ValidRoles[seg.Role]; !ok {
		return gridlayout.GridSpec{}, fmt.Errorf(
			"plugin %q: metadata composite field %q: unknown color role %q", pluginName, seg.Name, seg.Role)
	}
	if seg.Modifier != "" && !theme.IsKnownModifier(seg.Modifier) {
		return gridlayout.GridSpec{}, fmt.Errorf(
			"plugin %q: metadata composite field %q: unknown color modifier %q", pluginName, seg.Name, seg.Modifier)
	}
}
```

```go
// anchor-level validation
if a.Role != "" {
	if _, ok := workflow.ValidRoles[a.Role]; !ok {
		return gridlayout.GridSpec{}, fmt.Errorf(
			"plugin %q: metadata field %q: unknown color role %q", pluginName, a.Name, a.Role)
	}
	if a.Modifier != "" && !theme.IsKnownModifier(a.Modifier) {
		return gridlayout.GridSpec{}, fmt.Errorf(
			"plugin %q: metadata field %q: unknown color modifier %q", pluginName, a.Name, a.Modifier)
	}
}
```

Add `"github.com/boolean-maybe/tiki/theme"` to the imports in `plugin/parser.go` if not already present.

- [ ] **Step 6: Write a failing test in `plugin/parser_test.go` (or `plugin/detail_test.go`)**

Find the existing test that exercises `validateDetailMetadata` with a `<role>` markup case. Add a sibling test for `<role.accent>`:

```go
func TestValidateDetailMetadata_AcceptsRoleWithModifier(t *testing.T) {
	raw := [][]string{
		{`"<text.muted.accent>caption"`, "title"},
	}
	schema := /* same schema fixture used by other tests */
	_, err := validateDetailMetadata("test", raw, schema)
	if err != nil {
		t.Errorf("unexpected err: %v", err)
	}
}

func TestValidateDetailMetadata_RejectsUnknownModifier(t *testing.T) {
	raw := [][]string{
		{`"<text.muted.bogus>caption"`, "title"},
	}
	schema := /* same schema fixture used by other tests */
	_, err := validateDetailMetadata("test", raw, schema)
	if err == nil {
		t.Error("want err for unknown modifier")
	}
}
```

Use the exact fixture-construction pattern from existing `*_test.go` neighbors (read 2-3 of them first to match the schema fixture they share).

- [ ] **Step 7: Run tests to verify the failing case fails before changes and passes after**

```bash
go test ./gridlayout/... ./plugin/... ./theme/... ./workflow/...
```
Expected: PASS.

- [ ] **Step 8: Run linter**

```bash
golangci-lint run ./...
```
Expected: 0 issues.

- [ ] **Step 9: Commit**

```bash
git add gridlayout/spec.go gridlayout/parser.go plugin/parser.go theme/paint.go plugin/detail_test.go
git commit -m "gridlayout+plugin: thread role modifier through Segment/Anchor and validate it"
```

---

## Task 12: Documentation updates

**Files:**
- Modify: `CLAUDE.md`
- Modify: `theme/doc.go`
- Modify: `workflow/visual.go` (doc comments already done in Task 4 — verify)

- [ ] **Step 1: Add a "Color roles & modifiers" section to `CLAUDE.md`**

Find the existing "Color Design Preferences" section in `CLAUDE.md` (project-level). After it, add:

```markdown
## Color roles & modifiers

Role markup in `workflow.yaml` accepts an optional modifier suffix:

- `<role>` — solid color (existing behavior).
- `<role.accent>` — mild gradient emphasis. Derived from the role's solid
  color by lightening 20%.
- `<role.lift>` — bold gradient emphasis. Derived from the role's solid
  color by 1.6× saturation boost.

Modifier vocabulary is closed (`theme.KnownModifierNames`) and validated
at workflow load time. Themes declare **only solid colors**; gradients
are derived on demand. When `theme.UseGradients` is false (8/16-color
terminals), modified roles degrade to their base solid color.

Consumers obtain a `theme.Paint` via `theme.Theme.PaintResolver()` (text
output) or a `theme.PositionPaint` via `theme.PaintForRolePosition`
(screen-cell output). The solid-vs-gradient branch lives entirely inside
the `solidPaint` / `gradientPaint` types in `theme/paint.go`. View code
never branches on whether a role is solid or gradient.
```

- [ ] **Step 2: Update `theme/doc.go`**

Add a paragraph documenting the new interfaces and modifier vocabulary, mirroring the CLAUDE.md content but framed for Go-package consumers. Reference `PaintResolver`, `PaintForRolePosition`, and `KnownModifierNames`.

- [ ] **Step 3: Verify `workflow/visual.go` doc comments**

The doc comment on `ExpandVisual` in Task 4 already describes the new grammar. Re-read it to confirm clarity.

- [ ] **Step 4: Run doc-formatting checks (if the repo has them)**

```bash
go doc ./theme | head -40
go doc ./workflow | head -40
```
Expected: documentation reads cleanly with no broken references.

- [ ] **Step 5: Run linter**

```bash
golangci-lint run ./...
```
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md theme/doc.go workflow/visual.go
git commit -m "docs: document color role modifiers and Paint interfaces"
```

---

## Task 13: Final verification

- [ ] **Step 1: Run the entire test suite**

```bash
go test ./...
```
Expected: PASS.

- [ ] **Step 2: Run the linter on the full repo**

```bash
golangci-lint run ./...
```
Expected: 0 issues.

- [ ] **Step 3: Smoke-build the binary**

```bash
go build ./...
```
Expected: clean build, no errors.

- [ ] **Step 4: Confirm zero references to the old API anywhere**

```bash
grep -rn "GradientRole\|TikiIDGradient\|CaptionFallbackGradient\|tikiIDGradient\|captionFallbackGradient\|CaptionFallbackStart\|CaptionFallbackEnd\|gradientRole\b\|newGradientRole\|\.RoleResolver()" --include="*.go" .
```
Expected: zero matches (excluding `.claude/worktrees/` from old branches).

- [ ] **Step 5: Confirm no `claude/` branch slipped in**

```bash
git rev-parse --abbrev-ref HEAD
```
Expected: `feature/color-role-modifiers`.

- [ ] **Step 6: Skim the diff for accidental author-identity leaks**

```bash
git diff dev..feature/color-role-modifiers | grep -iE "/users/|/home/|@"
```
Expected: no matches (per project privacy rule).

- [ ] **Step 7: Optional — view the rendered TUI under the new model**

Spawn the `tiki-capture` skill, or run `tiki` against a project with sample data and screenshot the kanban view to confirm the caption row gradient and task ID gradient still look right. Compare visually against a screenshot from `dev`. This is the final acceptance check.
