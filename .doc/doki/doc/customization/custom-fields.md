# Custom Fields

## Table of contents

- [Overview](#overview)
- [Defining custom fields](#defining-custom-fields)
- [Field types](#field-types)
- [Enum fields](#enum-fields)
- [Using custom fields in ruki](#using-custom-fields-in-ruki)
- [Storage and frontmatter](#storage-and-frontmatter)
- [Templates](#templates)
- [Missing field behavior](#missing-field-behavior)

## Overview

Custom fields extend tikis with project-specific data beyond the schema-known frontmatter fields
(`title`, `status`, `priority`, and so on). Define them in `workflow.yaml` and they become first-class
citizens: usable in ruki queries, persisted in tiki frontmatter, and available across all views. Any
tiki can carry custom-field values, regardless of what other fields it has.

Use cases include:

- tracking a sprint or milestone name
- adding an effort estimate or story-point alternative
- flagging a tiki with a boolean (e.g. `blocked`, `reviewed`)
- recording a deadline timestamp with time-of-day precision
- categorizing tikis with a constrained set of values (enum)
- linking related tikis beyond `dependsOn`

## Defining custom fields

Add a `fields:` section to your `workflow.yaml`:

```yaml
fields:
  - name: sprint
    type: text
  - name: effort
    type: integer
  - name: blocked
    type: boolean
  - name: deadline
    type: datetime
  - name: category
    type: enum
    values:
      - frontend
      - backend
      - infra
      - docs
  - name: reviewers
    type: stringList
  - name: relatedTasks
    type: taskIdList
```

Field names must not collide with built-in field names or ruki reserved keywords.

Custom fields come from the `workflow.yaml` file (see [Configuration: Precedence](../config.md#precedence))

## Field types

| YAML type     | Description                           | ruki type        |
|---------------|---------------------------------------|------------------|
| `text`        | free-form string                      | `string`         |
| `integer`     | whole number                          | `int`            |
| `boolean`     | true or false                         | `bool`           |
| `datetime`    | timestamp (RFC3339 or YYYY-MM-DD)     | `timestamp`      |
| `enum`        | constrained string from `values` list | `enum`           |
| `stringList`  | set-like list of strings              | `list<string>`   |
| `taskIdList`  | set-like list of document id references| `list<ref>`      |

For list field types, values are normalized with set semantics:
- strings are trimmed
- empty entries are dropped
- duplicate entries are removed
- `taskIdList` entries are uppercased

## Enum fields

Enum fields require a `values:` list. Only those values are accepted when setting the field
(case-insensitive matching, canonical casing preserved). Attempting to assign a value outside the
list produces a validation error.

```yaml
fields:
  - name: severity
    type: enum
    values:
      - critical
      - major
      - minor
      - trivial
```

Enum domains are field-scoped: two different enum fields maintain independent value sets.
Cross-field enum assignment (e.g. `set severity = category`) is rejected even if the values happen
to overlap.

Non-enum fields must not include a `values:` list.

## Using custom fields in ruki

Custom fields work the same as built-in fields in all ruki contexts: `select`, `update`, `create`,
`order by`, `where`, and triggers.

### Filtering with select where

```sql
-- find blocked tasks
select where blocked = true

-- find tasks in a specific sprint
select where sprint = "sprint-7"

-- find critical tasks in the frontend category
select where severity = "critical" and category = "frontend"

-- find tasks with high effort
select where effort > 5

-- find tasks with a deadline before a date
select where deadline < 2026-05-01
```

### Updating with update set

For list fields, `+` behaves like set union and `-` removes matching values.

```sql
-- assign a sprint
update where id = id() set sprint="sprint-7"

-- mark as blocked
update where id = id() set blocked=true

-- set category and severity
update where id = id() set category="backend" severity="major"

-- clear a custom field (set to empty)
update where id = id() set sprint=empty

-- add a reviewer
update where id = id() set reviewers=reviewers + ["alice"]
```

### Ordering with order by

```sql
-- sort by effort descending
select where status = "ready" order by effort desc

-- sort by category, then priority
select where status = "backlog" order by category, priority

-- sort by deadline
select where deadline is not empty order by deadline
```

### Creating with custom field defaults

```sql
-- create a task with custom fields
create title="New feature" category="frontend" effort=3

-- create with enum and boolean
create title="Fix crash" severity="critical" blocked=false
```

### Plugin filters and actions

Custom fields integrate into view definitions in `workflow.yaml`:

```yaml
views:
  - name: Sprint Board
    kind: board
    key: "F5"
    lanes:
      - name: Current Sprint
        filter: select where sprint = "sprint-7" and status != "done" order by effort desc
        action: update where id = id() set sprint="sprint-7"
      - name: Next Sprint
        filter: select where sprint = "sprint-8" order by priority
        action: update where id = id() set sprint="sprint-8"
    actions:
      - key: "b"
        label: "Mark blocked"
        action: update where id = id() set blocked=true
```

## Storage and frontmatter

Custom fields are stored in document frontmatter alongside built-in fields:

```yaml
---
id: Q7XR2K
title: Implement search
type: story
status: inProgress
priority: 2
points: 3
tags:
    - search
sprint: sprint-7
blocked: false
category: backend
effort: 5
deadline: 2026-05-15T17:00:00Z
reviewers:
- alice
- bob
relatedTasks:
- ABC123
---
Search implementation details...
```

Custom fields appear after the built-in fields, sorted alphabetically by name.

On load, unknown frontmatter keys that are not registered custom fields are preserved as-is and
survive save-load round-trips. This allows workflow changes without losing data — see
[Schema evolution](../ruki/custom-fields-reference.md#schema-evolution-and-stale-data) for details.

## Field Defaults

Custom fields can declare a `default:` value directly in `workflow.yaml`. The value is
validated against the field's type and enum constraints during workflow load — invalid
defaults are hard errors, not silent fallbacks.

```yaml
fields:
  - name: severity
    type: enum
    values: [critical, high, medium, low]
    default: medium
  - name: blocked
    type: boolean
    default: false
  - name: escalations
    type: integer
    default: 0
```

Fields without a `default:` key are absent on new tikis.

## Missing field behavior

Custom fields are presence-aware, just like the schema-known fields. A custom field is either
*present* (its key appears in the tiki's frontmatter) or *absent*. Comparisons against absent custom
fields follow the same rules as schema-known absent fields — see
[Absent fields in semantics.md](../ruki/semantics.md#absent-fields) for the complete rule set. The
practical consequences for custom fields:

- `where <field> = <concrete-value>` is **false** on absent fields. `where blocked = false` does
  **not** match tikis that never had `blocked` set; it only matches tikis whose frontmatter literally
  wrote `blocked: false`. Use `not has(blocked)` to match "blocked never set" and
  `where blocked = false or not has(blocked)` to match "not currently blocked".
- `where <field> != <concrete-value>` is **true** on absent fields. `where blocked != true` matches
  both "explicitly false" and "never set".
- `where <field> = empty` and `where <field> is empty` are **true** on absent fields (absent is
  treated as empty for the empty-literal forms). The flip side is that an explicit `blocked: false`
  also satisfies `is empty`, since `false` is the zero value for booleans — `is empty` cannot
  distinguish "absent" from "present-but-zero".
- `where has(<field>)` is the only predicate that reliably distinguishes "key present" from "key
  absent", regardless of value. Use it whenever the difference matters.

The zero values produced when projecting an absent custom field are:

| Field type    | Projected as      |
|---------------|-------------------|
| `text`        | `""` (empty cell) |
| `integer`     | empty cell / `null` |
| `boolean`     | empty cell / `null` |
| `datetime`    | empty cell / `null` |
| `enum`        | `""` (empty cell) |
| `stringList`  | `[]` (empty list) |
| `taskIdList`  | `[]` (empty list) |

For boolean and integer fields the zero value (`false`, `0`) is also the `empty` value, so an
explicitly stored `false` and an absent boolean field are still indistinguishable through `is empty`.
Use `has(<field>)` if you need the distinction, or model the field as an enum with explicit values
(e.g. `yes` / `no` / unset).
