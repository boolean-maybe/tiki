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

Custom fields extend managed documents with project-specific data beyond the built-in frontmatter
fields (`title`, `status`, `priority`, and so on). Define them in `workflow.yaml` and they become
first-class citizens: usable in ruki queries, persisted in document frontmatter, and available across
all views. They apply to both workflow documents and plain documents — any managed document can carry
custom-field values.

Use cases include:

- tracking a sprint or milestone name
- adding an effort estimate or story-point alternative
- flagging a document with a boolean (e.g. `blocked`, `reviewed`)
- recording a deadline timestamp with time-of-day precision
- categorizing documents with a constrained set of values (enum)
- linking related documents beyond `dependsOn`

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

Fields without a `default:` key start empty on new tasks.

## Missing field behavior

When a custom field is not set on a task, ruki returns the typed zero value for that field's type:

| Field type    | Zero value         |
|---------------|--------------------|
| `text`        | `""` (empty string)|
| `integer`     | `0`                |
| `boolean`     | `false`            |
| `datetime`    | zero time          |
| `enum`        | `""` (empty string)|
| `stringList`  | `[]` (empty list)  |
| `taskIdList`  | `[]` (empty list)  |

This means `select where blocked = false` matches both tasks explicitly set to `false` and tasks
that never had the `blocked` field set. Use `is empty` / `is not empty` to distinguish:

```sql
-- tasks that explicitly have blocked set (to any value)
select where blocked is not empty

-- tasks where blocked was never set
select where blocked is empty
```

Note: for boolean and integer fields, the zero value (`false`, `0`) is also the `empty` value. An
explicitly stored `false` and a missing boolean field are indistinguishable at query time. If you
need the distinction, consider using an enum field with explicit values (e.g. `yes` / `no` / not set
via `empty`).
