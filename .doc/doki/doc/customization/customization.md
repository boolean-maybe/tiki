# Customization

tiki is highly customizable. `workflow.yaml` lets you define your workflow statuses, types, custom fields, and
the **views** that shape how documents are displayed and how you interact with them. Statuses define the
lifecycle stages tasks move through; views decide what you see on each screen (board lanes, list filters,
wiki pages, or document detail) and which keyboard actions are available. This section covers both.

> **Version.** This reference describes the 0.6.0 `workflow.yaml` schema that ships with unified documents.
> Pre-0.6.0 `type: tiki` / `type: doki` plugins, `views: plugins:` wrappers, `fetcher:`/`url:`/`text:`, and
> `view: compact|expanded` are no longer accepted. See [Workflow format versions](../workflow-format.md)
> for the migration map and rejection-error table.

## Description

An optional top-level `description:` field in `workflow.yaml` describes what
the workflow is for. It supports multi-line text via YAML's block scalar (`|`)
and is used by `tiki workflow describe <name>` to preview a workflow before
installing it.

```yaml
description: |
  Release workflow. Coordinate feature rollout through
  Planned → Building → Staging → Canary → Released.
```

## Statuses

Workflow statuses are defined in `workflow.yaml` under the `statuses:` key. Every project must define
its statuses here — there is no hardcoded fallback. See
[Custom statuses and types](custom-status-type.md). The default `workflow.yaml` ships with:

```yaml
statuses:
  - key: backlog
    label: Backlog
    emoji: "📥"
    default: true
  - key: ready
    label: Ready
    emoji: "📋"
    active: true
  - key: inProgress
    label: "In Progress"
    emoji: "⚙️"
    active: true
  - key: review
    label: Review
    emoji: "👀"
    active: true
  - key: done
    label: Done
    emoji: "✅"
    done: true
```

Each status has:
- `key` — canonical camelCase identifier. Used in filters, actions, and frontmatter.
- `label` — display name shown in the UI (defaults to key when omitted)
- `emoji` — emoji shown alongside the label
- `active` — marks the status as "active work" (used for activity tracking)
- `default` — the status assigned to new workflow documents (exactly one required)
- `done` — marks the status as "completed" (exactly one required)

You can customize these to match your team's workflow. All filters and actions in view definitions
(see below) must reference valid status keys.

## Types

Task types are defined in `workflow.yaml` under the `types:` key. A missing `types:` section is an
error. See [Custom statuses and types](custom-status-type.md) for the full validation rules. The
default `workflow.yaml` ships with:

```yaml
types:
  - key: story
    label: Story
    emoji: "🌀"
  - key: bug
    label: Bug
    emoji: "💥"
  - key: spike
    label: Spike
    emoji: "🔍"
  - key: epic
    label: Epic
    emoji: "🗂️"
```

Each type has:
- `key` — canonical lowercase identifier. Used in filters, actions, and frontmatter.
- `label` — display name shown in the UI (defaults to key when omitted)
- `emoji` — emoji shown alongside the label

Mark one type `default: true` to use it as the creation default for new workflow documents.
If no type is marked, the first configured type wins.

## Task Creation Defaults

When you create a new workflow document, field defaults come from two sources:

**Built-in defaults** (hardcoded, not configurable):
- `priority` = 3
- `points` = 1
- `tags` = `["idea"]`

**Workflow defaults** (configurable in `workflow.yaml`):
- `status` — the status marked `default: true`
- `type` — the type marked `default: true`, or the first type if none is marked
- custom fields — any field with a `default:` value (see [Custom fields](custom-fields.md))

```yaml
types:
  - key: bug
    label: Bug
    emoji: "🐛"
    default: true

fields:
  - name: severity
    type: enum
    values: [critical, high, medium, low]
    default: medium
```

## Views

Every screen in the tiki TUI is a **view**. Views are defined at the top level of `workflow.yaml` under a
`views:` list. Each view has a `kind:` that decides what it does:

| kind | purpose | required fields |
|---|---|---|
| `board` | kanban-style lanes with per-lane filters and move actions | `lanes:` |
| `list` | single-column list view | `lanes:` (typically one) |
| `wiki` | markdown viewer bound to a document by relative path | `path:` |
| `detail` | markdown viewer for the currently-selected document | — |

Views are matched to keyboard shortcuts via `key:`, and at most one view may be marked `default: true` so
the TUI knows which screen to open on startup.

Here is a simple single-lane board called Backlog:

```yaml
views:
  - name: Backlog
    label: Backlog
    description: "Tasks waiting to be picked up, sorted by priority"
    kind: board
    key: "F3"
    lanes:
      - name: Backlog
        columns: 4
        filter: select where status = "backlog" and type != "epic" order by priority, id
    actions:
      - key: "b"
        label: "Add to board"
        action: update where id = id() set status="ready"
```

This shows every document in `status = "backlog"`, sorts by priority and then id, and arranges them visually
in 4 columns inside a single lane. The `actions:` list defines a keyboard shortcut `b` that moves the
selected document to the board by setting its status to `ready`.

A documentation view is simply a `kind: wiki` entry pointing at a relative Markdown file under `.doc/`:

```yaml
views:
  - name: Docs
    label: Docs
    description: "Project notes and documentation files"
    kind: wiki
    path: "index.md"
    key: "F2"
```

Path resolution is relative to `.doc/` — this example loads `.doc/index.md`. You can nest any depth:
`path: "architecture/index.md"` loads `.doc/architecture/index.md`.

### Multi-lane board

A board view with multiple lanes lets you move documents between lanes with `Shift-←`/`Shift-→`, just like
the main kanban. Each lane declares a `filter:` (a ruki `select` statement) and optionally an `action:` (a
ruki `update` statement that fires when a document is moved *into* the lane):

```yaml
views:
  - name: Custom
    label: Custom
    kind: board
    key: "F4"
    lanes:
      - name: Ready
        columns: 1
        width: 20
        filter: select where status = "ready" order by priority, title
        action: update where id = id() set status="ready"
      - name: In Progress
        columns: 1
        width: 30
        filter: select where status = "inProgress" order by priority, title
        action: update where id = id() set status="inProgress"
      - name: Review
        columns: 1
        width: 30
        filter: select where status = "review" order by priority, title
        action: update where id = id() set status="review"
      - name: Done
        columns: 1
        width: 20
        filter: select where status = "done" order by priority, title
        action: update where id = id() set status="done"
```

### Compact vs expanded

Board and list views support a `mode:` field that switches between compact and expanded card layouts:

```yaml
views:
  - name: Kanban
    kind: board
    mode: compact          # or: mode: expanded
    lanes:
      - name: Backlog
        filter: select where status = "backlog"
```

This replaces the pre-0.6.0 `view: compact`/`view: expanded` field, which is no longer accepted.

### Lane width

Each lane can optionally specify a `width` as a percentage (1-100) to control how much horizontal
space it occupies. Widths are relative proportions — they don't need to sum to 100. If width is
omitted, the lane gets an equal share of the remaining space.

```yaml
lanes:
  - name: Sidebar
    width: 25
  - name: Main
    width: 50
  - name: Details
    width: 25
```

If no lanes specify width, all lanes are equally sized (the default behavior).

### Global actions

You can define actions at the top level of `workflow.yaml` under `actions:`. Top-level actions are **global**
— they are available from every view:

```yaml
actions:
  - key: "a"
    label: "Assign to me"
    kind: ruki
    action: update where id = id() set assignee=user()
  - key: F11
    kind: view
    label: "Open board"
    view: kanban          # name of a view declared below

views:
  - name: kanban
    kind: board
    ...
  - name: backlog
    kind: board
    ...
```

Two action kinds are supported at the top level:

- **`kind: ruki`** — executes a ruki statement (`update`, `select`, `delete`, …). This is the classic
  keyboard-shortcut behavior. The `action:` field carries the statement. When invoked from a wiki or
  detail view that received a selection via navigation, that selection threads into the ruki statement
  so `id()` resolves against it.
- **`kind: view`** — navigates to another view by name. The `view:` field names the target view. If the
  target is a `kind: detail` view (or otherwise requires a selection), the current selection is carried
  through and `require: ["selection:one"]` is honored.

When `kind:` is omitted, the parser infers it: `action:` set ⇒ `kind: ruki`, `view:` set ⇒ `kind: view`.

Global actions appear in the header alongside per-view actions. If a per-view action uses the same key as
a global action, the per-view action takes precedence for that view. There is no cross-file merging — the
single active workflow file wins.

### Per-view actions

In addition to lane actions (which fire when moving documents between lanes), each view can declare a
per-view `actions:` list. These shortcuts apply to the currently selected document and are displayed in the
header when the view is active.

```yaml
actions:
  - key: "b"
    label: "Add to board"
    action: update where id = id() set status="ready"
  - key: "a"
    label: "Assign to me"
    action: update where id = id() set assignee=user()
```

Each action has:
- `key` - a single printable character used as the keyboard shortcut
- `label` - description shown in the header and action palette
- `action` - a `ruki` statement (`update`, `create`, `delete`, or `select`)
- `hot` - (optional) controls header visibility. `hot: true` shows the action in the header,
  `hot: false` hides it. When absent, actions default to visible in the header. This does not affect
  the action palette — all actions are always discoverable via `?` regardless of the `hot` setting
- `input` - (optional) declares that the action prompts for user input before executing. The value is
  the scalar type of the input: `string`, `int`, `bool`, `date`, `timestamp`, or `duration`. The
  action's `ruki` statement must use `input()` to reference the value
- `require` - (optional) a list of context attributes the action needs to be enabled. When
  requirements are not met, the action is visible but greyed out in the header and palette, and its
  hotkey does nothing. See [Action requirements](#action-requirements) below

Example — keeping a verbose action out of the header but still accessible from the palette:

```yaml
actions:
  - key: "x"
    label: "Archive and notify"
    action: update where id = id() set status="done"
    hot: false
```

When the shortcut key is pressed, the action is applied to the currently selected document.
For example, pressing `b` in the Backlog view changes the selected document's status to `ready`,
effectively moving it to the board.

`select` actions execute for side effects only — the output is ignored. They don't require a selected document.

### Input-backed actions

Actions with `input:` prompt the user for a value before executing. When the action key is pressed,
a modal input box opens with the action label as the prompt. The user types a value and presses
Enter to execute, or Esc to cancel.

```yaml
actions:
  - key: "A"
    label: "Assign to..."
    action: update where id = id() set assignee = input()
    input: string
  - key: "t"
    label: "Add tag"
    action: update where id = id() set tags = tags + input()
    input: string
  - key: "T"
    label: "Remove tag"
    action: update where id = id() set tags = tags - input()
    input: string
  - key: "p"
    label: "Set points"
    action: update where id = id() set points = input()
    input: int
  - key: "D"
    label: "Set due date"
    action: update where id = id() set due = input()
    input: date
```

Supported `input:` types: `string`, `int`, `bool`, `date` (YYYY-MM-DD), `timestamp` (RFC3339 or
YYYY-MM-DD), `duration` (e.g. `2day`, `1week`).

Validation rules:
- An action with `input:` must use `input()` in its `ruki` statement
- An action using `input()` must declare `input:` — otherwise the workflow fails to load
- `input()` may only appear once per action

### Choose-backed actions

Actions using `choose()` open an interactive Quick Select document picker. The subquery inside
`choose()` determines which documents appear as candidates.

```yaml
actions:
  - key: "e"
    label: "Link to epic"
    action: update where id = choose(select where type = "epic") set dependsOn = dependsOn + id()
  - key: "l"
    label: "Add to epic"
    action: update where id = id() set dependsOn = dependsOn + choose(select where type != "epic")
```

When the shortcut key is pressed, the Quick Select modal opens with the filtered candidate list.
The user fuzzy-filters by typing, navigates with arrow keys, and confirms with Enter. Esc cancels
the operation.

Validation rules:
- `choose()` requires exactly one argument: a `select` subquery
- `choose()` may only appear once per action
- `choose()` and `input()` are mutually exclusive within a single action

### Action requirements

Actions can declare context requirements that control when they are enabled. Requirements are
evaluated against the current application. When any requirement is unmet, the action is disabled.


```yaml
actions:
  - key: "c"
    label: "Chat about task"
    action: select where id = id() | run("claude -p 'Discuss: $1'")
    require: ["ai", "id"]
```

This action requires both `ai` (an AI agent configured in `config.yaml`) and `id` (a task selected in the current view).

#### Built-in context attributes

| Attribute | Set when |
|-----------|----------|
| `id` | Exactly one task is selected — legacy alias for `selection:one` |
| `selection:one` | Exactly one task is selected |
| `selection:any` | One or more tasks are selected |
| `selection:many` | Two or more tasks are selected |
| `ai` | `ai.agent` is configured in `config.yaml` |
| `view:<view-id>` | Identifies the currently active view (e.g. `view:plugin:Kanban`) |

`id` and `selection:one` are equivalent; both require exactly one selected task. Prefer whichever reads better in
context — `id` is shorter, `selection:one` is symmetric with the other cardinality tokens.

#### Auto-inference

Tiki infers selection requirements from the ruki statement so authors rarely need to declare them explicitly:

- Using `id()` auto-infers `id` (equivalent to `selection:one`).
- Using `ids()` auto-infers `selection:any` — the action requires at least one selection but accepts any
  cardinality above that. Override with an explicit `require:` entry (e.g. `["selection:many"]`) when you want to
  constrain further.
- Using `selected_count()` does **not** auto-infer anything. The builtin exists so ruki can branch on cardinality
  (including the zero case), and auto-inferring `selection:any` would make the zero branch unreachable. Authors
  who want gating should add an explicit `require:` entry.

Explicitly listing an auto-inferred requirement is allowed but redundant.

#### Multi-selection actions

Use `ids()` in the ruki statement to operate on every selected task:

```yaml
actions:
  - key: "D"
    label: "Mark selected done"
    action: update where id in ids() set status = "done"
```

This action inherits `selection:any`, so it is enabled as soon as at least one task is selected. To require two or
more selected tasks (e.g. a merge operation), add `require: ["selection:many"]` explicitly.

#### Bulk actions

Mutating actions (`update`, `delete`) that do *not* use `id()` or `ids()` are bulk actions — they operate on all
matching tasks, not just the selected ones. Bulk actions remain enabled even when nothing is selected:

```yaml
actions:
  - key: "X"
    label: "Delete all done"
    action: delete where status = "done"
```

This action has no selection requirement (neither explicit nor inferred) so it stays enabled regardless of
selection state.

#### Negated requirements

Prefix a requirement with `!` to require that the attribute is *absent*. In YAML, negated requirements must be quoted:

```yaml
actions:
  - key: "K"
    label: "Go to Kanban"
    action: select where status = "ready"
    require: ["!view:plugin:Kanban"]
```

This action is disabled when the user is already on the Kanban view — the `view:plugin:Kanban`
attribute would be present, failing the `!view:plugin:Kanban` check.

### ruki expressions

View filters, lane actions, and per-view/global actions all use the [ruki](../ruki/index.md)
language. Filters use `select` statements. Actions support `update`, `create`, `delete`, and
`select` statements (`select` for side effects only, output ignored).

#### Filter (select)

The `filter` field uses a `ruki` `select` statement to determine which documents appear in a lane.
Sorting is part of the select — use `order by` to control display order.

```sql
-- basic filter with sort
select where status = "backlog" and type != "epic" order by priority, id

-- recent items, most recent first
select where now() - updatedAt < 24hour order by updatedAt desc

-- multiple conditions
select where type = "epic" and status = "backlog" and priority > 1 order by priority, points desc

-- assigned to me
select where assignee = user() order by priority
```

#### Action (update)

The `action` field uses a `ruki` `update` statement. In view context, `id()` refers to the currently selected document.

```sql
-- set status on move
update where id = id() set status="ready"

-- set multiple fields
update where id = id() set status="backlog" priority=2

-- assign to current user
update where id = id() set assignee=user()
```

#### Supported fields

- `id` - document identifier (bare 6-char uppercase, e.g. `"M7N2XK"`)
- `title` - task title text
- `type` - task type (must match a key defined in `workflow.yaml` types)
- `status` - workflow status (must match a key defined in `workflow.yaml` statuses)
- `assignee` - assigned user
- `priority` - numeric priority value (1-5)
- `points` - story points estimate
- `tags` - list of tags
- `dependsOn` - list of document ids this document depends on
- `due` - due date (YYYY-MM-DD format)
- `recurrence` - recurrence pattern (cron format)
- `createdAt` - creation timestamp
- `updatedAt` - last update timestamp

#### Conditions

- **Comparison**: `=`, `!=`, `>`, `>=`, `<`, `<=`
- **Logical**: `and`, `or`, `not` (precedence: not > and > or)
- **Membership**: `"value" in field`, `status not in ["done", "cancelled"]`
- **Emptiness**: `assignee is empty`, `tags is not empty`
- **Quantifiers**: `dependsOn any status != "done"`, `dependsOn all status = "done"`
- **Grouping**: parentheses `()` to control evaluation order

#### Literals and built-ins

- Strings: double-quoted (`"ready"`, `"alex"`)
- Integers: `1`, `5`
- Dates: `2026-03-25`
- Durations: `2hour`, `14day`, `3week`, `1month`
- Lists: `["bug", "frontend"]`
- `user()` — current `tiki` identity (configured `identity.name` or `identity.email` → git user → OS user)
- `now()` — current timestamp
- `id()` — currently selected document (in view context)
- `input()` — user-supplied value (in actions with `input:` declaration)
- `choose(select where ...)` — interactively pick a task from Quick Select
- `count(select where ...)` — count matching documents

For the full language reference, see the [ruki documentation](../ruki/index.md).