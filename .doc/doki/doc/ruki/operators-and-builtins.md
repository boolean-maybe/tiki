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
- `list<string> + string` and `list<string> + list<string>` yield `list<string>`
- `list<ref> + id-or-ref-compatible value` and `list<ref> + list<ref>` yield `list<ref>`
- `date + duration` yields `date`
- `timestamp + duration` yields `timestamp`

`-`

- `int - int` yields `int`
- `list<string> - string` and `list<string> - list<string>` yield `list<string>`
- `list<ref> - id-or-ref-compatible value` and `list<ref> - list<ref>` yield `list<ref>`
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
| `call(...)` | `string` | exactly 1 | argument must be `string` |
| `user()` | `string` | 0 | no additional validation |

Examples:

```sql
select where count(select where status = "done") >= 1
select where updatedAt < now()
create title="x" due=next_date(recurrence)
select where blocks(id) is empty
select where id() in dependsOn
create title=call("echo hi")
select where assignee = user()
```

Runtime notes:

- `id()` is semantically valid only in plugin runtime.
- When a validated statement uses `id()`, plugin execution must provide a non-empty selected task ID.
- `id()` is rejected for CLI, event-trigger, and time-trigger semantic runtimes.
- `call(...)` is currently rejected by semantic validation.

`run(...)`

- not a normal expression built-in
- only valid as the top-level action of an `after` trigger
- its command expression must validate as `string`

Example:

```sql
after update where new.status = "in progress" run("echo hello")
```

## Shell-related forms

`ruki` includes two shell-related forms:

- `call(...)` as a string-returning expression
- `run(...)` as an `after`-trigger action

- `call(...)` returns a string
- `run(...)` is used as the top-level action of an `after` trigger
