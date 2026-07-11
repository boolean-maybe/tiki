# Types And Values

## Table of contents

- [Overview](#overview)
- [Value types](#value-types)
- [Field catalog](#field-catalog)
- [Literals](#literals)
- [The empty literal](#the-empty-literal)
- [Enum fields](#enum-fields)
- [List constraints](#list-constraints)
- [Type notes](#type-notes)

## Overview

This page explains the value types used in `ruki`. You do not write types explicitly. `ruki` works them out from
the values, expressions, built-in functions, and tiki fields you use.

## Value types

`ruki` uses these value types:

| Type | Meaning |
|---|---|
| `string` | plain string values |
| `int` | integer values |
| `date` | day-level date values |
| `timestamp` | timestamp values such as `createdAt` and `updatedAt` |
| `duration` | duration literals and date or timestamp differences |
| `bool` | boolean result type (reserved, not currently produced by any expression) |
| `id` | bare document identifier (`^[A-Z0-9]{6}$`) |
| `ref` | bare document reference (same format as `id`) |
| `recurrence` | recurrence value |
| `list<string>` | list of strings |
| `list<ref>` | list of references |
| `enum` | workflow-declared enum |

## Field catalog

The runtime hardcodes a small set of system fields (`id`, `title`, `description`, `createdBy`,
`createdAt`, `updatedAt`, `filepath`). Every other field — including `status`, `type`, `priority`,
`points`, `tags`, and any project-specific fields — is declared in `workflow.yaml fields:` and joins
the same catalog at load time. From `ruki`'s perspective, all fields behave identically.

| Field | Type |
|---|---|
| `id` | `id` |
| `title` | `string` |
| `description` | `string` |
| `status` | enum (declared in `workflow.yaml`) |
| `type` | enum (declared in `workflow.yaml`) |
| `tags` | `list<string>` |
| `dependsOn` | `list<ref>` |
| `due` | `date` |
| `recurrence` | `recurrence` |
| `assignee` | `string` |
| `priority` | enum (declared in `workflow.yaml`) |
| `points` | enum (declared in `workflow.yaml`) |
| `createdBy` | `string` |
| `createdAt` | `timestamp` |
| `updatedAt` | `timestamp` |
| `filepath` | `string` |

Workflow fields declared as `type: user` also appear as `string` in ruki. The `user` type affects only the
detail editor; ruki has no separate user value type.

`filepath` is a synthetic, read-only field populated by the file-backed store. It holds the absolute path to the tiki's
markdown file for persisted tikis, and is an empty string for in-memory or unsaved tikis. It never appears in YAML
frontmatter and cannot be assigned via `create` or `update`.

The `filepath()` and `filepaths()` builtins (see
[operators-and-builtins.md](operators-and-builtins.md)) expose the same value for the currently selected tiki(s)
inside plugin actions, so authors can write `update where id = id() set ... | run("editor " + filepath())` without
threading a separate `select filepath` subquery.

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

Recurrence values have no bare literal form. Construct them with the `daily()`, `weekly("<weekday>")`, and
`monthly(<day>)` builtins (see [operators-and-builtins.md](operators-and-builtins.md)). The `empty` literal expresses
"no recurrence".

Qualified and unqualified references are not literals, but they participate in type inference:

- `category` resolves through the schema
- `old.category` or `new.category` resolve through the same schema, then pass qualifier checks

## The empty literal

`empty` is a special context-sensitive literal. Its type depends on where you use it.

Implemented behavior:

- `empty` can be assigned to most field types — the assignment deletes the key from frontmatter so
  subsequent loads see the field as absent
- `title` and enum fields reject `empty` assignment. The empty literal has no defined coercion to those
  types, so the runtime rejects `set <field> = empty` as a hard error. Enum fields can still be **absent**
  on sparse tikis — what is rejected is the explicit `empty` assignment, not absence itself.
- `empty` can be compared against any typed expression
- `is empty` and `is not empty` are allowed for any expression type

Examples:

```sql
create title="x" assignee=empty               -- sets assignee absent on the new tiki
update where id = "ABC123" set due=empty      -- clears the due date
select where assignee = empty                 -- matches absent or zero-valued assignee
select where due is empty                     -- same, with the is-empty predicate
```

## Enum fields

Every `type: enum` workflow field uses the same semantic type:

- string literals are validated against that field's declared values
- comparisons against enum fields are stricter than generic string comparisons
- each enum field has an independent value domain
- unknown values are rejected on writes; stale values loaded from disk remain visible for manual repair

Examples:

```sql
select where category = "done"
select where severity = "high"
create title="x" category="done"
create title="x" severity="high"
```

## List constraints

List rules are intentionally strict:

- list literals must be homogeneous
- non-empty lists infer their type from the first element
- empty list literals default to `list<string>` until context narrows them
- `id` and `ref` elements cause a list literal to be treated as `list<ref>`

Important edge cases:

- `dependsOn=["ABC123"]` is valid because a string-literal list can be assigned to `list<ref>`
- `dependsOn=["ABC123", title]` is invalid because `list<ref>` assignment only permits literal string elements in
  that special case
- `dependsOn=["TIKI-ABC"]` is invalid: references must be bare document IDs (`^[A-Z0-9]{6}$`), not the
  legacy TIKI-prefixed format
- `tags=[1, 2]` is invalid because `tags` is `list<string>`

Examples:

```sql
create title="x" tags=["bug", "frontend"]
create title="x" dependsOn=["ABC123", "DEF456"]
create title="x" dependsOn=[]
```

Invalid examples:

```sql
create title="x" tags=[1, 2]
create title="x" dependsOn=["ABC123", title]
create title="x" dependsOn=["TIKI-ABC"]      -- legacy prefixed ID rejected
select where status in ["done", 1]
```

## Type notes

- `string`, enum, `id`, and `ref` are treated as string-like in some comparison and concatenation
  paths, but they are not interchangeable everywhere.
- Membership checks are stricter than general comparison compatibility. For `in` and `not in` with list
  collections, only exact type matches count, except that `id` and `ref` are treated as compatible with each
  other. When the right side is a `string` field, `in` performs a substring check — both sides must be
  `string` type (not enum, `id`, or `ref`).
- Enum fields reject non-literal field references in assignments such as `category=title`.
- Each enum field accepts only the values declared for that field in `workflow.yaml`.
