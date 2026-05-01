# Customization Tutorial

This guide walks you through customizing tiki step by step. You already have a
working workflow out of the box — we will start from there and gradually make it
your own. Each section builds on the previous one, and at any point along the way
your configuration will be valid and ready to use.

By the end you will have a fully customized workflow with new statuses, types,
views, keyboard shortcuts, custom fields, automation rules, documentation pages,
and a visual theme — all tailored to the way your team works.

## Before you start

Everything in this guide goes into your project's `.doc/` directory:

- `.doc/workflow.yaml` — your workflow customization
- `.doc/config.yaml` — appearance and display settings

tiki looks for configuration in three places:

1. **User config** (`~/.config/tiki/`)
2. **The project directory** (`.doc/`) — can be saved to git
3. **The current directory** (`./`) — local overrides

The more specific location wins. In this tutorial we work with `.doc/` so
the customizations are project-level and travel with your repo.

> **YAML spacing matters.** Use two spaces for each level of indentation, never
> tabs. If something does not work, check your spacing first — it is almost
> always the culprit.

---

## 1. Meet your default workflow

Before changing anything, let us look at what ships with tiki. When you
run `tiki init`, you get a workflow with:

**Five statuses** — the stages a task moves through:

| Status | Meaning |
|--------|---------|
| 📥 Backlog | Where new tasks land |
| 📋 Ready | Picked up and ready to start |
| ⚙️ In Progress | Someone is working on it |
| 👀 Review | Waiting for a teammate to check |
| ✅ Done | Finished |

**Four task types** — what kind of work it is:

| Type | Meaning |
|------|---------|
| 🌀 Story | A feature or user-facing change |
| 💥 Bug | Something broken that needs fixing |
| 🔍 Spike | Research or investigation |
| 🗂️ Epic | A big-picture goal that groups smaller tasks |

**Five views** — different screens you switch between with hotkeys:

| View | Key | What it shows |
|------|-----|---------------|
| Kanban | F1 | The main board with Ready, In Progress, Review, Done columns |
| Docs | F2 | Project documentation files |
| Backlog | F3 | Tasks waiting to be picked up |
| Roadmap | F4 | Epics organized by Now, Next, Later |
| Recent | Ctrl-R | Tasks changed in the last 24 hours |

Plus a set of keyboard shortcuts (actions) and automation rules (triggers) that
we will explore in later sections.

---

## 2. Adding a status

Let us start with a small change: adding a "Blocked" status for tasks that are
stuck waiting on something.

Statuses are defined in `workflow.yaml` under the `statuses:` key. There is one
important thing to know: **when you define statuses, you replace the entire
list**. That means you need to include all the statuses you want — the originals
plus your new ones.

Create `.doc/workflow.yaml` with this content:

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
  - key: blocked
    label: Blocked
    emoji: "🚧"
  - key: review
    label: Review
    emoji: "👀"
    active: true
  - key: done
    label: Done
    emoji: "✅"
    done: true
```

This is the same as the default, with one new entry: `blocked`.

Let us look at the flags on each status:

- **`default: true`** — where new tasks start. Exactly one status must have this.
  Here that is Backlog.
- **`done: true`** — the finish line. Exactly one status must have this. Here
  that is Done.
- **`active: true`** — means work is happening. You can mark as many as you like.
  Blocked is intentionally *not* active — the task is stuck, not being worked on.

Each status also has a **key** and a **label**. The key is the permanent
identifier — it is what gets stored in your task files and used in filters. The
label is what you see on screen. They can be different, as we will see in a
moment.

> **Keep your keys stable.** Once tasks exist with a particular status key,
> changing or removing that key will cause problems. It is always safe to add
> new statuses or change a label.

---

## 3. Adding a type

Next, let us add a "Chore" type for housekeeping tasks — things like updating
dependencies, cleaning up old branches, or writing documentation.

Types work the same way as statuses: **when you define types, you replace the
entire list**.

Add this to your `.doc/workflow.yaml`, right after the statuses section:

```yaml
types:
  - key: story
    label: Story
    emoji: "🌀"
  - key: bug
    label: Bug
    emoji: "💥"
  - key: chore
    label: Chore
    emoji: "🔧"
  - key: spike
    label: Spike
    emoji: "🔍"
  - key: epic
    label: Epic
    emoji: "🗂️"
```

This keeps all four original types and adds `chore` in the middle.

One thing to note: the **first type in the list becomes the default** for new
tasks. Here that is `story`. If you wanted chores to be the default, you would
move it to the top of the list.

> **Never remove a type that existing tasks use.** If you have tasks saved as
> "spike" and you remove that type from the list, those tasks will stop loading.
> It is always safe to add new types.

---

## 4. Customizing the Kanban view

Now let us customize the main board. The default Kanban has four lanes: Ready,
In Progress, Review, and Done. We want to:

1. Add a **Blocked** lane for our new status
2. Rename "Review" to **"Testing"** on screen (while keeping the key `review`
   so existing tasks stay valid)

First, update the `statuses:` section we wrote earlier — change the label on
`review`:

```yaml
  - key: review
    label: Testing
    emoji: "🧪"
    active: true
```

Notice the key is still `review`. Your existing tasks that are in review will
keep working perfectly — they just show up under the name "Testing" now. We also
swapped the emoji to a test tube to match.

Now for the Kanban view itself. Views live at the top level of `workflow.yaml`
under `views:`. Every view declares a `kind:` (here, `board`) that tells tiki
what shape to draw. Here is our customized Kanban:

```yaml
views:
  - name: Kanban
    label: Kanban
    description: "Your team's sprint board"
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Ready
        filter: select where status = "ready" and type != "epic" order by priority, createdAt
        action: update where id = id() set status="ready"
      - name: In Progress
        filter: select where status = "inProgress" and type != "epic" order by priority, createdAt
        action: update where id = id() set status="inProgress"
      - name: Blocked
        width: 15
        filter: select where status = "blocked" and type != "epic" order by priority, createdAt
        action: update where id = id() set status="blocked"
      - name: Testing
        filter: select where status = "review" and type != "epic" order by priority, createdAt
        action: update where id = id() set status="review"
      - name: Done
        filter: select where status = "done" and type != "epic" order by priority, createdAt
        action: update where id = id() set status="done"
```

There is a lot here, so let us unpack it piece by piece.

**What is a lane?** A lane is a column on screen. Each lane shows a filtered
list of tasks. When you move a task into a lane (with Shift-Left or
Shift-Right), the lane's action runs automatically.

**What does `filter` do?** The filter tells tiki which tasks belong in this lane.
It is written in a small built-in language called [ruki](../ruki/index.md) — but
do not let that intimidate you, it reads almost like English:

```
select where status = "ready" and type != "epic" order by priority, createdAt
```

This means: *show me tasks where the status is "ready" and the type is not
"epic", sorted by priority first, then by creation date.*

**What does `action` do?** The action is what happens when you move a task into
this lane:

```
update where id = id() set status="ready"
```

This means: *find the task I have selected (`id()` means "whichever one is
highlighted") and set its status to "ready".*

**What about `width`?** Each lane can have a width as a percentage. Here we gave
Blocked a width of 15 — it is a narrower column since hopefully not many tasks
are stuck. Lanes without a width share the remaining space equally.

> **When you override a view's lanes, you replace all of them.** tiki merges
> views by name — so writing a view called "Kanban" overrides the default
> Kanban. But the `lanes:` list inside it is replaced wholesale, not merged.
> Make sure to include every lane you want.

---

## 5. Adding a new view

The Kanban board shows everyone's work. But sometimes you just want to see
your own tasks. Let us add a "My Tasks" view.

Add this as another entry under the top-level `views:` list (after the Kanban
definition):

```yaml
  - name: My Tasks
    description: "Tasks assigned to you"
    kind: board
    key: "F5"
    lanes:
      - name: My Tasks
        columns: 3
        filter: select where assignee = user() and status != "done" order by priority, createdAt
```

A few new things here:

- **`key: "F5"`** — press F5 to switch to this view. You can use function keys
  (F1-F12), Ctrl combinations (like `Ctrl-R`), or single characters.
- **`columns: 3`** — this lane uses a 3-column grid layout instead of a single
  tall list. Handy when you have many tasks and want to use the screen width.
- **`user()`** — this is a handy shortcut that means "you". It resolves to
  your configured `identity.name` (or `identity.email` if only the email is
  set) from `config.yaml` (see
  [Configuration](../config.md#identity-resolution)), falling back to your
  git user or your OS account username when `identity` is unset.

---

## 6. Actions — keyboard shortcuts

Actions are one-key shortcuts that do something to the task you have selected.
The default workflow ships with several:

| Key | What it does |
|-----|-------------|
| `a` | Assign the task to you |
| `y` | Copy the task ID to your clipboard |
| `+` | Raise the priority |
| `-` | Lower the priority |
| `u` | Flag as urgent (sets priority to 1 and adds an "urgent" tag) |
| `A` | Assign to someone else (asks you to type a name) |
| `t` | Add a tag (asks you to type one) |
| `T` | Remove a tag (asks you to type one) |

Let us look at how a couple of these work, then add our own.

**A simple action** — "Assign to me":

```yaml
- key: "a"
  label: "Assign to me"
  action: update where id = id() set assignee=user()
```

When you press `a`, tiki finds the selected task (`id()`) and sets its assignee
to you (`user()`).

**An action that asks for input** — "Assign to...":

```yaml
- key: "A"
  label: "Assign to..."
  action: update where id = id() set assignee=input()
  input: string
```

The `input: string` line tells tiki to open a text prompt when you press `A`.
Whatever you type becomes the value of `input()` in the action. Here it sets
the assignee to whatever name you type in.

**An action that works with tags** — "Flag urgent":

```yaml
- key: "u"
  label: "Flag urgent"
  action: update where id = id() set priority=1 tags=tags+["urgent"]
  hot: false
```

This one sets the priority to 1 *and* adds the tag "urgent" to the existing
list of tags. The expression `tags + ["urgent"]` means "take the current tags
and add one more."

The `hot: false` line hides this action from the header bar to keep it
uncluttered. You can still find it in the action palette (press `*` to open it).

### Adding our own action

Let us add a "Mark blocked" shortcut. This goes under the **top-level**
`actions:` section — entries there are **global actions** available from every
view:

```yaml
actions:
  - key: "k"
    kind: ruki
    label: "Mark blocked"
    action: update where id = id() set status="blocked"
```

> **Per-view vs global actions.** Global actions (top-level `actions:`) work
> everywhere. Per-view actions (under a specific view's `actions:`) only work
> in that view. If you override a view's `actions:` list, it replaces the whole
> list for that view — same as lanes.

> **Action requirements.** Actions can declare `require:` to control when
> they are enabled. For example, `require: ["ai"]` disables the action when
> no AI agent is configured. Actions using `id()` automatically require a
> task selection — you don't need to add `require: ["id"]` manually. See
> [Action requirements](customization.md#action-requirements) for the full
> reference.

---

## 7. Custom fields

So far we have been working with the fields that every task comes with: title,
status, type, priority, tags, assignee, and so on. But what if you need to
track something specific to your project?

Custom fields let you add your own data to every task. Let us add three:

- **severity** — how bad is this bug? (critical, high, medium, or low)
- **component** — which part of the system? (frontend, backend, or infra)
- **estimate** — how many hours do you think this will take?

Add a `fields:` section to your `.doc/workflow.yaml`:

```yaml
fields:
  - name: severity
    type: enum
    values: [critical, high, medium, low]
  - name: component
    type: enum
    values: [frontend, backend, infra]
  - name: estimate
    type: integer
```

The `type` tells tiki what kind of data the field holds:

| Type | What it means | Example values |
|------|--------------|----------------|
| `text` | Any text | "needs design review" |
| `integer` | A whole number | 1, 5, 40 |
| `boolean` | Yes or no | true, false |
| `datetime` | A date and time | 2026-03-25 |
| `enum` | One of a fixed set of choices | critical, high, medium, low |
| `stringList` | A list of text values | ["alice", "bob"] |
| `taskIdList` | A list of document references | ["A1B2C3"] |

For `enum` fields, you provide a `values:` list — only those choices are
accepted.

### Using custom fields in a view

Now let us build a "Bug Triage" view that groups bugs by severity:

```yaml
  - name: Bug Triage
    description: "Bugs grouped by severity"
    kind: board
    key: "F6"
    lanes:
      - name: Critical
        width: 30
        filter: select where severity = "critical" and status != "done" order by priority, createdAt
        action: update where id = id() set severity="critical"
      - name: High
        filter: select where severity = "high" and status != "done" order by priority, createdAt
        action: update where id = id() set severity="high"
      - name: Medium
        filter: select where severity = "medium" and status != "done" order by priority, createdAt
        action: update where id = id() set severity="medium"
      - name: Low
        filter: select where severity = "low" and status != "done" order by priority, createdAt
        action: update where id = id() set severity="low"
```

Custom fields work exactly like built-in fields in filters and actions. Moving
a task to the "Critical" lane sets its severity to critical.

### An action with custom field input

Let us also add a shortcut to set the estimate. Add this to your top-level
`actions:` list (global actions):

```yaml
  - key: "e"
    kind: ruki
    label: "Set estimate"
    action: update where id = id() set estimate=input()
    input: int
```

Press `e`, type a number, and the estimate is saved.

---

## 8. Triggers — automation rules

Triggers are rules that run automatically when tasks change. They can block
actions that should not happen, react to changes, or run on a schedule.

The default workflow ships with several triggers. Let us read a couple to
understand how they work, then add our own.

### Understanding a shipped trigger

Here is one from the bundled kanban workflow:

```yaml
- description: tasks must pass through review before completion
  ruki: >
    before update
      where new.status = "done" and old.status != "review"
      deny "tasks must go through review before marking done"
```

Let us break it down:

- **`before update`** — this trigger fires before a task is changed
- **`new.status = "done"`** — someone is trying to move the task to Done
- **`old.status != "review"`** — but it was not in Review (or in our case,
  Testing) beforehand
- **`deny "..."`** — block the change and show this message

The words `old` and `new` are how triggers refer to the task before and after
the change. Think of it as: "`old` is what the task looks like right now, `new`
is what someone is trying to make it look like."

Here is another one — this time a reaction that happens *after* a change:

```yaml
- description: remove deleted task from dependency lists
  ruki: >
    after delete
      update where old.id in dependsOn set dependsOn=dependsOn - [old.id]
```

This one fires after a task is deleted. It finds all other tasks that depended
on the deleted one and removes it from their dependency list. The expression
`dependsOn - [old.id]` means "take the current dependency list and remove this
one entry."

### Adding our own triggers

Remember: **the `triggers:` section replaces the entire list**. When you
customize triggers, copy over the ones you want to keep from the default, then
add yours.

Here are two new triggers we will add to the existing set:

**A work-in-progress limit** — no one can have more than 3 tasks in progress
at the same time:

```yaml
- description: per-person WIP limit of 3
  ruki: >
    before update
      where new.status = "inProgress"
        and count(select where assignee = new.assignee and status = "inProgress") > 2
      deny "you already have 3 tasks in progress — finish one first"
```

The `count(select where ...)` part counts how many tasks match a condition. If
the count is already over 2, adding another one is blocked.

**Auto-prioritize critical bugs** — when a bug is set to critical severity,
automatically raise its priority to 1:

```yaml
- description: auto-prioritize critical bugs
  ruki: >
    after update
      where new.severity = "critical"
      update where id = new.id set priority=1
```

This fires after any update. If the task's severity is now critical, it sets
the priority to 1 automatically.

> You may have noticed that these little instructions follow a pattern:
> `select where ...`, `update where ... set ...`, `before`/`after`. This is
> called **ruki** — a small language built into tiki for filters, actions, and
> triggers. You already know enough to build most things. If you want to
> explore further, see the [ruki reference](ruki/index.md).

---

## 9. Documentation views

The default workflow includes a Docs view that renders Markdown files from your
project. It uses `kind: wiki` — a view that shows a document body (and anything
you navigate to via links) instead of a list of tasks:

```yaml
  - name: Docs
    label: Docs
    description: "Project notes and documentation files"
    kind: wiki
    path: "index.md"
    key: "F2"
```

`path:` is resolved relative to `.doc/`, so this loads `.doc/index.md` as the
entry point. You navigate from there by following links — plain Markdown links
to relative paths, or `[[ID]]` wikilinks to other managed documents.

You can add more documentation views by adding more `kind: wiki` entries. For
example, an architecture notes section:

```yaml
  - name: Architecture
    description: "System architecture and design decisions"
    kind: wiki
    path: "architecture/index.md"
    key: "F7"
```

This creates a separate view at F7 pointing to a different starting file. You
would create `.doc/architecture/index.md` with links to your architecture
documents.

---

## 10. Look and feel

Appearance settings live in a separate file: `.doc/config.yaml`. This file
merges field by field — you only need to include what you want to change.

### Picking a theme

```yaml
appearance:
  theme: tokyo-night
```

Available themes:

| Dark themes | Light themes |
|------------|-------------|
| `dark` (built-in default) | `light` (built-in default) |
| `dracula` | `catppuccin-latte` |
| `tokyo-night` | `solarized-light` |
| `gruvbox-dark` | `gruvbox-light` |
| `catppuccin-mocha` | `github-light` |
| `solarized-dark` | |
| `nord` | |
| `monokai` | |
| `one-dark` | |

Set `theme: auto` (the default) and tiki will detect whether your terminal has
a dark or light background and pick accordingly.

### Other display settings

```yaml
# Show or hide the header bar at the top
header:
  visible: true

# Minimum terminal color support for smooth gradients
# 256 works for most terminals, 16777216 for full truecolor
appearance:
  gradientThreshold: 256
```

### Compact vs expanded view

Board and list views can use either a compact or expanded layout. Set the
`mode:` field on the view in `workflow.yaml`:

```yaml
  - name: Roadmap
    kind: board
    mode: expanded
```

---

## 11. The complete customized workflow

Here is everything we have built, combined into a single `.doc/workflow.yaml`.
You can use this as a starting point and adjust it to your needs.

```yaml
# --- Customized team workflow ---
# Place this file at .doc/workflow.yaml in your project

# Statuses: the lifecycle of a task
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
  - key: blocked
    label: Blocked
    emoji: "🚧"
  - key: review
    label: Testing
    emoji: "🧪"
    active: true
  - key: done
    label: Done
    emoji: "✅"
    done: true

# Types: kinds of work
types:
  - key: story
    label: Story
    emoji: "🌀"
  - key: bug
    label: Bug
    emoji: "💥"
  - key: chore
    label: Chore
    emoji: "🔧"
  - key: spike
    label: Spike
    emoji: "🔍"
  - key: epic
    label: Epic
    emoji: "🗂️"

# Custom fields: project-specific data on every task
fields:
  - name: severity
    type: enum
    values: [critical, high, medium, low]
  - name: component
    type: enum
    values: [frontend, backend, infra]
  - name: estimate
    type: integer

# Global actions: keyboard shortcuts available in every view
actions:
  - key: "k"
    kind: ruki
    label: "Mark blocked"
    action: update where id = id() set status="blocked"
  - key: "e"
    kind: ruki
    label: "Set estimate"
    action: update where id = id() set estimate=input()
    input: int

views:
  # The main board with our added Blocked and renamed Testing lanes
  - name: Kanban
    label: Kanban
    description: "Your team's sprint board"
    kind: board
    default: true
    key: "F1"
    lanes:
      - name: Ready
        filter: select where status = "ready" and type != "epic" order by priority, createdAt
        action: update where id = id() set status="ready"
      - name: In Progress
        filter: select where status = "inProgress" and type != "epic" order by priority, createdAt
        action: update where id = id() set status="inProgress"
      - name: Blocked
        width: 15
        filter: select where status = "blocked" and type != "epic" order by priority, createdAt
        action: update where id = id() set status="blocked"
      - name: Testing
        filter: select where status = "review" and type != "epic" order by priority, createdAt
        action: update where id = id() set status="review"
      - name: Done
        filter: select where status = "done" and type != "epic" order by priority, createdAt
        action: update where id = id() set status="done"

  # Your personal task list
  - name: My Tasks
    description: "Tasks assigned to you"
    kind: board
    key: "F5"
    lanes:
      - name: My Tasks
        columns: 3
        filter: select where assignee = user() and status != "done" order by priority, createdAt

  # Bugs grouped by severity
  - name: Bug Triage
    description: "Bugs grouped by severity"
    kind: board
    key: "F6"
    lanes:
      - name: Critical
        width: 30
        filter: select where severity = "critical" and status != "done" order by priority, createdAt
        action: update where id = id() set severity="critical"
      - name: High
        filter: select where severity = "high" and status != "done" order by priority, createdAt
        action: update where id = id() set severity="high"
      - name: Medium
        filter: select where severity = "medium" and status != "done" order by priority, createdAt
        action: update where id = id() set severity="medium"
      - name: Low
        filter: select where severity = "low" and status != "done" order by priority, createdAt
        action: update where id = id() set severity="low"

  # Architecture documentation
  - name: Architecture
    description: "System architecture and design decisions"
    kind: wiki
    path: "architecture/index.md"
    key: "F7"

# Triggers: automation rules
# This section replaces the entire default trigger list.
# We keep the shipped triggers we want and add our own.
triggers:
  # --- Shipped triggers we are keeping ---
  - description: block completion with open dependencies
    ruki: >
      before update
        where new.status = "done" and new.dependsOn any status != "done"
        deny "cannot complete: has open dependencies"
  - description: tasks must pass through review before completion
    ruki: >
      before update
        where new.status = "done" and old.status != "review"
        deny "tasks must go through testing before marking done"
  - description: remove deleted task from dependency lists
    ruki: >
      after delete
        update where old.id in dependsOn set dependsOn=dependsOn - [old.id]
  - description: clean up completed tasks after 24 hours
    ruki: >
      every 1day
        delete where status = "done" and updatedAt < now() - 1day
  - description: tasks must have an assignee before starting
    ruki: >
      before update
        where new.status = "inProgress" and new.assignee is empty
        deny "assign someone before moving to in-progress"
  - description: auto-complete epics when all child tasks finish
    ruki: >
      after update
        where new.status = "done" and new.type != "epic"
        update where type = "epic" and new.id in dependsOn and dependsOn all status = "done"
        set status="done"
  - description: cannot delete tasks that are actively being worked
    ruki: >
      before delete
        where old.status = "inProgress"
        deny "cannot delete an in-progress task — move to backlog or done first"
  - description: spawn next occurrence when recurring task completes
    ruki: >
      after update
        where new.status = "done" and old.recurrence is not empty
        create title=old.title priority=old.priority tags=old.tags
               recurrence=old.recurrence due=next_date(old.recurrence) status="backlog"

  # --- Our new triggers ---
  - description: per-person WIP limit of 3
    ruki: >
      before update
        where new.status = "inProgress"
          and count(select where assignee = new.assignee and status = "inProgress") > 2
        deny "you already have 3 tasks in progress — finish one first"
  - description: auto-prioritize critical bugs
    ruki: >
      after update
        where new.severity = "critical"
        update where id = new.id set priority=1
```

And the companion `.doc/config.yaml`:

```yaml
appearance:
  theme: tokyo-night

header:
  visible: true
```

## 12. What's next

You now have a fully customized workflow. Here are some places to go from here:

- **[Customization reference](customization.md)** — the complete list of
  everything you can put in `workflow.yaml`
- **[ruki reference](ruki/index.md)** — the full language for filters, actions,
  and triggers
- **[Custom fields](custom-fields.md)** — more on field types, templates, and
  edge cases
- **[Themes](../themes.md)** — screenshots of every available theme
- **[Recipes](../ideas/plugins.md)** — ready-made view and trigger ideas you can
  copy into your workflow
