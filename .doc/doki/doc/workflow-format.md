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

statuses:
  - key: backlog
    label: Backlog
    default: true

types:
  - key: story
    label: Story

fields:
  - name: severity
    type: enum
    values: [low, medium, high]

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
    mode: compact              # renamed from old `view: compact|expanded`
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
```

### View `kind:` replaces `type:`

| kind      | purpose                                                                  | required fields           | status                                |
|-----------|--------------------------------------------------------------------------|---------------------------|---------------------------------------|
| `board`   | kanban-style lanes with per-lane filters and move actions                | `lanes`                   | shipped                               |
| `list`    | single-column list view                                                  | `lanes` (typically one)   | shipped                               |
| `wiki`    | markdown viewer bound to a document by relative path                     | `path:`                   | shipped (path only; see below)        |
| `detail`  | markdown viewer of the currently-selected document                       | —                         | shipped                               |
| `search`  | the global search view                                                   | —                         | **not implemented** — parser rejects  |
| `timeline`| future phase                                                             | —                         | reserved — parser rejects             |

`wiki` views accept `path:` today. The alternative `document: <ID>` form (binding a wiki view to a task by id
rather than by relative path) is **not implemented**: the parser rejects any view that sets `document:`. A clean
ID-binding story needs a richer document-index contract than the current `store.PathForID` exposes, so it is
deferred as a future enhancement without a scheduled phase.

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
| `view: compact`/`view: expanded` on a view     | `mode: compact`/`mode: expanded`                      |
| `sort: <field>` on a view                      | `order by <field>` inside each lane's `filter:`       |

### Rejection-error table

Users upgrading will see one of these messages; each names the legacy field and points at its replacement:

- `"type" is no longer supported — use kind: board instead`
- `"type" is no longer supported — use kind: wiki instead`
- `"view:" as a display mode is no longer supported — use mode: compact or mode: expanded on a board/list view`
- `"fetcher" is no longer supported — use document: or path: on a kind: wiki view`
- `"text" is no longer supported — use document: or path: on a kind: wiki view`
- `"url" is no longer supported — use document: or path: on a kind: wiki view`
- `"sort" is no longer supported — use order by inside a lane's filter: ruki statement`
- `views: must be a top-level list — the views.plugins wrapper is no longer supported`
- `unknown view kind "X" — expected board, list, wiki, or detail`
- `kind: timeline is reserved but not yet implemented`
- `kind: search is reserved but not yet implemented` (the built-in global search UI is not plugin-instantiable today)
- `document: (ID-based resolution) is not yet implemented — use path: with a relative filepath`

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

### Statuses

```yaml
statuses:
  - key: inProgress
    label: "In Progress"
    emoji: "⚙️"
    active: true
    default: false
    done: false
```

- `key` — canonical camelCase identifier
- `label` — display name (defaults to key)
- `emoji` — unicode emoji
- `active` — marks "in-progress" work (optional, default false)
- `default` — status for new tasks (optional; at most one). When no status is marked `default: true`,
  piped input and ruki `create` produce **plain documents** (only `id:` and `title:` in the
  frontmatter) instead of workflow tasks — use this for notes-only projects that should not
  auto-capture as board items.
- `done` — marks completion (exactly one required)

Keys must be canonical camelCase. See
[Custom statuses and types](customization/custom-status-type.md) for normalization and validation rules.

### Types

```yaml
types:
  - key: bug
    label: Bug
    emoji: "💥"
```

- `key` — canonical lowercase identifier (no separators)
- `label` — display name (defaults to key)
- `emoji` — unicode emoji

At least one type required. Mark one type `default: true` to make it the creation default;
if none is marked, the first type wins. See [Custom statuses and types](customization/custom-status-type.md).

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
