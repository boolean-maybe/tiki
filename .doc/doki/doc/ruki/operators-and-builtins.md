# Operators And Built-ins

## Table of contents

- [Overview](#overview)
- [Operator precedence and associativity](#operator-precedence-and-associativity)
- [Comparison operators](#comparison-operators)
- [Membership](#membership)
- [Any And All](#any-and-all)
- [Binary expression operators](#binary-expression-operators)
- [Built-in functions](#built-in-functions)
- [Shell-related forms](#shell-related-forms)

## Overview

This page describes the operators and built-in functions available in `ruki`.

## Operator precedence and associativity

Condition precedence:

| Level | Operators or forms | Associativity |
|---|---|---|
| 1 | condition in parentheses, comparison, `is empty`, `in`, `not in`, `any`, `all` | n/a |
| 2 | `not` | right-associative by grammar recursion |
| 3 | `and` | left-associative |
| 4 | `or` | left-associative |

Expression precedence:

| Level | Operators | Associativity |
|---|---|---|
| 1 | `+`, `-` | left-associative |

Examples:

```sql
select where not status = "done"
select where priority = 1 or priority = 2 and status = "done"
create title="x" due=2026-03-25 + 2day - 1day
```

## Comparison operators

Supported operators:

- `=`
- `!=`
- `<`
- `>`
- `<=`
- `>=`

Supported by type:

| Type | `=` / `!=` | Ordering operators |
|---|---|---|
| `string` | yes | no |
| `status` | yes | no |
| `type` | yes | no |
| `id` | yes | no |
| `ref` | yes | no |
| `int` | yes | yes |
| `date` | yes | yes |
| `timestamp` | yes | yes |
| `duration` | yes | yes |
| `bool` | yes | no |
| `recurrence` | yes | no |
| list types | yes | no |

List equality uses set semantics: order and duplicate count do not affect `=` / `!=`.

Examples:

```sql
select where priority <= 2
select where due < 2026-03-25
select where updatedAt - createdAt > 7day
select where title != "hello"
```

Invalid examples:

```sql
select where status < "done"
select where title < "hello"
```

## Membership

Membership and substring:

- `<expr> in <collection>`
- `<expr> not in <collection>`

The `in` operator has two modes depending on the right-hand side:

**List membership** — when the right side is a list:

- checks whether the value appears in the list
- membership uses stricter compatibility than general comparison typing
- `id` and `ref` are treated as compatible for membership

**Substring check** — when the right side is a `string` field:

- checks whether the left side is a substring of the right side
- both sides must be `string` type (not `status`, `type`, `id`, or `ref`)

Examples:

```sql
select where "bug" in tags
select where id in dependsOn
select where status in ["done", "cancelled"]
select where status not in ["done"]
select where "bug" in title
select where "bug" in title and "fix" in title
select where "ali" in assignee
select where "bug" not in title
```

Invalid examples:

```sql
select where "done" in status
select where "x" in id
```

## Any And All

`any` means at least one related tiki matches the condition.

`all` means every related tiki matches the condition.

Use them after a field that contains related tikis, such as `dependsOn`:

- `<expr> any <condition>`
- `<expr> all <condition>`

Rules:

- the left side must be a field that contains related tikis, such as `dependsOn`
- after `any` or `all`, write a condition about those related tikis
- do not use `old.` or `new.` inside that condition

Examples:

```sql
select where dependsOn any status != "done"
select where dependsOn all status = "done"
```

These mean:

- `dependsOn any status != "done"`: at least one dependency is not done
- `dependsOn all status = "done"`: every dependency is done

Invalid examples:

```sql
select where tags any status = "done"
before update where dependsOn any old.status = "done" deny "blocked"
```

## Binary expression operators

`+`

- string-like plus string-like yields `string`
- `int + int` yields `int`
- `list<string> + string` and `list<string> + list<string>` yield `list<string>` (set union, no duplicates)
- `list<ref> + id-or-ref-compatible value` and `list<ref> + list<ref>` yield `list<ref>` (set union, no duplicates)
- `date + duration` yields `date`
- `timestamp + duration` yields `timestamp`

`-`

- `int - int` yields `int`
- `list<string> - string` and `list<string> - list<string>` yield `list<string>` (remove matching entries)
- `list<ref> - id-or-ref-compatible value` and `list<ref> - list<ref>` yield `list<ref>` (remove matching entries)
- `date - duration` yields `date`
- `date - date` yields `duration`
- `timestamp - duration` yields `timestamp`
- `timestamp - timestamp` yields `duration`

Examples:

```sql
create title="hello" + " world"
create title="x" tags=tags + ["new"]
create title="x" dependsOn=dependsOn - ["TIKI-ABC123"]
create title="x" due=2026-03-25 + 2day
select where updatedAt - createdAt > 1day
```

Invalid examples:

```sql
create title="x" priority=1 + "a"
select where due = 2026-03-25 + 2026-03-20
create title="x" dependsOn=dependsOn + tags
```

## Built-in functions

`ruki` has these built-ins:

| Name | Result type | Arguments | Notes |
|---|---|---|---|
| `count(...)` | `int` | exactly 1 | argument must be a `select` subquery |
| `now()` | `timestamp` | 0 | no additional validation |
| `next_date(...)` | `date` | exactly 1 | argument must be `recurrence` |
| `blocks(...)` | `list<ref>` | exactly 1 | argument must be `id`, `ref`, or string literal |
| `id()` | `id` | 0 | valid only in plugin runtime; resolves to selected tiki ID |
| `input()` | declared type | 0 | valid only in plugin actions with `input:` declaration |
| `call(...)` | `string` | exactly 1 | argument must be `string` |
| `user()` | `string` | 0 | no additional validation |
| `choose(...)` | `ref` | exactly 1 | argument must be a `select` subquery; plugin runtime only |

Examples:

```sql
select where count(select where status = "done") >= 1
select where updatedAt < now()
create title="x" due=next_date(recurrence)
select where blocks(id) is empty
select where id() in dependsOn
create title=call("echo hi")
select where assignee = user()
update where id = id() set assignee = input()
update where id = id() set tags = tags + [input()]
update where id = choose(select where type = "epic") set dependsOn = dependsOn + id()
create title = input()
```

Runtime notes:

- `id()` is semantically valid only in plugin runtime.
- When a validated statement uses `id()`, plugin execution must provide a non-empty selected task ID.
- Actions using `id()` automatically gain an `id` [requirement](../customization/customization.md#action-requirements) — the action is disabled when no task is selected. Mutating actions (`update`, `delete`) that do not use `id()` are bulk actions and remain enabled without a selection.
- `id()` is rejected for CLI, event-trigger, and time-trigger semantic runtimes.
- `call(...)` is currently rejected by semantic validation.
- `input()` returns the value typed by the user at the action prompt. Its return type matches the `input:` declaration on the action (e.g. `input: string` means `input()` returns `string`). Only valid in plugin action statements that declare `input:`. May only appear once per action. Accepted `timestamp` input formats: RFC3339, with YYYY-MM-DD as a convenience fallback.
- `choose()` is semantically valid only in plugin runtime. Opens an interactive Quick Select picker — the user fuzzy-filters candidate tasks and confirms one with Enter. Esc cancels the operation. The selected task's ID is the return value. `choose()` may only appear once per action. `choose()` and `input()` are mutually exclusive within a single action. Rejected for CLI, event-trigger, and time-trigger semantic runtimes.

`run(...)`

- not a normal expression built-in
- only valid as the top-level action of an `after` trigger
- its command expression must validate as `string`

Example:

```sql
after update where new.status = "in progress" run("echo hello")
```

## Shell-related forms

`ruki` includes four shell-related forms:

**`call(...)`** — a string-returning expression

- Returns a string result from a shell command
- Can be used in any expression context

**`run(...)` in triggers** — an `after`-trigger action

- Used as the top-level action of an `after` trigger
- Command string may reference `old.` and `new.` fields
- Example: `after update where new.status = "in progress" run("echo hello")`

**`| run(...)` on select** — a pipe suffix

- Executes a command for each row returned by `select`
- Uses positional arguments `$1`, `$2`, etc. for field substitution
- Command must be a string literal or expression, but **field references are not allowed in the command** itself
- `|` is a statement suffix, not an operator
- Example: `select id, title where status = "done" | run("myscript $1 $2")`

**`| clipboard()` on select** — a pipe suffix

- Copies selected field values to the system clipboard
- Fields within a row are tab-separated; rows are newline-separated
- Takes no arguments — grammar enforces empty parentheses
- Cross-platform via `atotto/clipboard` (macOS, Linux, Windows)
- Example: `select id where id = id() | clipboard()`

Comparison of pipe targets and trigger actions:

| Form | Context | Field access | Behavior |
|---|---|---|---|
| trigger `run()` | after-trigger action | `old.`, `new.` allowed | shell execution via expression evaluation |
| pipe `\| run()` | select suffix | field refs disallowed in command | shell execution with positional args `$1`, `$2` |
| pipe `\| clipboard()` | select suffix | n/a (no arguments) | writes rows to system clipboard |

See [Semantics](semantics.md#pipe-actions-on-select) for pipe evaluation model.
