# Color Role Modifiers — Design

**Status:** approved, ready for implementation plan
**Date:** 2026-05-16
**Branch context:** `dev` (no feature branch yet; create at plan stage)

## Problem

Tiki's UI today colors text and surfaces through a closed vocabulary of
semantic *roles* (e.g. `text.primary`, `status.danger`). A workflow author
can reference a role from `workflow.yaml` via `<role>text` markup, and the
theme binds each role to a single solid color (`tcell.Color`).

Gradients exist in the codebase but live outside this vocabulary. They are
declared as dedicated `*Theme` fields (`tikiIDGradient`,
`captionFallbackGradient`) consumed by hand-coded renderers
(`view/task_box.go`, `view/gradient_caption_row.go`). A workflow author has
no way to ask for a gradient from `workflow.yaml`, and the solid-vs-gradient
branch is spread across views.

The upcoming configurable task box (future work) compounds the problem: the
renderer will receive role references parsed from `workflow.yaml` and apply
them sight unseen. It cannot pre-bake gradient handling for any specific
role; it needs a uniform mechanism that hides the solid-vs-gradient
distinction at the point of use.

## Goal

Let any role reference in `workflow.yaml` request a gradient via a small,
closed vocabulary of *modifier* suffixes — without the workflow author
naming a second color, declaring endpoints, or otherwise describing the
gradient. The author expresses **intent** (mild or bold emphasis); the
theme system derives the gradient from the base role's resolved color.

Consumers of role references stay role-blind: they obtain a render-time
strategy object and apply it. The solid-vs-gradient branch lives in
exactly two places (one per consumer flavor) and nowhere else.

## Non-goals

- Per-theme gradient endpoint declarations. Themes continue to declare
  only solid colors; gradients are derived from those colors.
- Arbitrary user-defined modifier names. The vocabulary is curated and
  closed, validated at workflow load time.
- Modifier chaining (`<role.accent.lift>`). One modifier per reference.
- Building the configurable task box. This design unblocks it; the box
  itself is separate work.

## Vocabulary & syntax

A role reference in `workflow.yaml` is one of:

- `<role>` — a solid color, behavior identical to today.
- `<role.modifier>` — a gradient derived from the role's solid color via
  the algorithm bound to `modifier`.

The base `role` is any name `theme.Theme.ResolveByName` accepts today
(canonical dotted names plus legacy aliases). The optional `.modifier` is
exactly one of:

- **`.accent`** — mild emphasis. Derives a gentle gradient (darken-by-20%
  start, original-color end). Matches today's `gradientFromColor(c, 0.2)`
  algorithm byte-for-byte.
- **`.lift`** — bold emphasis. Derives a vibrant gradient
  (saturation-boosted start, original-color end). Matches today's
  `GradientFromColorVibrant` algorithm with `vibrantBoost = 1.6`
  byte-for-byte.

The literal-`<` escape rule (`<<` → `<`) is unchanged.

**Disambiguation:** when a token contains dots, the parser splits on the
**last** dot. If the suffix is a known modifier, the prefix is the base
role; otherwise the entire token is treated as a base role name (preserves
existing dotted names like `text.muted`, `statusline.main.fg`). This is
unambiguous as long as no canonical role name ends in `.accent` or
`.lift`. A startup-time assertion in `theme/` guards this invariant.

**Validation:** `workflow.ValidateVisualMarkup` learns the closed modifier
vocabulary alongside the existing role vocabulary. Typos like `.lifted`
fail loudly at workflow load — same failure model as unknown role names.

## Interface shape

Two new interfaces in `theme/`, one for each flavor of consumer:

```go
// Paint applies a role (optionally modified) to a string. Solid
// implementations emit a single tview color tag; gradient implementations
// emit one tag per visible rune.
type Paint interface {
    PaintString(s string) string
}

// PositionPaint yields a tcell.Color for a normalized position t in
// [0,1]. Solid implementations return the same color regardless of t;
// gradient implementations interpolate.
type PositionPaint interface {
    ColorAt(t float64) tcell.Color
}
```

Two concrete implementations (unexported, in `theme/paint.go`) satisfy
**both** interfaces:

- `solidPaint{ role Role }` — `PaintString` returns `role.Tag() + s + "[-]"`;
  `ColorAt` returns `role.TCell()` regardless of `t`.
- `gradientPaint{ base Role, algo gradientAlgo }` — at render time,
  computes the gradient from `base.TCell()` via `algo`. `PaintString`
  walks the runes of `s` emitting one `[#rrggbb]` tag per rune.
  `ColorAt(t)` interpolates the gradient at `t`. Uses the existing
  `util/gradient.InterpolateRGB` math so output stays byte-identical to
  today's gradient renderers.

A `gradientAlgo` is a small strategy type — one function per modifier —
that takes a base `tcell.Color` and returns a `theme.Gradient`. Adding a
new modifier is one new function plus one switch entry.

The modifier → implementation table:

```go
func paintFor(base Role, modifier string) (Paint, bool) {
    switch modifier {
    case "":       return solidPaint{role: base}, true
    case "accent": return gradientPaint{base: base, algo: algoAccent}, true
    case "lift":   return gradientPaint{base: base, algo: algoLift}, true
    }
    return nil, false
}
```

This is the single point of truth that ties grammar to implementation.
The workflow loader's modifier vocabulary is derived from the same
switch (via a `KnownModifierNames()` helper), so adding a modifier later
auto-updates the validator.

**Existing `theme.Role` is unchanged.** Solid-only call sites continue to
call `role.Tag()`, `role.TCell()`, etc. `Paint` is opt-in: only call
sites that consume `<role.modifier>` markup or were previously
special-cased for gradients migrate to `Paint`.

## Wiring through the workflow loader

The pipeline `workflow.yaml` → `ValidateVisualMarkup` → `ExpandVisual` →
tview text stays in place. The changes are localized:

**`workflow.scanVisual` (tokenizer):** when a token `<x>` is encountered,
attempt to split `x` on the last dot. If the suffix is a known modifier,
use `(prefix, suffix)` as `(role, modifier)`; otherwise use `(x, "")`.
The tokenizer otherwise unchanged.

**Resolver signature:** the resolver passed into `ExpandVisual` evolves:

```go
// Today
type RoleResolver func(role string) (tag string, ok bool)

// New
type RoleResolver func(role, modifier string) (Paint, bool)
```

`ExpandVisual` switches from "emit one tag, write text, emit `[-]`" to
"obtain a `Paint`, write `paint.PaintString(text)`". For solid roles
(empty modifier), the resulting byte sequence equals today's output.
The "global trailing `[-]`" behavior goes away — each painted segment
emits its own local reset inside `Paint.PaintString`, which is cleaner
when modified and plain segments interleave.

**Theme adapter:** `Theme.RoleResolver()` is replaced by
`Theme.PaintResolver()` returning the modifier-aware signature. The old
`RoleResolver` method is deleted (no compat shim — full migration).

## Migrating existing gradient consumers

End state: **no parallel systems.** `GradientRole`, the dedicated theme
fields (`tikiIDGradient`, `captionFallbackGradient`), and palette-level
gradient endpoint declarations all go away. Everything routes through
`Paint` / `PositionPaint`.

### Inventory

The implementation plan will enumerate consumers via grep. From the
exploration done during brainstorming, the known set is:

1. **Task ID rendering** (`view/task_box.go`). Currently calls
   `theme.Roles().TikiIDGradient()` directly. Migrates to consuming a
   `Paint` resolved from a named role with `.accent` modifier. The
   *current* fixed task box keeps the same visual output during this
   change; the configurable task box (future work) consumes the same
   `Paint` resolver and gains gradient support for free, no further
   theme changes required.

2. **Caption row background** (`view/gradient_caption_row.go`). Screen-
   cell painter case. Migrates from `theme.Gradient` field to a
   `PositionPaint` field. The caller resolves the per-plugin base color
   with `.lift` modifier and hands the resulting `PositionPaint` to
   `GradientCaptionRow`. The widget loses its `gradient theme.Gradient`
   field and `computeCaptionGradient` helper; it just calls
   `paint.ColorAt(t)` per column.

3. Any other `GradientRole` / `theme.Gradient` / `util/gradient`
   consumer outside `theme/`. Plan-stage grep enumerates these.

### Deletions

- `theme.GradientRole` interface and `gradientRole` concrete type
  (`theme/role.go`).
- Theme fields `tikiIDGradient` and `captionFallbackGradient` and their
  getters (`theme/theme.go`, `theme/binding.go`).
- All `newGradientRole(...)` calls in `theme/binding.go` (14 themes × 2
  calls = 28 lines).
- Each palette's `CaptionFallbackStart` / `CaptionFallbackEnd` fields
  (`theme/palettes/*.go`). These were gradient-endpoint declarations —
  exactly the kind of knob the new model promises themes don't have.
- The old `Theme.RoleResolver()` method.

### Fallback simplification

Today's `gradientRole.FallbackRole() Role` lets gradient roles render as
a solid color on 8/16-color terminals. The new `gradientPaint` keeps the
capability with a simpler binding: when `theme.UseGradients.Load()` is
false, **`gradientPaint` degrades to `solidPaint{role: base}`** — i.e.
`PaintString` emits a single `[base.Tag()]s[-]` and `ColorAt` returns
`base.TCell()` regardless of `t`. The "fallback role" is the same role
used as the gradient's source, not a separately-bound third color. No
theme-level fallback binding to keep in sync. If a real need for a
separate fallback ever appears, adding it back is one field on
`gradientPaint`.

The terminal-capability switches `theme.UseGradients` and
`theme.UseWideGradients` stay where they are; the conditional just
moves inside the two `gradientPaint` methods. View code stops checking
terminal capability — that's now a theme-internal concern.

### Unchanged

- `util/gradient` package and its `InterpolateRGB` / `GradientFromColor`
  / `GradientFromColorVibrant` math. `gradientPaint` calls into these
  internally; output stays byte-identical for migrated consumers.
- `theme.Roles()` accessor and `SetTheme` swap mechanism.
- Every existing solid-role call site (e.g. `TextPrimary().Tag()`).

## Verification

- **Existing snapshot tests** in `theme/snapshot_test.go` cover solid
  roles. They must pass unchanged — solid is the load-bearing case for
  the entire UI.
- **New snapshot test** asserts `gradientPaint.PaintString` matches a
  golden per-rune tag sequence for a fixed base color and modifier.
  Locks the algorithm so future refactors can't silently drift output.
- **Regression test** for `GradientCaptionRow`: render byte-identical
  output to the pre-migration code for one fixed terminal size + theme
  + lane configuration. This is the strongest guarantee that "no
  visible change" holds for the most prominent gradient surface.
- `golangci-lint run ./...` returns 0 issues (project rule).

## Documentation updates (part of implementation)

- **`CLAUDE.md`** (project) — new "Color roles & modifiers" section
  documenting the `.accent` / `.lift` vocabulary, the resolver shape,
  and the rule that themes never declare gradient endpoints.
- **`theme/doc.go`** — extend role-vocabulary documentation with the
  modifier set and `Paint` / `PositionPaint` interfaces.
- **`workflow/visual.go`** doc comments — describe the grammar
  (`<role>` or `<role.modifier>`, closed modifier vocabulary, last-dot
  split rule, escape rule unchanged).
- **`config/workflows/kanban.yaml`** (and any sample workflows) — sweep
  for hand-coded color decisions that the modifier vocabulary can now
  express. Nice-to-have, not gating.

## Open questions

None at design time. Decisions captured in brainstorming:

- Vocabulary: `.accent` (mild) + `.lift` (bold). Chosen for intent
  semantics, not appearance.
- Scope: any role can be a gradient via modifier (not opt-in subset).
- Migration: full — delete `GradientRole` and parallel theme fields, no
  compat shim.
- Interface shape: two narrow interfaces (`Paint`, `PositionPaint`),
  same two concrete types satisfy both.
- Parser disambiguation: last-dot split with startup-time invariant
  check that no canonical role ends in a modifier name.
- Fallback: base role is its own fallback (no separate binding).
