# Types And Values

## Table of contents

- [Overview](#overview)
- [Value types](#value-types)
- [Field catalog](#field-catalog)
- [Literals](#literals)
- [The empty literal](#the-empty-literal)
- [Enum normalization](#enum-normalization)
- [List constraints](#list-constraints)
- [Type notes](#type-notes)

## Overview

This page explains the value types used in `ruki`. You do not write types explicitly. `ruki` works them out from the values, expressions, built-in functions, and tiki fields you use.

## Value types

`ruki` uses these value types:

| Type | Meaning |
|---|---|
| `string` | plain string values |
| `int` | integer values such as `priority` and `points` |
| `date` | day-level date values |
| `timestamp` | timestamp values such as `createdAt` and `updatedAt` |
| `duration` | duration literals and date or timestamp differences |
| `bool` | boolean result type (reserved, not currently produced by any expression) |
| `id` | tiki identifier |
| `ref` | tiki reference |
| `recurrence` | recurrence value |
| `list<string>` | list of strings |
| `list<ref>` | list of references |
| `status` | workflow status enum |
| `type` | workflow tiki-type enum |

## Field catalog

The workflow field catalog exposes these fields to `ruki`:

| Field | Type |
|---|---|
| `id` | `id` |
| `title` | `string` |
| `description` | `string` |
| `status` | `status` |
| `type` | `type` |
| `tags` | `list<string>` |
| `dependsOn` | `list<ref>` |
| `due` | `date` |
| `recurrence` | `recurrence` |
| `assignee` | `string` |
| `priority` | `int` |
| `points` | `int` |
| `createdBy` | `string` |
| `createdAt` | `timestamp` |
| `updatedAt` | `timestamp` |

## Literals

Implemented literal forms:

| Form | Example | Inferred type |
|---|---|---|
| string literal | `"Fix login"` | `string` |
| int literal | `2` | `int` |
| date literal | `2026-03-25` | `date` |
| duration literal | `2day` | `duration` |
| list literal | `["bug", "frontend"]` | usually `list<string>` |
| empty literal | `empty` | context-sensitive |

Qualified and unqualified references are not literals, but they participate in type inference:

- `status` resolves through the schema
- `old.status` or `new.status` resolve through the same schema, then pass qualifier checks

## The empty literal

`empty` is a special context-sensitive literal. Its type depends on where you use it.

Implemented behavior:

- `empty` can be assigned to most field types
- `title`, `status`, `type`, and `priority` reject `empty` assignment — these fields are required
- `empty` can be compared against any typed expression
- `is empty` and `is not empty` are allowed for any expression type

Examples:

```sql
create title="x" assignee=empty
create title="x" priority=empty
select where assignee = empty
select where due is empty
```

## Enum normalization

`status` and `type` are special:

- they have dedicated semantic types
- they accept validated string literals
- comparisons against enum fields are stricter than generic string comparisons

`status`

- normalized through the injected schema
- production normalization lowercases, trims, and converts `-` and space to `_`
- recognized values depend on the workflow status registry

`type`

- validated through the injected schema against the `types:` section of `workflow.yaml`
- production normalization lowercases, trims, and removes separators
- the bundled kanban workflow ships with `story`, `bug`, `spike`, and `epic`
- type keys must be canonical (matching normalized form); aliases are not supported
- unknown type values are rejected — no silent fallback

Examples:

```sql
select where status = "done"
select where type = "bug"
create title="x" status="done"
create title="x" type="feature"
```

## List constraints

List rules are intentionally strict:

- list literals must be homogeneous
- non-empty lists infer their type from the first element
- empty list literals default to `list<string>` until context narrows them
- `id` and `ref` elements cause a list literal to be treated as `list<ref>`

Important edge cases:

- `dependsOn=["TIKI-ABC123"]` is valid because a string-literal list can be assigned to `list<ref>`
- `dependsOn=["TIKI-ABC123", title]` is invalid because `list<ref>` assignment only permits literal string elements in that special case
- `tags=[1, 2]` is invalid because `tags` is `list<string>`

Examples:

```sql
create title="x" tags=["bug", "frontend"]
create title="x" dependsOn=["TIKI-ABC123", "TIKI-DEF456"]
create title="x" dependsOn=[]
```

Invalid examples:

```sql
create title="x" tags=[1, 2]
create title="x" dependsOn=["TIKI-ABC123", title]
select where status in ["done", 1]
```

## Type notes

- `string`, `status`, `type`, `id`, and `ref` are treated as string-like in some comparison and concatenation paths, but they are not interchangeable everywhere.
- Membership checks are stricter than general comparison compatibility. For `in` and `not in` with list collections, only exact type matches count, except that `id` and `ref` are treated as compatible with each other. When the right side is a `string` field, `in` performs a substring check — both sides must be `string` type (not `status`, `type`, `id`, or `ref`).
- Enum fields reject non-literal field references in assignments such as `status=title` or `type=title`.
- The exact accepted status values depend on runtime workflow configuration, while the accepted type values depend on the type registry supplied to the parser.
