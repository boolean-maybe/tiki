# Workflow Format Versions

This document tracks the evolution of the `workflow.yaml` schema. Each section describes one format version:
what it introduced and what changed from the previous version. For usage details see
[Customization](customization/customization.md). For file locations and precedence see [Configuration](config.md).

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
- built-in attributes: `id` (task selected), `ai` (AI agent configured), `view:<view-id>` (active view)
- `id` is auto-inferred when the action uses `id()` — explicit `require: ["id"]` is allowed but redundant
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
- `default` — status for new tasks (exactly one required)
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
