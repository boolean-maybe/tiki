# Triggers

## Table of contents

- [Overview](#overview)
- [What triggers look like](#what-triggers-look-like)
- [Configuration](#configuration)
- [Patterns](#patterns)
- [Tips and gotchas](#tips-and-gotchas)
- [Execution pipeline](#execution-pipeline)
- [Before-trigger behavior](#before-trigger-behavior)
- [After-trigger behavior](#after-trigger-behavior)
- [Cascade depth](#cascade-depth)
- [The run() action](#the-run-action)
- [Configuration discovery details](#configuration-discovery-details)
- [Startup and error handling](#startup-and-error-handling)

## Overview

Triggers are reactive rules that fire when tikis are created, updated, or deleted. A before-trigger can block a mutation with a denial message. An after-trigger can react to a mutation by creating, updating, deleting tikis, or running a shell command.

This page covers how triggers are configured, how they execute at runtime, and common patterns. For the grammar, see [Syntax](syntax.md). For structural rules and qualifier scoping, see [Semantics](semantics.md). For parse and validation errors, see [Validation And Errors](validation-and-errors.md).

## What triggers look like

A before-trigger guards against unwanted changes:

```sql
-- block completing a tiki that has unfinished dependencies
before update
  where new.status = "done" and dependsOn any status != "done"
  deny "cannot complete tiki with open dependencies"
```

- `before update` — fires before an update is persisted
- `where ...` — the guard condition; the trigger only fires when this matches
- `deny "..."` — the rejection message returned to the caller

An after-trigger automates a reaction:

```sql
-- when a recurring tiki is completed, create the next occurrence
after update
  where new.status = "done" and old.recurrence is not empty
  create title=old.title priority=old.priority tags=old.tags
         recurrence=old.recurrence due=next_date(old.recurrence) status="ready"
```

- `after update` — fires after an update is persisted
- `where ...` — the guard; the action only runs when this matches
- `create ...` — the action to perform (can also be `update`, `delete`, or `run(...)`)

Triggers without a `where` clause fire on every matching event:

```sql
-- clean up reverse dependencies whenever a tiki is deleted
after delete
  update where old.id in dependsOn set dependsOn=dependsOn - [old.id]
```

## Configuration

Triggers are defined in `workflow.yaml` under the `triggers:` key. Each entry has two fields:

- `ruki` — the trigger rule in Ruki syntax (required)
- `description` — an optional label

```yaml
triggers:
  - description: "block done with open deps"
    ruki: >-
      before update
      where new.status = "done" and dependsOn any status != "done"
      deny "resolve dependencies first"

  - description: "auto-assign urgent"
    ruki: >-
      after create
      where new.priority <= 2 and new.assignee is empty
      update where id = new.id set assignee="booleanmaybe"

  - description: "cleanup deps on delete"
    ruki: >-
      after delete
      update where old.id in dependsOn set dependsOn=dependsOn - [old.id]
```

`workflow.yaml` is searched in the standard configuration locations described in [Configuration](../config.md#configuration-precedence). If multiple files define a `triggers:` section, the last one wins — cwd overrides project, which overrides user. A file without a `triggers:` key does not override anything. An explicit empty list (`triggers: []`) overrides inherited triggers to zero.

## Patterns

### Limit work in progress

Prevent anyone from having too many in-progress tikis at once:

```sql
before update
  where new.status = "in progress"
    and count(select where assignee = new.assignee and status = "in progress") >= 3
  deny "WIP limit reached for this assignee"
```

The `count(select ...)` evaluates against the candidate state — the proposed update is already reflected in the count, so the limit fires before persistence.

### Auto-assign urgent work

Automatically assign high-priority tikis that arrive without an owner:

```sql
after create
  where new.priority <= 2 and new.assignee is empty
  update where id = new.id set assignee="booleanmaybe"
```

The after-trigger fires after the tiki is persisted, then updates it with the assignee. This cascades through the mutation gate, so any update validators (like WIP limits) still apply to the auto-assignment.

### Recurring task creation

When a recurring tiki is completed, create the next occurrence:

```sql
after update
  where new.status = "done" and old.recurrence is not empty
  create title=old.title priority=old.priority tags=old.tags
         recurrence=old.recurrence due=next_date(old.recurrence) status="ready"
```

The new tiki inherits the original's title, priority, tags, and recurrence pattern. Its due date is set to the next occurrence using `next_date()`.

### Dependency cleanup on delete

When a tiki is deleted, remove it from every other tiki's `dependsOn` list:

```sql
after delete
  update where old.id in dependsOn set dependsOn=dependsOn - [old.id]
```

This fires after every delete, with no guard condition. The `old.id in dependsOn` condition finds tikis that depend on the deleted one, and the set clause removes the reference.

### Cascade completion

Auto-complete an epic when all its dependencies are done:

```sql
after update
  where new.status = "done"
  update where id in blocks(old.id) and type = "epic"
    and dependsOn all status = "done"
    set status="done"
```

When any tiki is marked done, this finds epics that block on it. If all of the epic's other dependencies are also done, the epic is completed automatically. This itself fires further after-update triggers, so cascade chains work naturally (up to the depth limit).

### Propagate cancellation

When a tiki is cancelled, cancel downstream tikis that haven't started:

```sql
after update
  where new.status = "cancelled"
  update where id in blocks(old.id) and status in ["backlog", "ready"]
    set status="cancelled"
```

Only tikis in `backlog` or `ready` are affected — in-progress work is not cancelled automatically.

### Run an external command

Trigger a script when a tiki enters a specific state:

```sql
after update
  where new.status = "in progress" and "claude" in new.tags
  run("claude -p 'implement tiki " + old.id + "'")
```

The `run()` action evaluates the expression to a command string, then executes it via `sh -c` with a 30-second timeout. Command failures are logged but do not block the mutation chain.

## Tips and gotchas

- Test your guard condition as a `select where ...` statement first. If the select returns unexpected results, the trigger will fire unexpectedly too.
- Before-triggers are fail-closed. If the guard expression itself has a runtime error, the mutation is rejected. Keep guard logic straightforward.
- Triggers that modify the same fields they guard on can cascade. For example, an after-update trigger that changes `status` will fire other after-update triggers. Design triggers to converge — avoid chains that cycle indefinitely. The cascade depth limit (8) prevents runaway loops, but silent termination is rarely what you want.
- `run()` commands execute with the permissions of the tiki process. Treat the `ruki` field in `workflow.yaml` the same as any other executable configuration.
- A parse error in any trigger definition prevents the app from starting. Validate your `workflow.yaml` before deploying.

---

## Execution pipeline

When a tiki is created, updated, or deleted, the mutation goes through this pipeline:

1. **Depth check** — reject if the trigger cascade depth exceeds the limit
2. **Before-validators** — run all registered before-triggers for this event; collect rejections
3. **Persist** — write the change to the store
4. **After-hooks** — run all registered after-triggers for this event

Before-triggers are registered as mutation validators. They run before persistence and can block the mutation. After-triggers are registered as hooks. They run after persistence and cannot undo it.

All validators for a given event run — rejections are accumulated, not short-circuited. If any validator rejects, the mutation is blocked and none of the rejection messages are lost.

After-hooks run in definition order. Each hook's errors are logged but do not propagate — the original mutation is unaffected.

## Before-trigger behavior

Before-triggers use **fail-closed** semantics:

- If the guard condition matches, the mutation is rejected with the `deny` message.
- If the guard condition evaluation itself errors (e.g. a runtime type error), the mutation is also rejected. This prevents bad triggers from silently allowing mutations they were meant to block.

The context provided to before-triggers depends on the event:

| Event | `old` | `new` | `allTasks` |
|---|---|---|---|
| create | nil | proposed task | stored tasks + proposed |
| update | persisted (cloned) | proposed version | stored tasks with proposed applied |
| delete | task being deleted | nil | current stored tasks |

For before-update triggers, `allTasks` reflects the **candidate state** — the proposed update is already applied in the task list. This matters for aggregate predicates like WIP limits using `count(select ...)`, which need to see the world as it would look after the update.

## After-trigger behavior

After-triggers use **fail-open** semantics:

- If the guard condition matches, the action executes.
- If the guard condition evaluation itself errors, the trigger is skipped and the error is logged. The mutation chain continues.
- If the action fails, the error is logged. The original mutation is not rolled back.

After-hooks read a fresh task list from the store each time they fire. This means cascaded triggers see the current state of the world, including changes made by earlier triggers in the chain.

After-triggers support two action forms:

- A CRUD action (`create`, `update`, or `delete`) — executed through the mutation gate, which fires its own triggers
- A `run()` command — executed as a shell command (see [The run() action](#the-run-action))

## Cascade depth

After-triggers can cause further mutations, which fire their own triggers, and so on. To prevent infinite loops, cascade depth is tracked:

- The root mutation (user-initiated) runs at depth 0.
- Each triggered mutation increments the depth by 1.
- At depth >= 8, after-hooks are skipped with a warning log.
- At depth > 8, the mutation gate rejects the mutation entirely.

The maximum cascade depth is **8**. Termination is graceful — a warning is logged, not a panic. Within a cascade, each after-hook reads the latest store state, so it sees changes from earlier triggers.

## The run() action

When an after-trigger uses `run(...)`, the command expression is evaluated to a string, then executed:

- The command runs via `sh -c <command-string>`.
- A **30-second timeout** is enforced. If the command exceeds it, the child process is killed.
- Command failure (non-zero exit) is **logged** but does not block the mutation chain.
- Command success is also logged.

The command string is dynamically evaluated from the trigger's expression, which may reference `old.id`, `new.status`, or other fields via string concatenation.

## Configuration discovery details

Trigger definitions are loaded from `workflow.yaml` using the standard [configuration precedence](../config.md#configuration-precedence). The **last file with a `triggers:` key wins**:

| File | `triggers:` key | Effect |
|---|---|---|
| user config | yes, 2 triggers | base: 2 triggers |
| project config | absent | no override, user triggers survive |
| cwd config | `triggers: []` | override: 0 triggers |

A file that exists but has no `triggers:` key expresses no opinion and does not override. An explicit empty list (`triggers: []`) is an active override that disables inherited triggers.

If two candidate paths resolve to the same absolute path (e.g. when the project root is the current directory), the file is read once.

## Startup and error handling

Triggers are loaded during application startup, after the store is initialized but before controllers are created.

- Each trigger definition is parsed with the ruki parser. A parse error in any trigger is **fail-fast**: the application will not start, and the error message identifies the failing trigger by its `description` (or by index if no description is set).
- If no `triggers:` section is found in any workflow file, zero triggers are loaded and the app starts normally.
- Successfully loaded triggers are logged with a count at startup.
