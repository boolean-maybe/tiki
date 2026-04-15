# Quick Start

## Table of contents

- [Overview](#overview)
- [Mental model](#mental-model)
- [CRUD statements](#crud-statements)
- [Conditions and expressions](#conditions-and-expressions)
- [Triggers](#triggers)
- [Where to go next](#where-to-go-next)

## Overview

This page is a practical introduction to the `ruki` language. It covers the main statement forms, the conditions they use, and the trigger rules that let you block or react to changes.

## Mental model

`ruki` has two top-level forms:

- Statements: `select`, `create`, `update`, and `delete`
- Triggers: `before` or `after` rules attached to `create`, `update`, or `delete`

Statements read and change tiki fields such as `status`, `type`, `tags`, `dependsOn`, `priority`, and `due`. Triggers use the same fields and conditions, but add `before` or `after` timing around `create`, `update`, or `delete`.

The simplest way to read `ruki` is:

- `select` filters tikis
- `create` assigns fields for a new tiki
- `update` finds tikis with `where`, then applies `set`
- `delete` finds tikis with `where`, then removes them
- `before ... deny "message"` blocks an operation when its guard matches
- `after ... <action>` reacts to an operation by creating, updating, deleting, or `run(...)`

## CRUD statements

`select` reads:

```sql
select
select title, status
select id, title where status = "done"
select where "bug" in tags and priority <= 2
select where status != "done" order by priority limit 3
```

`create` writes one or more assignments:

```sql
create title="Fix login"
create title="Fix login" priority=2 status="ready" tags=["bug"]
```

`update` always has a `where` clause and a `set` clause:

```sql
update where id = "TIKI-ABC123" set status="done"
update where status = "ready" and "sprint-3" in tags set status="cancelled"
```

`delete` always has a `where` clause:

```sql
delete where id = "TIKI-ABC123"
delete where status = "cancelled" and "old" in tags
```

`select` may pipe results to a shell command or to the clipboard:

```sql
select id, title where status = "done" | run("myscript $1 $2")
select id where id = id() | clipboard()
```

`| run(...)` executes the command for each row with field values as positional arguments (`$1`, `$2`). `| clipboard()` copies the selected fields to the system clipboard.

## Conditions and expressions

Conditions support:

- comparisons such as `status = "done"` or `priority <= 2`
- emptiness checks such as `assignee is empty`
- membership checks such as `"bug" in tags`
- quantifiers over `list<ref>` values such as `dependsOn any status != "done"`
- boolean composition with `not`, `and`, and `or`

Examples:

```sql
select where status = "done" and priority <= 2
select where assignee is empty
select where status not in ["done", "cancelled"]
select where dependsOn all status = "done"
select where not (status = "done" or priority = 1)
```

Expressions include literals, field references, built-in calls, list literals, subqueries for `count(...)`, and `+` or `-` binary expressions:

```sql
create title="Fix login"
create title="x" due=2026-03-25 + 2day
create title="x" tags=tags + ["needs-triage"]
create title="x" due=next_date(recurrence)
select where count(select where status = "done") >= 1
```

## Triggers

Triggers add timing and event context around the same condition language.

`before` triggers can only `deny`:

```sql
before update where new.status = "done" and dependsOn any status != "done" deny "cannot complete tiki with open dependencies"
before delete where old.priority <= 2 deny "cannot delete high priority tikis"
```

`after` triggers can perform an action:

```sql
after create where new.priority <= 2 and new.assignee is empty update where id = new.id set assignee="booleanmaybe"
after update where new.status = "done" and old.recurrence is not empty create title=old.title priority=old.priority tags=old.tags recurrence=old.recurrence due=next_date(old.recurrence) status="ready"
after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]
```

`after` triggers may also use `run(...)` as a top-level action:

```sql
after update where new.status = "in progress" and "claude" in new.tags run("claude -p 'implement tiki " + old.id + "'")
```

Triggers are configured in `workflow.yaml` under the `triggers:` key. See [Triggers](triggers.md) for configuration details and runtime behavior.

## Where to go next

### Recipes
ready-to-use examples for common workflow patterns

- [Plugins](../ideas/plugins.md)
- [Triggers](../ideas/triggers.md)

### More info

- Use [Triggers](triggers.md) for configuration, execution model, and runtime behavior.
- Use [Syntax](syntax.md) for the grammar-level reference.
- Use [Types And Values](types-and-values.md) for the type system and literal rules.
- Use [Operators And Built-ins](operators-and-builtins.md) for precedence and function signatures.
- Use [Examples](examples.md) for more complete programs and invalid cases.
