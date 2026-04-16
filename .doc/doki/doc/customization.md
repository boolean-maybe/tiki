# Customization

tiki is highly customizable. `workflow.yaml` lets you define your workflow statuses and configure views (plugins) for 
how tikis are displayed and organized. Statuses define the lifecycle stages your tasks move through, 
while plugins control what you see and how you interact with your work. This section covers both.

## Statuses

Workflow statuses are defined in `workflow.yaml` under the `statuses:` key. Every tiki project must define 
its statuses here — there is no hardcoded fallback. See [Custom statuses and types](custom-status-type.md). The default `workflow.yaml` ships with:

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
- `default` — the status assigned to new tikis (exactly one required)
- `done` — marks the status as "completed" (exactly one required)

You can customize these to match your team's workflow. All filters and actions in view definitions (see below) must reference valid status keys.

## Types

Task types are defined in `workflow.yaml` under the `types:` key. If omitted, built-in defaults are used.
See [Custom statuses and types](custom-status-type.md) for the full validation and inheritance rules. The default `workflow.yaml` ships with:

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

The first configured type is used as the default for new tikis.

## Task Template

When you create a new tiki — whether in the TUI or command line — field defaults come from a template file. 
Place `new.md` in your config directory to override the built-in defaults
The file uses YAML frontmatter for field defaults

### Built-in default

```markdown
---
title:
points: 1
priority: 3
tags:
    - idea
---
```

Type and status are omitted but can be added, otherwise they default to the first configured type and the status marked `default: true`.

## Plugins

tiki TUI app is much like a lego - everything is a customizable view. Here is, for example,
how Backlog is defined:

```yaml
views:
  plugins:
    - name: Backlog
      description: "Tasks waiting to be picked up, sorted by priority"
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

that translates to - show all tikis in the status `backlog`, sort by priority and then by ID arranged visually in 4 columns in a single lane.
The `actions` section defines a keyboard shortcut `b` that moves the selected tiki to the board by setting its status to `ready`
You define the name, description, hotkey, and `ruki` expressions for filtering and actions. The `description` is displayed in the header when the view is active. Save this into a `workflow.yaml` file in the config directory

Likewise the documentation is just a plugin:

```yaml
views:
  plugins:
    - name: Docs
      description: "Project notes and documentation files"
      type: doki
      fetcher: file
      url: "index.md"
      key: "F2"
```

that translates to - show `index.md` file located under `.doc/doki`
installed in the same way

### Multi-lane plugin

Backlog is a pretty simple plugin in that it displays all tikis in a single lane. Multi-lane tiki plugins offer functionality
similar to that of the board. You can define multiple lanes per view and move tikis around with Shift-Left/Shift-Right
much like in the board. You can create a multi-lane plugin by defining multiple lanes in its definition and assigning
actions to each lane. An action defines what happens when you move a tiki into the lane. Here is a multi-lane plugin
definition that roughly mimics the board:

```yaml
name: Custom
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

### Lane width

Each lane can optionally specify a `width` as a percentage (1-100) to control how much horizontal space it occupies. Widths are relative proportions — they don't need to sum to 100. If width is omitted, the lane gets an equal share of the remaining space.

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

### Global plugin actions

You can define actions under `views.actions` that are available in **all** tiki plugin views. This avoids repeating common shortcuts in every plugin definition.

```yaml
views:
  actions:
    - key: "a"
      label: "Assign to me"
      action: update where id = id() set assignee=user()
  plugins:
    - name: Kanban
      ...
    - name: Backlog
      ...
```

Global actions appear in the header alongside per-plugin actions. If a per-plugin action uses the same key as a global action, the per-plugin action takes precedence for that view.

When multiple workflow files define `views.actions`, they merge by key across files — later files override same-keyed globals from earlier files.

### Per-plugin actions

In addition to lane actions that trigger when moving tikis between lanes, you can define plugin-level actions
that apply to the currently selected tiki via a keyboard shortcut. These shortcuts are displayed in the header when the plugin is active.

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
- `label` - description shown in the header
- `action` - a `ruki` statement (`update`, `create`, `delete`, or `select`)

When the shortcut key is pressed, the action is applied to the currently selected tiki.
For example, pressing `b` in the Backlog plugin changes the selected tiki's status to `ready`, effectively moving it to the board.

`select` actions execute for side-effects only — the output is ignored. They don't require a selected tiki.

### ruki expressions

Plugin filters, lane actions, and plugin actions all use the [ruki](ruki/index.md) language. Filters use `select` statements. Actions support `update`, `create`, `delete`, and `select` statements (`select` for side-effects only, output ignored).

#### Filter (select)

The `filter` field uses a `ruki` `select` statement to determine which tikis appear in a lane. Sorting is part of the select — use `order by` to control display order.

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

The `action` field uses a `ruki` `update` statement. In plugin context, `id()` refers to the currently selected tiki.

```sql
-- set status on move
update where id = id() set status="ready"

-- set multiple fields
update where id = id() set status="backlog" priority=2

-- assign to current user
update where id = id() set assignee=user()
```

#### Supported fields

- `id` - task identifier (e.g., "TIKI-M7N2XK")
- `title` - task title text
- `type` - task type (must match a key defined in `workflow.yaml` types)
- `status` - workflow status (must match a key defined in `workflow.yaml` statuses)
- `assignee` - assigned user
- `priority` - numeric priority value (1-5)
- `points` - story points estimate
- `tags` - list of tags
- `dependsOn` - list of dependency tiki IDs
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
- `user()` — current user
- `now()` — current timestamp
- `id()` — currently selected tiki (in plugin context)
- `count(select where ...)` — count matching tikis

For the full language reference, see the [ruki documentation](ruki/index.md).