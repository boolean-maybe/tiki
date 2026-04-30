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
| `count(...)` | `int` | exactly 1 | argument must be a `select` subquery; usable as a top-level expression |
| `exists(...)` | `bool` | exactly 1 | argument must be a `select` subquery; true when any document matches; usable as a top-level expression |
| `has(...)` | `bool` | exactly 1 | argument must be a bare or qualified field reference (`has(status)`, `has(new.status)`, `has(outer.status)`, `has(target.status)`, `has(targets.status)`); true when the referenced task has an explicit value for that workflow field — used to distinguish "absent" from "present with zero value". See qualifier rules below. |
| `now()` | `timestamp` | 0 | no additional validation |
| `next_date(...)` | `date` | exactly 1 | argument must be `recurrence` |
| `blocks(...)` | `list<ref>` | exactly 1 | argument must be `id`, `ref`, or string literal |
| `id()` | `id` | 0 | plugin runtime only; requires exactly one selected tiki |
| `ids()` | `list<ref>` | 0 | plugin runtime only; returns the full set of selected tiki IDs |
| `selected_count()` | `int` | 0 | plugin runtime only; returns the number of selected tikis |
| `input()` | declared type | 0 | valid only in plugin actions with `input:` declaration |
| `call(...)` | `string` | exactly 1 | argument must be `string` |
| `user()` | `string` | 0 | resolves to the current Tiki identity (configured → git → OS user); errors if none resolves |
| `choose(...)` | `ref` | exactly 1 | argument must be a `select` subquery; plugin runtime only |

Examples:

```sql
select where updatedAt < now()
create title="x" due=next_date(recurrence)
select where blocks(id) is empty
select where id() in dependsOn
select where count(select where assignee = outer.assignee and status = "in progress") >= 3
before update where new.type = "epic"
and exists(select where id in new.dependsOn and status != "done")
deny "epic has unfinished dependencies"
create title=call("echo hi")
select where assignee = user()
select where has(status) and not has(assignee)
update where id = id() set assignee = input()
update where id = id() set tags = tags + [input()]
update where id in ids() set status = "done"
update where id = choose(select where type = "epic") set dependsOn = dependsOn + id()
create title = input()
```

Runtime notes:

- `id()` is the scalar selection builtin. It is only valid in plugin runtime and requires the caller to provide
  exactly one selected task ID. Zero selections raise `MissingSelectedTaskIDError`; two or more raise
  `AmbiguousSelectedTaskIDError` (the message suggests switching to `ids()`).
- Actions that reference `id()` automatically gain an `id`
  [requirement](../customization/customization.md#action-requirements) which is equivalent to `selection:one`. The
  action is disabled when the selection count is not exactly one.
- `ids()` is the multi-selection builtin. It returns the full `list<ref>` of currently selected task IDs, so the
  idiomatic multi-target update is `update where id in ids() set ...`. Zero selections produce an empty list
  rather than an error, so the statement becomes a no-op when nothing is selected.
- `selected_count()` returns the number of currently selected tasks, letting ruki branch on cardinality (e.g.
  `select where selected_count() >= 2`).
- Actions that reference `ids()` auto-infer a `selection:any` requirement (at least one selected task) unless the
  author supplies an explicit `selection:*` requirement. Actions that reference `selected_count()` do **not**
  auto-infer any selection requirement — the whole point of the builtin is to let ruki branch on cardinality
  including the zero case, so gating the action on a non-zero selection would make that branch unreachable. See
  [action requirements](../customization/customization.md#action-requirements) for the `selection:one`,
  `selection:any`, and `selection:many` cardinality tokens.
- `id()`, `ids()`, and `selected_count()` are all rejected for CLI, event-trigger, and time-trigger semantic
  runtimes.
- `user()` returns the current Tiki identity. Resolution order:
  configured `identity.name` or `identity.email` (via `config.yaml` or
  `TIKI_IDENTITY_NAME` / `TIKI_IDENTITY_EMAIL`), then the git user when
  `store.git` is `true`, then the OS account username. Either config field
  alone is sufficient — when only email is set, it is used as the display
  string. When none of those sources yields a value, `user()` returns an
  error (`user() is unavailable (no current user configured)`). See
  [Configuration / Identity resolution](../config.md#identity-resolution)
  for the full layered behavior.
- `exists(...)` is a non-interactive boolean builtin. Its subquery body is validated recursively.
- `has(<field>)` is the presence predicate. It returns `true` when the current document has an explicit value
  for the named workflow field and `false` when the field is absent. This is the only way to distinguish
  "absent" from "present with zero value": `where priority = 0` only matches documents whose priority was
  explicitly set to `0`, while `where has(priority)` matches any document that declares priority at all.
  Plain documents (no workflow frontmatter) always report every workflow field as absent, so
  `select where has(status)` is the canonical filter for "show only workflow documents". The argument must
  be a bare or qualified field reference — `has("status")` with a string literal is rejected. Qualifier
  resolution mirrors ordinary field references and is gated by the surrounding context:
  - `has(outer.X)` — parent-query row; valid only inside a `count()`/`choose()`/`exists()` subquery body.
  - `has(target.X)` — the exactly-one selected task; valid only in plugin runtime (same cardinality contract
    as `target.X` reads).
  - `has(targets.X)` — true iff **any** selected task has the field present; valid only in plugin runtime.
    Zero selections always yield `false`.
  - `has(new.X)` / `has(old.X)` — the proposed/previous task in trigger guards and actions.
- `count(...)`, `exists(...)`, `now()`, `user()`, and other non-interactive builtins may appear as top-level
  expression statements: `tiki exec 'count(select where status = "done")'` prints the scalar result instead of
  a table. Bare field references are rejected at the top level (no current task), but remain valid inside the
  subquery argument to `count(...)` or `exists(...)`.
- `call(...)` is currently rejected by semantic validation.
- `input()` returns the value typed by the user at the action prompt. Its return type matches the `input:`
  declaration on the action, so `input: string` means `input()` returns `string`. Only valid in plugin action
  statements that declare `input:`. May only appear once per action. Accepted `timestamp` input formats: RFC3339,
  with YYYY-MM-DD as a convenience fallback.
- `choose()` is semantically valid only in plugin runtime. Opens an interactive Quick Select picker, where the user
  fuzzy-filters candidate tasks and confirms one with Enter. Esc cancels the operation. The selected task's ID is the
  return value. `choose()` may only appear once per action. `choose()` and `input()` are mutually exclusive within a
  single action. Rejected for CLI, event-trigger, and time-trigger semantic runtimes.

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
- Example: `select filepath | run("some-app $1")` — pipes each task's absolute markdown file path as `$1`

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
