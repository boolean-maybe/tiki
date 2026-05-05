# Semantics

## Table of contents

- [Overview](#overview)
- [Statement semantics](#statement-semantics)
- [Trigger semantics](#trigger-semantics)
- [Qualifier scope](#qualifier-scope)
- [Plugin target qualifiers](#plugin-target-qualifiers)
- [Time trigger semantics](#time-trigger-semantics)
- [Condition and expression semantics](#condition-and-expression-semantics)

## Overview

This page explains how `ruki` statements, triggers, conditions, and expressions behave.

## Statement semantics

`select`

- `select` without `where` means a statement with no condition node.
- `select where ...` validates the condition and its contained expressions.
- `select ... order by <field> [asc|desc], ...` specifies result ordering.
- A subquery form `select` or `select where ...` can appear only inside `count(...)`, `choose(...)`, or
  `exists(...)`. Subqueries do not support `order by`.

`order by`

- Each field must exist in the schema and be an orderable type.
- Orderable types: `int`, `date`, `timestamp`, `duration`, `string`, `status`, `type`, `id`, `ref`.
- Non-orderable types: `list<string>`, `list<ref>`, `recurrence`, `bool`.
- Default direction is ascending. Use `desc` for descending.
- Duplicate fields are rejected.
- Only bare field names are allowed — `old.` and `new.` qualifiers are not valid in `order by`.

`limit`

- Must be a positive integer.
- Applied after filtering and sorting, before any pipe action.
- If the limit exceeds the result count, all results are returned (no error).

`create`

- `create` is a list of assignments.
- At least one assignment is required.
- The resulting tiki must have a non-empty `title`. This can come from an explicit `title=...` assignment or from
  the tiki template.
- Duplicate assignments to the same field are rejected.
- Every assigned field must exist in the injected schema.
- `id`, `createdBy`, `createdAt`, `updatedAt`, and `filepath` are immutable and cannot be assigned.

`update`

- `update` has two parts: a `where` condition and a `set` assignment list.
- At least one assignment in `set` is required.
- The `where` clause and every right-hand side expression are validated.
- Duplicate assignments inside `set` are rejected.
- `id`, `createdBy`, `createdAt`, `updatedAt`, and `filepath` are immutable and cannot be assigned.

`delete`

- `delete` always requires a `where` condition.
- The `where` condition is validated exactly like `select where ...`.

## Trigger semantics

Triggers have the shape:

```text
<timing> <event> [where <condition>] <deny-or-action>
```

Rules:

- `before` triggers must have `deny`.
- `before` triggers must not have an action or `run(...)`.
- `after` triggers must not have `deny`.
- `after` triggers must have either a CRUD action or `run(...)`.
- trigger CRUD actions may be `create`, `update`, or `delete`, but not `select`

Examples:

```sql
before update where new.status = "done" and dependsOn any status != "done" deny "open dependencies"
after create where new.priority <= 2 and new.assignee is empty update where id = new.id set assignee="booleanmaybe"
after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]
```

At runtime, triggers execute in a pipeline: before-triggers run as validators before persistence, the mutation is
persisted, then after-triggers run as hooks. For the full execution model, cascade behavior, configuration, and
runtime details, see [Triggers](triggers.md).

## Qualifier scope

Qualifier rules depend on the event:

- `create` triggers: `new.` is allowed, `old.` is not
- `delete` triggers: `old.` is allowed, `new.` is not
- `update` triggers: both `old.` and `new.` are allowed
- standalone statements: neither `old.` nor `new.` is allowed
- `outer.` is allowed only inside a subquery body
- `target.` and `targets.` are allowed only in plugin runtime (see
  [Plugin target qualifiers](#plugin-target-qualifiers)); they are rejected in event triggers
  and time triggers

Examples:

```sql
before create where new.type = "story" and new.description is empty deny "stories must have a description"
before delete where old.priority <= 2 deny "cannot delete high priority tikis"
before update where old.status = "in progress" and new.status = "done" deny "review required"
select where not exists(select where outer.id in dependsOn)
```

![Qualifier scope by context](images/qualifier-scope.svg)

Important special case:

- inside a quantifier body such as `dependsOn any ...`, `old.` and `new.` are disabled again
- use bare fields inside the quantifier body, not `old.` or `new.`
- `outer.` remains available if that quantifier body is itself inside a subquery

Example:

```sql
before update where dependsOn any status = "done" deny "blocked"
```

Correlated subqueries:

- inside `count(select ...)`, `choose(select ...)`, and `exists(select ...)`, bare fields refer to the subquery's
  candidate task
- `outer.field` refers to the immediate parent row that invoked the subquery
- nested subqueries rebind `outer.` to their direct parent subquery row, not to the top-level row
- in trigger subqueries, `old.` and `new.` still refer to the trigger snapshots while `outer.` refers to the
  guard or action target row

Examples:

```sql
select where not exists(select where outer.id in dependsOn)
select where count(select where assignee = outer.assignee and status = "in progress") > 1
before update where exists(select where id = outer.id and assignee = new.assignee) deny "blocked"
```

## Plugin target qualifiers

Plugin runtime adds two qualifiers that resolve against the currently selected tasks:

- `target.<field>` — the field value of the single selected task. Uses the same exactly-one-selection
  contract as `id()`: zero selections raise `MissingSelectedTaskIDError`, more than one raises
  `AmbiguousSelectedTaskIDError`.
- `targets.<field>` — a deduped projection of the named field across all selected tasks, preserving
  first-seen order (by selection order, then by field value order within a list field). Zero selected
  tasks produce an empty list, matching `ids()`.

Rules:

- both qualifiers are only valid in plugin runtime; standalone CLI, event triggers, and time triggers
  reject them at semantic validation time.
- `targets.<field>` flattens list-valued fields. `targets.tags` and `targets.dependsOn` produce a flat
  list of unique values rather than a list of lists.
- `targets.<field>` does not automatically exclude the selected task IDs from the result. Subtract
  them explicitly if needed (for example `targets.dependsOn - targets.id`).
- projection types stay within existing list types:
  - `id`, ref fields, and `list<ref>` fields project to `list<ref>`.
  - `string`, `status`, `type`, enum, and `list<string>` fields project to `list<string>`.
  - scalar types without a list representation (`int`, `date`, `timestamp`, `duration`, `bool`,
    `recurrence`) are rejected — use `target.<field>` for those or convert explicitly.
- a selected task ID that does not resolve to a known task raises a clear runtime error.
- lane `filter:` expressions reject `target.` and `targets.` at parse time. Lane filters run on
  every render with no selection payload, so those qualifiers cannot resolve. Use them in plugin
  actions or lane `action:` expressions instead (lane actions receive the moved task as a single
  selection).

Examples:

```sql
-- act on the same task the user is focused on
update where id = target.id set status = "done"

-- expand work to the selected task's blockers, de-duped across the selection
select where id in targets.dependsOn

-- copy the selected task's assignee to other items in the board
update where type = "bug" set assignee = target.assignee

-- filter the board down to statuses the selection covers
select where status in targets.status
```

Plugin actions that reference `target.<field>` or `targets.<field>` auto-infer the same selection
requirements as `id()` and `ids()`: `target.` requires exactly one selected task, and `targets.`
requires at least one. The executor preflights the single-selection contract before evaluating
the statement, so the error surfaces even when `target.<field>` sits behind a short-circuited
condition.

## Time trigger semantics

Time triggers have the shape:

```text
every <duration> <statement>
```

Rules:

- the interval must be a positive duration (e.g. `1hour`, `2day`, `1week`)
- the inner statement must be `create`, `update`, or `delete` — not `select`
- `run()` is not allowed inside a time trigger
- `old.` and `new.` qualifiers are not allowed — there is no mutation context for a periodic operation
- `target.` and `targets.` are not allowed — time triggers have no plugin selection context
- bare field references in the inner statement resolve against the tasks being matched, exactly as in standalone
  statements

Examples:

```sql
every 1hour update where status = "in_progress" and updatedAt < now() - 7day set status="backlog"
every 1day delete where status = "done" and updatedAt < now() - 30day
every 2week create title="sprint review" status="ready" priority=3
```

## Condition and expression semantics

Conditions:

- a bare expression condition must evaluate to a boolean value
- comparisons validate both operand types before checking operator legality
- `is empty` and `is not empty` are allowed on every supported type
- `in` and `not in` require a collection on the right side
- `any` and `all` require `list<ref>` on the left side
- list equality (`=` / `!=`) is set-like: order and duplicate count are ignored

### Absent fields

Every field on a tiki is either *present* (the frontmatter declares the key) or *absent* (the key is
not in the frontmatter at all). Comparisons and quantifiers have well-defined semantics for absent
fields — the engine does not hard-error on absence, it just resolves to a defined value. Schema-known
fields (`status`, `type`, `priority`, `points`, `tags`, `dependsOn`, `due`, `recurrence`, `assignee`) and
custom user-defined fields share the same presence-aware behavior.

Rules that follow from presence:

- Equality with a concrete value is asymmetric on absent fields: `where <field> = <value>` is **false**
  and `where <field> != <value>` is **true**. `where priority = 0` does not match tikis that never
  declared priority; only tikis whose frontmatter literally wrote `priority: 0` match. `where priority
  != 0` matches both "declared priority other than 0" *and* "no priority declared".
- Equality with `empty` treats absent as empty: `where <field> = empty` is **true** for absent fields,
  and `where <field> != empty` is **false**. This is the only equality form where absent and
  present-zero behave the same.
- Ordering comparisons (`<`, `>`, `<=`, `>=`) on an absent field hard-error — there is no defined ordering
  for "no value". Guard ordering predicates with `has(<field>)` when the input could be sparse.
- `where <field> is empty` treats absent as empty and evaluates **true**; `where <field> is not empty`
  evaluates **false**. So `is empty` cannot distinguish "absent" from "present-but-zero" — use
  `has(<field>)` when you need that distinction.
- `where <list> = []` is an equality comparison against a concrete value, not against `empty`, so it
  follows the asymmetric rule: it does **not** match absent lists. Match absent lists with `is empty`,
  `= empty`, or `not has(<field>)` instead.
- `any` returns **false** over an absent list (no elements to satisfy the predicate). `all` returns
  **true** over an absent list (vacuous truth) — the same result as a present-but-empty list.
- `in` returns **false** when its left-hand side is absent (`where assignee in ["bob"]` does not match
  tikis without `assignee`). `not in` returns **true** in the same case (`where assignee not in
  ["bob"]` *does* match tikis without `assignee`). When you want "value-is-set and not in this list",
  combine `has(<field>)` with `not in`.
- `where has(<field>)` is the only predicate that distinguishes "present with zero value" from "absent".
  `has()` also accepts qualified references — `has(new.assignee)` is the canonical "new tiki declares an
  assignee" predicate in triggers.
- `order by <field>` requires every row to have a defined value. Absent values raise an error; gate the
  query with `has(<field>)` or sort on a different field. (Some sparse-friendly views may add documented
  null-ordering in the future.)
- Projections (`select id, title, priority`) render absent scalar fields as empty cells in the table
  formatter and as JSON `null` in the JSON formatter. List fields always render as `[]` so downstream
  scripts can iterate without a null check; if the caller needs to distinguish "absent list" from
  "present-empty list", they should gate the select with `has(<field>)` in the where clause.
- Arithmetic contexts (`set dependsOn = dependsOn + "ABC123"`, `set tags = tags - ["a"]`) treat an absent
  list operand as an empty list because the expression *constructs* a new list. The assignment then
  writes the resulting list to frontmatter — this is the canonical "first-time add" idiom. The list is
  written even when the result is empty, so `has(<field>)` becomes true after the assignment regardless
  of length.

Setting any field on a tiki simply writes that key into its frontmatter:
`update where id = "ABC123" set status = "ready"` adds `status: ready`. There is no separate "promotion"
step or hidden classifier — fields are added like ordinary map entries. Sparse serialization is
preserved: `set points = 0` writes exactly `points: 0`, not a full schema.

Removing a field is explicit. The canonical clear is `set <field> = empty`: assigning the empty literal
deletes the key from frontmatter so subsequent loads see it as absent. List arithmetic does *not* delete
a list field even when the result is empty — `set tags = tags - ["only-tag"]` writes an empty list
(`tags: []`) and `has(tags)` remains true. Use `set tags = empty` to actually remove the key.

Examples:

```sql
select where true
select where blocked
select where not blocked
select where title     -- invalid: title is string-typed
select where priority  -- invalid: priority is int-typed
```

Expressions:

- field references resolve through the injected schema
- qualified references use the same field catalog, then apply qualifier-policy checks
- list literals must be homogeneous
- `empty` is a context-sensitive zero value, resolved by surrounding type checks
- subqueries are only legal as the argument to `count(...)`, `choose(...)`, or `exists(...)`

Binary `+` and `-` are semantic rather than purely numeric:

- string-like `+` yields `string`
- `int + int` and `int - int` yield `int`
- `list<string> +/- string-or-list<string>` yields `list<string>` with set semantics
- `list<ref> +/- id-ref-compatible values` yields `list<ref>` with set semantics
- `date + duration` yields `date`
- `date - duration` yields `date`
- `date - date` yields `duration`
- `timestamp + duration` yields `timestamp`
- `timestamp - duration` yields `timestamp`
- `timestamp - timestamp` yields `duration`

![Binary operator type resolution](images/binary-op-types.svg)

For the detailed type rules and built-ins, see [Types And Values](types-and-values.md) and
[Operators And Built-ins](operators-and-builtins.md).

## Pipe actions on select

`select` statements may include an optional pipe suffix:

```text
select <fields> where <condition> [order by ...] [limit N] | run(<command>)
select <fields> where <condition> [order by ...] [limit N] | clipboard()
```

### `| run(...)` — shell execution

Evaluation model:

- The `select` runs first, producing zero or more rows.
- For each row, the `run()` command is executed with positional arguments (`$1`, `$2`, etc.) substituted from the
  selected fields in left-to-right order.
- Each command execution has a **30-second timeout**.
- Command failures are **non-fatal** — remaining rows still execute.
- Stdout and stderr are **fire-and-forget** (not captured or returned).

Rules:

- Explicit field names are required — `select *` and bare `select` are rejected when used with a pipe.
- The command expression must be a string literal or string-typed expression, but **field references are not
  allowed** in the command string itself.
- Positional arguments `$1`, `$2`, etc. are substituted by the runtime before each command execution.

Example:

```sql
select id, title where status = "done" | run("myscript $1 $2")
```

For a task with `id = "ABC123"` and `title = "Fix bug"`, the command becomes:

```bash
myscript "ABC123" "Fix bug"
```

Pipe `| run(...)` on select is distinct from trigger `run()` actions. See [Triggers](triggers.md) for the difference.

### `| clipboard()` — copy to clipboard

Evaluation model:

- The `select` runs first, producing zero or more rows.
- The selected field values are written to the system clipboard.
- Fields within a row are **tab-separated**; rows are **newline-separated**.
- Uses `atotto/clipboard` internally — works on macOS, Linux (requires `xclip` or `xsel`), and Windows.

Rules:

- Explicit field names are required — same restriction as `| run(...)`.
- `clipboard()` takes no arguments — the grammar enforces empty parentheses.

Examples:

```sql
select id where id = id() | clipboard()
select id, title where status = "done" | clipboard()
```

For a single task with `id = "ABC123"` and `title = "Fix bug"`, the clipboard receives:

```text
ABC123	Fix bug
```

