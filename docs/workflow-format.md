# Workflow Format Versions

This document tracks the evolution of the `workflow.yaml` schema. Each section describes one format version:
what it introduced and what changed from the previous version. For usage details see
[Customization](customization/customization.md). For file locations and precedence see [Configuration](config.md).

---

## 0.6.0

Breaking redesign of the views and actions schema as part of the unified-documents effort.
Removes the `type: tiki` / `type: doki` split and replaces it with explicit view kinds.
Promotes `actions:` to a top-level section so globals are not coupled to the views wrapper.
No backwards-compatibility shims — old workflows must migrate.

### New top-level shape

```yaml
version: "0.6.0"

fields:
  - name: status
    type: enum
    values:
      - value: backlog
        label: Backlog
        default: true
      - value: done
        label: Done
  - name: type
    type: enum
    values:
      - value: story
        label: Story
        default: true
  - name: severity
    type: enum
    values:
      - value: low
      - value: medium
        default: true
      - value: high

actions:                       # top-level global actions
  - key: "a"
    kind: ruki
    label: "Assign to me"
    action: update where id = id() set assignee=user()
  - key: F11
    kind: view
    label: "Open board"
    view: kanban               # name of a view declared below

views:                         # top-level list (no `plugins:` wrapper)
  - name: kanban               # stable identifier (used by actions[].view)
    label: Kanban              # optional display text — falls back to name
    kind: board
    default: true
    layout: |                   # required: the tiki-box layout shared by all lanes
      type.visual + " " + id
      <highlight>title
      "priority " + priority.visual
    lanes:
      - name: Backlog
        filter: select where status = "backlog"
      - name: Done
        filter: select where status = "done"

  - name: docs
    kind: wiki
    path: index.md              # `document: <ID>` (ID-based binding) is not yet implemented

  - name: selected
    kind: detail
    require: ["selection:one"]
    layout: |
      <highlight>title | --
      "Status:"       | status
```

### View `kind:` replaces `type:`

| kind      | purpose                                                                  | required fields           | status                                |
|-----------|--------------------------------------------------------------------------|---------------------------|---------------------------------------|
| `board`   | kanban-style lanes with per-lane filters and move actions                | `lanes`                   | shipped                               |
| `list`    | single-column list view                                                  | `lanes` (typically one)   | shipped                               |
| `wiki`    | markdown viewer bound to a document by relative path                     | `path:`                   | shipped (path only; see below)        |
| `detail`  | configurable single-tiki view: title, declared metadata fields, body     | —                         | shipped                               |
| `search`  | the global search view                                                   | —                         | **not implemented** — parser rejects  |
| `timeline`| future phase                                                             | —                         | reserved — parser rejects             |

`wiki` views accept `path:` today. The alternative `document: <ID>` form (binding a wiki view to a task by id
rather than by relative path) is **not implemented**: the parser rejects any view that sets `document:`. A clean
ID-binding story needs a richer document-index contract than the current `store.PathForID` exposes, so it is
deferred as a future enhancement without a scheduled phase.

`board`, `list`, and `detail` views all share the same `layout:` field, which declares the 2D
layout grid of fields that compose the rendered card or detail box. `layout:` is **required** on
these view kinds. Description is always rendered by detail views; title renders only if declared
in the `layout:` grid. The previous "render the selected document's raw markdown" meaning of
`kind: detail` is gone — that behavior now lives only on `kind: wiki`.

`layout:` is a YAML block scalar (`|`) whose content describes the grid one row per line.
Cells within a row are separated by `|`. Inside a double-quoted string, `|` is literal and
not treated as a cell delimiter. Each cell is trimmed of surrounding whitespace before
tokenization. Blank lines and lines starting with `#` are ignored. The grid visually mirrors
the rendered layout box.

```yaml
views:
  - name: Detail
    kind: detail
    require: ["selection:one"]
    layout: |
      <highlight>title | --         | --            | --
      "Status:"       | status     | "Type:"       | type
      "Priority:"     | priority   | "Points:"     | points
      "Assignee:"     | assignee   | "Due:"        | due
      "Recurrence:"   | recurrence | "Created:"    | createdAt
      "Updated:"      | updatedAt  | "By:"         | createdBy
      "Tags:"         | tags       | "Depends On:" | dependsOn
    actions:
      - key: "a"
        label: "Assign to me"
        action: update where id = id() set assignee=user()
```

Cell vocabulary:

| Cell              | Meaning                                                                  |
| ----------------- | ------------------------------------------------------------------------ |
| `name`            | field, value-only — sizes to its content (auto), uncapped                |
| `name:N`          | field, fixed width of exactly N character cells                          |
| `name:auto`       | field, size to content; `name:auto..N` caps at N then truncates with `…` |
| `name:fr`         | field, grows to absorb residual width; `name:Wfr` takes weight W (def 1) |
| `name:MIN..MAX`   | bounds on any mode: `:8..`, `:..20`, `:8..20`. A plain `:fr` floors at    |
|                   | its content (min-content); an explicit `:0..` floor lets it shrink to 0. |
| `name?`           | hide this field (and its `.caption`) when the tiki has no value for it.   |
|                   | Composes with sizing: `tags?:fr`, `assignee?:auto..20`.                  |
| `name.visual`     | field, show the visual indicator (emoji/icon) instead of the label       |
| `name.caption`    | field's caption text (label), rendered as a static label rather than the |
|                   | value. Sheds together with the field's value cell.                       |
| `name.count`      | item count of a list field, rendered as an integer (`3`). Valid only on   |
|                   | `stringList` / `tikiIdList` fields — validated at workflow load. Empty or |
|                   | missing list renders `0`; hide it with `name?.count`. Works standalone    |
|                   | (`tags.count`, `dependsOn?.count:4..`) and as a composite segment         |
|                   | (`("Deps: " + dependsOn.count)`).                                        |
| `<role>name`      | field with semantic color role — replaces default styling for text       |
|                   | fields (title, custom strings). Structured fields ignore the role.       |
|                   | Bare-legal in YAML (no quoting needed).                                  |
| `a + " " + b`    | composite cell — concatenates field refs and/or `"quoted"` literals.     |
|                   | At least one segment must be a field ref. Segments support `.visual`,    |
|                   | `.count`, and `<role>` individually; the cell sizes to its rendered      |
|                   | content (segment-level `:N` sizing is rejected — size the whole cell).    |
|                   | Single-field composites are editable; multi-field render read-only.      |
| `"any text"`      | literal caption — any quoted YAML string that is not a bare marker or a  |
|                   | valid identifier. Used to label adjacent fields. Width defaults to the   |
|                   | text length + 1.                                                         |
| `--`              | column span — continue the anchor immediately to the left                |
| `^`               | row span — continue the anchor immediately above                         |
| `_`               | empty cell                                                               |

Fields render value-only — there is no automatic `Status:` prefix in the value cell anymore.
Place captions explicitly in the grid using literal strings (e.g. `"Status:"`) wherever you
want them. This is the same mechanism a layout author would use to label any custom workflow
field.

Semantic role vocabulary for `<role>`:

| Role          | Intent                                                                       |
| ------------- | ---------------------------------------------------------------------------- |
| `<muted>`     | De-emphasized text — labels, placeholders, unfilled bar segments             |
| `<accent>`    | Primary accent (green) — done-status, filled bar segments, field captions    |
| `<highlight>` | Bright focus (theme highlight color) — selection, focus marker, title text   |
| `<info>`      | Informational (orange) — header view name, plugin-switch keys                |
| `<action>`    | Contextual action shortcut (cyan-blue) — view-scoped action keys             |
| `<text>`      | Primary readable text — neutral, highest-contrast                            |
| `<danger>`    | Critical / error / blocker (red)                                             |
| `<warn>`      | Warning / attention (orange) — same hex as `<info>`                          |
| `<ok>`        | Healthy / success (green)                                                    |

The set is closed: any other name fails workflow load. The mapping from role name to
concrete color lives in `config.ColorConfig.ResolveRole` and changes per theme. Two roles
intentionally share a color in every theme: `<info>` and `<warn>` both resolve to
`WarnColor`, and the statusline-info foreground shares `OkColor` with `<ok>`. This is by
design after the InfoLabelColor → WarnColor and StatuslineOk → OkColor merges.

Cell delimiter and quoting: `|` separates cells within a row. To include a literal `|` in a
caption, wrap the caption in double quotes (e.g. `"a | b"`). Markers `--`, `_`, and `^` are
written as-is — no YAML quoting is needed because the block scalar is one big string. Lines
starting with `#` are treated as comments and skipped; to start a caption with `#`, quote it
(e.g. `"#tag"`).

Width and shedding semantics:

- A bare field name sizes to its content (max-content), uncapped. The flat per-field default
  width is gone.
- `:fr` columns split the residual width left after fixed/content columns are sized, in
  proportion to their weights (`2fr` gets twice a `1fr`). This replaces the old `<->` stretcher.
- Bounds (`:MIN..MAX`) clamp any mode. An `auto` value longer than `MAX` is truncated with a
  single-cell `…`.
- A `:fr` column will not shrink below its content (min-content) unless given an explicit `:0..`
  floor. A `:fr` column may also be given a usable **grow floor** (e.g. `:16..fr`): the solver
  counts that floor toward required width, so a grow column that can't be granted its floor is shed
  (or forces a neighbour to shed) instead of shrinking to a useless sliver. This lets one grow
  column both absorb slack at wide widths and shed cleanly when narrow. A plain `:fr` (no floor)
  keeps absorbing whatever residual remains and never forces a shed.
- If the terminal can't fit the row, the column with the **lowest floor** is dropped; ties break
  right-to-left. Drop order therefore follows ascending floor — a column survives longer the higher
  its floor. Mark a column droppable with a low (or `0`) floor; pin one by giving it a high floor or
  a fixed `:N`.
- **A field's value and its `.caption` always shed together.** Caption and value may sit in
  different columns (caption-beside-value layouts); when either is dropped by the width algorithm,
  the other is dropped too, so a label never survives with no value beside or beneath it. The
  pairing is by field name, so use `field.caption` cells (not literal-string captions) for any field
  whose caption must shed with its value. Literal-string captions have no field identity and do not
  participate in co-shedding.
- A multi-column (`--`) span contributes only a content minimum to its columns; it never sets a
  column's sizing mode. A column's mode comes from the single-column cells that occupy it.

Layout traversal and edit order:

- Layout anchors are *declared* top-to-bottom, left-to-right (row-major) in the grid block, but
  edit-mode Tab order is **column-major**: focus moves down a column top-to-bottom first, then to
  the next column left-to-right. Arrange columns so the natural Tab order reads down the
  highest-frequency column first.
- Every cell is left-aligned in v1 — no syntax for right/center alignment yet.
- Caption literals appear in the read-only detail view exactly as written. The full-screen
  TaskEditView (the separate "edit" form, not the in-place editors on the detail view) currently
  re-packs fields into a flat multi-column layout and **does not** surface caption literals.
  In-place edit mode on the detail view does honor caption literals because it reuses the same
  parsed grid.

Per-view actions register *after* built-in detail actions, so a per-view entry that reuses a built-in
key (such as `e` for Edit) shadows the built-in. Avoid the keys the detail view already uses for Edit,
Fullscreen, and Edit source unless you intentionally want to replace them.

Validation rules for `layout:` on `kind: detail`:

- `layout:` accepts workflow-declared field names plus the supported audit fields (`createdBy`,
  `createdAt`, `updatedAt`). Unknown names fail workflow load.
- Identity/body fields `description`, `body`, and `id` are rejected — they are always rendered by
  the detail view chrome. `title` IS allowed and renders in-grid like any other text field.
  If omitted from the grid, no title is displayed. Accepts optional `<role>` annotation.
- Path fields (`filepath`, `path`) — values live on the tiki struct rather than in Fields and have
  no typed renderer; rejected.
- Grid-shape errors (ragged rows, orphan `--` or `^`) fail workflow load with `row,col`
  coordinates.
- Board/list-only fields (`lanes:`) and wiki-only fields (`path:`, `document:`) are rejected.
- Per-view `actions:` are allowed and surface alongside the built-in detail actions.
- `require:` is honored as the navigation gate (typically `["selection:one"]`).

The detail view recognizes any field declared in `workflow.yaml fields:` plus the kanban-style
well-known names (`status`, `type`, `priority`, `points`, `assignee`, `due`, `recurrence`, `tags`,
`dependsOn`). Of those, `status`, `type`, and `priority` are fully editable in place; the rest render
read-only today and will gain richer editors in a future iteration.

Anti-pattern examples (each fails workflow load):

```yaml
# orphan column span — no anchor to the left
layout: |
  -- | status
```

```yaml
# ragged rows
layout: |
  status | type | priority
  points
```

Audit fields (`createdBy`, `createdAt`, `updatedAt`) are supported and render as read-only rows.
The bundled kanban workflow includes them in its `layout:` grid.

Opening a detail view goes through normal action dispatch:

```yaml
actions:
  - key: Enter
    label: Open
    kind: view
    view: Detail
    require: ["selection:one"]
```

`Enter` is no longer a built-in board/list shortcut — it is the workflow `kind: view` action above. The
selected tiki threads through `PluginViewParams` so the target detail view's `require:` is honored and the
correct document is rendered.

### Top-level `actions:` with `kind: ruki | view`

Actions declared at the top level are global — available from every view. A view's own `actions:` list still
overrides globals by key. There are two action kinds:

- `kind: ruki` — runs a ruki statement (this is the pre-Phase-6 behavior; the `action:` field carries it).
  Fires from every view kind. When invoked from a wiki/detail view that received a selection via navigation,
  that selection threads into the ruki `ExecutionInput` so `id()` and `set <field> = ...` resolve against it.
  Exception: the interactive variants — actions that set `input:` or use `choose()` — currently fire only from
  board/list views. On wiki/detail views they are filtered out of the action registry and refused by the
  dispatcher, because non-board controllers do not implement the input/choose prompt pipeline today. A future
  enhancement may lift this restriction (see "Not implemented in Phase 6" in the plan).
- `kind: view` — navigates to another view by name. The target must exist in the `views:` list. When one task
  is selected on the source view (or the source view received a selection via a prior `kind: view` action), the
  selection is encoded into `PluginViewParams` and carried into the target so `require: ["selection:one"]` on
  the target view is honored and `kind: detail` views render the carried document.

When `kind:` is omitted, the parser infers it: `action:` set ⇒ `ruki`; `view:` set ⇒ `view`. Setting both or
neither is an error.

### `mode:` on `kind: view` actions targeting a detail view

A `kind: view` action that targets a `kind: detail` view may declare a `mode:` field that controls how the
detail view opens. The vocabulary is closed:

- `view` — open the detail view in read-only display mode. This is the default when `mode:` is omitted.
- `edit` — open the detail view in edit mode, focused on the first metadata field declared in the
  workflow's `metadata:` list.
- `new` — create a fresh draft tiki and open the detail view in edit mode focused on Title.
- `edit-desc` — open the detail view in edit mode focused on the Description textarea.
- `edit-tags` — open the detail view in edit mode focused on the Tags textarea.

Validation rules, enforced at workflow-load time:

- `mode:` is only valid on `kind: view` actions. Setting it on a `kind: ruki` action fails with
  `mode: only valid on kind: view actions`.
- The action's `view:` target must resolve to a `kind: detail` view. Pointing `mode:` at a board, list, or
  wiki view fails with `mode: only valid when targeting a kind: detail view`.
- `mode: new` is rejected on a detail view's own `actions:` list — a detail view is already viewing a tiki,
  so creating a new one from there is not a self-action. The error is
  `mode: new not valid on a detail view's own actions`.
- An unrecognized value fails with `mode must be one of view, edit, new, edit-desc, edit-tags (got "X")`.

The bundled kanban workflow uses all five modes from its top-level `actions:` list:

```yaml
actions:
  - key: Enter
    label: Open
    kind: view
    view: Detail
    require: ["selection:one"]
  - key: "e"
    label: Edit
    kind: view
    view: Detail
    mode: edit
    require: ["selection:one"]
  - key: "n"
    label: New
    kind: view
    view: Detail
    mode: new
  - key: "Ctrl-D"
    label: "Edit description"
    kind: view
    view: Detail
    mode: edit-desc
    require: ["selection:one"]
  - key: "Ctrl-T"
    label: "Edit tags"
    kind: view
    view: Detail
    mode: edit-tags
    require: ["selection:one"]
```

Note that the `New` action has no `require:` clause — `mode: new` synthesizes its own draft tiki rather than
acting on a selection, so the action is available even when no row is selected on the source view.

### `[[ID]]` wikilinks

Markdown bodies may reference other documents by their bare ID inside `[[...]]` brackets. Resolution goes
through `store.ReadStore.PathForID` + `GetTask` so links survive file moves. Unknown ids render as a literal
`[[<ID>]]` with a `*(not found)*` marker so broken references are visible rather than silently dropped. Wiring
is active on task detail views and wiki/detail plugin views.

### Migration from 0.5.x

Old configs are rejected with specific errors that point at the new syntax. The mapping:

| pre-0.6.0                                       | 0.6.0                                                 |
|------------------------------------------------|-------------------------------------------------------|
| `type: tiki`                                   | `kind: board` (or `kind: list` for 1-lane views)      |
| `type: doki` + `fetcher: file` + `url: x.md`   | `kind: wiki` + `path: x.md`                           |
| `type: doki` + `fetcher: internal` + `text:`   | write a `.md` file under `.doc/` and use `kind: wiki` |
| `views.plugins:` wrapper                       | top-level `views:` list                               |
| `views.actions:`                               | top-level `actions:`                                  |
| `view: compact`/`view: expanded` on a view     | declare a `layout:` field on the board/list view      |
| `mode: compact`/`mode: expanded` on a view     | declare a `layout:` field on the board/list view      |
| `metadata:` on a detail view                   | rename to `layout:` (same grid syntax)                |
| `sort: <field>` on a view                      | `order by <field>` inside each lane's `filter:`       |
| top-level `statuses:`                          | `fields:` entry named `status` with `type: enum`      |
| top-level `types:`                             | `fields:` entry named `type` with `type: enum`        |

### Rejection-error table

Users upgrading will see one of these messages; each names the legacy field and points at its replacement:

- `"type" is no longer supported — use kind: board instead`
- `"type" is no longer supported — use kind: wiki instead`
- `"view:" as a display mode is no longer supported — declare a layout: field on the board/list view instead`
- `"mode" is no longer supported — use layout: to declare the tiki-box layout on a board/list view`
- `"metadata" is no longer supported — metadata: has been renamed to layout: — update your workflow.yaml`
- `"fetcher" is no longer supported — use document: or path: on a kind: wiki view`
- `"text" is no longer supported — use document: or path: on a kind: wiki view`
- `"url" is no longer supported — use document: or path: on a kind: wiki view`
- `"sort" is no longer supported — use order by inside a lane's filter: ruki statement`
- `top-level statuses: is no longer supported; define status as a fields: enum`
- `top-level types: is no longer supported; define type as a fields: enum`
- `views: must be a top-level list — the views.plugins wrapper is no longer supported`
- `unknown view kind "X" — expected board, list, wiki, or detail`
- `kind: timeline is reserved but not yet implemented`
- `kind: search is reserved but not yet implemented` (the built-in global search UI is not plugin-instantiable today)
- `document: (ID-based resolution) is not yet implemented — use path: with a relative filepath`
- `view kind "board" requires a non-empty layout: field` — missing/empty `layout:` on a board, list, or detail view
- `layout: only valid on kind: board, list, or detail` — set on a wiki view
- ``layout cannot include "description"`` (or `body`/`id`) — identity/body fields are always rendered by the detail view chrome
- `layout field "X" is not a workflow-declared field` — typo, or the field is not in `workflow.yaml fields:`
- `layout: row N has M cells, expected K` — the grid has a ragged row
- ``layout: row N, col M: orphan '--'`` — no anchor to the left of the column-span marker
- ``layout: row N, col M: orphan row-span '^'`` — no anchor above the row-span marker
- ``cell "X": invalid sizing …`` — a malformed `:[mode][min..max]` sizing suffix (e.g. `:0`, `:garbage`)

Loading is fail-closed: any one of these errors (or any lane/action/require parse failure) refuses the whole
workflow rather than silently loading only the views that parsed. A partial workflow would diverge from what you
declared, so boot is refused until the file is fixed.

---

## 0.5.3

Removes `new.md` task template files. Task-creation defaults now live entirely in `workflow.yaml`.

### `new.md` removed

The separate `new.md` template file (and its discovery/precedence chain) is eliminated.
`tiki workflow install` no longer downloads `new.md`. `tiki workflow reset new` is removed.

### Type `default: true`

Type entries now support an optional `default: true` flag, matching status behavior.
When set, that type is used for new tasks instead of the positional first-type fallback.
Multiple `default: true` types are rejected at load time.

```yaml
types:
  - key: bug
    label: Bug
    emoji: "🐛"
    default: true
  - key: regression
    label: Regression
    emoji: "🔁"
```

### Field `default:` key

Custom field definitions accept an optional `default:` value. The value is validated
against the field's type and enum constraints during workflow load — invalid defaults
are hard errors. Valid defaults are copied into new tasks automatically.

```yaml
fields:
  - name: severity
    type: enum
    values: [critical, high, medium, low]
    default: medium
  - name: regression
    type: boolean
    default: false
```

### Built-in defaults

Built-in field defaults remain hardcoded and are not configurable:
- `priority` = 3
- `points` = 1
- `tags` = `["idea"]`

List and collection defaults use set semantics:
- values are trimmed
- empty entries are dropped
- duplicate entries are removed

### Removed template-only defaults

Fields that were only configurable via `new.md` frontmatter are dropped:
`description`, `title`, `dependsOn`, `due`, `recurrence`, `assignee`.

---

## 0.5.1

Expands action keys beyond single characters, adds `choose()` and `require` to actions,
removes the 10-action limit.

### Action key format expanded

Action keys now support modifier combos and function keys:

```yaml
actions:
  - key: Ctrl-Q
    label: "Quick create"
    action: create title=input()
    input: string
```

Supported formats:
- single character: `"a"`, `"Y"`, `"+"`
- function keys: `F1` through `F12`
- modifier combos: `Ctrl-X`, `Alt-X`, `Shift-X`
- modifier + function key: `Ctrl-F1`, `Alt-F3`, `Shift-F5`

Keys are normalized to a canonical form — `Shift-x` and `Shift-X` both become `X`.
Duplicate detection uses the canonical string, not the raw input.

### `choose()` builtin

Actions can use `choose()` to open an interactive task picker:

```yaml
actions:
  - key: "e"
    label: "Link to epic"
    action: update where id = choose(select where type = "epic") set dependsOn = dependsOn + id()
```

- `choose()` takes exactly one argument: a `select` subquery
- may appear once per action
- mutually exclusive with `input()` in the same action
- lane filters and lane actions cannot use `choose()` or `input()`

See [Choose-backed actions](customization/customization.md#choose-backed-actions).

### `require` field on actions

Actions can declare context requirements that control when they are enabled:

```yaml
actions:
  - key: "c"
    label: "Chat about task"
    action: select where id = id() | run("claude -p 'Discuss: $1'")
    require: ["ai", "id"]
```

- `require` — list of context attribute strings (optional)
- selection cardinality attributes: `id` / `selection:one` (exactly one selected), `selection:any` (one or more),
  `selection:many` (two or more)
- other built-in attributes: `ai` (AI agent configured), `view:<view-id>` (active view)
- `id` is auto-inferred when the action uses `id()`; `selection:any` is auto-inferred when the action uses
  `ids()`; `selected_count()` does not auto-infer anything because it is designed for zero-selection branches
- explicit entries are allowed but redundant unless you want to tighten the constraint (e.g.
  `require: ["selection:many"]`)
- negation: prefix with `!` (e.g. `"!view:plugin:Kanban"` — disabled when on Kanban view)
- disabled actions show greyed out in header and palette; hotkey is ignored

See [Action requirements](customization/customization.md#action-requirements).

### 10-action limit removed

Actions per section are no longer capped at 10.

---

## 0.5.0 — Baseline

First versioned workflow format. Establishes the complete schema.

### Top-level structure

```yaml
version: "0.5.0"
description: |
  Multi-line workflow description.
statuses: [...]
types: [...]
views: { actions: [...], plugins: [...] }
fields: [...]
triggers: [...]
```

- `version` — semver string (optional)
- `description` — multi-line text via YAML block scalar `|` (optional)
- `statuses` — status definitions (required)
- `types` — task type definitions (required)
- `views` — view configuration with global actions and plugins (optional)
- `fields` — custom field definitions (optional)
- `triggers` — automation rules (optional)

Single highest-priority file wins — no cross-file merging.

### Workflow fields

`workflow.yaml fields:` is the single source of truth for every workflow field. The runtime hardcodes only
system fields — `id`, `title`, `description`/`body`, `createdAt`, `updatedAt`, `createdBy`, `filepath` — and
loads everything else (status, type, priority, points, tags, dependsOn, due, recurrence, assignee, plus any
project-specific fields) from the `fields:` list. Reserved system field names cannot be redeclared.

`status` and `type` are ordinary enum fields. They have no special semantics in the runtime; their meaning
comes entirely from the values you declare. The legacy top-level `statuses:` and `types:` sections are no
longer accepted and produce a clear migration error.

```yaml
fields:
  - name: status
    type: enum
    values:
      - value: backlog
        label: Backlog
        emoji: "📥"
        default: true
      - value: done
        label: Done
        emoji: "✅"
  - name: type
    type: enum
    values:
      - value: story
        label: Story
        default: true
  - name: priority
    type: integer
    default: 3
  - name: tags
    type: stringList
    default: ["idea"]
```

**Per-field properties:**

- `name` — identifier (must be a valid ruki identifier; not a reserved system field name)
- `type` — one of: `text`, `integer`, `boolean`, `date`, `datetime`, `enum`, `stringList`, `tikiIdList`, `recurrence`
- `default` — creation default for non-enum fields
- `values` — required for enum fields; lists the allowed values

**Enum value properties:**

- `value` — canonical key
- `label` — display name (defaults to `value`)
- `emoji` — unicode emoji shown in UI
- `default: true` — at most one value per enum may carry this flag; that value is the creation default

The legacy `active:` and `done:` flags on enum values are rejected. If you want a visual cue for terminal
states, set the `emoji:` field (e.g., ✅ on the "done" value).

### Views

```yaml
views:
  actions: [...]    # global actions, available in all tiki plugins
  plugins: [...]    # plugin/view definitions
```

Global actions are appended to each tiki plugin's action list.
Per-plugin actions with the same key take precedence over globals.

### Actions

Defined in `views.actions` (global) and `plugins[].actions` (per-plugin).

```yaml
actions:
  - key: "a"
    label: "Assign to me"
    action: update where id = id() set assignee=user()
    hot: true
    input: string
```

- `key` — single printable character
- `label` — description shown in header and action palette
- `action` — [ruki](ruki/index.md) statement (`update`, `create`, `delete`, or `select`)
- `hot` — show in header (optional, default true). All actions appear in the palette regardless
- `input` — scalar type for user prompt: `string`, `int`, `bool`, `date`, `timestamp`, `duration` (optional)

Validation:
- max 10 actions per section
- `input` requires `input()` in the ruki statement and vice versa
- `input()` may appear once per action

See [Input-backed actions](customization/customization.md#input-backed-actions).

### Plugins

```yaml
plugins:
  - name: Kanban
    description: "Board view"
    key: "F1"
    default: true
    view: compact
    type: tiki
    foreground: "#ff0000"
    background: "#000000"
    lanes: [...]
    actions: [...]
```

- `name` — display name (required)
- `description` — shown in header when active
- `key` — activation hotkey (single character or function key)
- `default` — open on startup (only one plugin)
- `view` — `compact` or `expanded` (tiki plugins only, default compact)
- `type` — `tiki` (default) or `doki`
- `foreground`, `background` — hex color overrides
- `lanes` — lane definitions (tiki plugins)
- `actions` — per-plugin shortcut actions

Doki plugins use `fetcher` (`file` or `internal`), `url`, and `text` instead of lanes.

### Lanes

```yaml
lanes:
  - name: Ready
    columns: 1
    width: 25
    filter: select where status = "ready" order by priority
    action: update where id = id() set status="ready"
```

- `name` — lane title (required)
- `columns` — number of columns (default 1)
- `width` — percentage of horizontal space (0 = equal share)
- `filter` — ruki `select` statement (required)
- `action` — ruki `update` statement for moves into this lane

Max 10 lanes per plugin. Filter must be `SELECT`, action must be `UPDATE`.

### Fields

Custom field definitions for task records. See [Custom fields](customization/custom-fields.md).

### Triggers

Automation rules using ruki DSL. See [Triggers](ruki/triggers.md).
