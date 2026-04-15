# Custom Fields Reference

## Table of contents

- [Overview](#overview)
- [Registration and loading](#registration-and-loading)
- [Naming constraints](#naming-constraints)
- [Type coercion rules](#type-coercion-rules)
- [Enum domain isolation](#enum-domain-isolation)
- [Validation rules](#validation-rules)
- [Persistence and round-trip behavior](#persistence-and-round-trip-behavior)
- [Schema evolution and stale data](#schema-evolution-and-stale-data)
- [Template defaults](#template-defaults)
- [Query behavior](#query-behavior)
- [Missing-field semantics](#missing-field-semantics)

## Overview

Custom fields extend tiki's built-in field catalog with user-defined fields declared in `workflow.yaml`. This reference covers the precise rules for how custom fields are loaded, validated, persisted, and queried — the behavioral contract behind the [Custom Fields](../custom-fields.md) user guide.

## Registration and loading

Custom field definitions are loaded from all `workflow.yaml` files across the three-tier search path (user config, project config, working directory). Files that define `fields:` but no `views:` still contribute field definitions.

Definitions from multiple files are merged by name:

- **identical redefinition** (same name, same type, same enum values in same order): silently accepted
- **conflicting redefinition** (same name, different type or values): fatal error naming both source files

After merging, fields are sorted by name for deterministic ordering and registered into the field catalog alongside built-in fields. Once registered, custom fields are available for ruki parsing, validation, and execution.

Registration happens during bootstrap before any task or template loading occurs.

## Naming constraints

Field names must:

- match the ruki identifier pattern (letters, digits, underscores; must start with a letter)
- not collide with ruki reserved keywords (`select`, `update`, `where`, `and`, `or`, `not`, `in`, `is`, `empty`, `order`, `by`, `asc`, `desc`, `set`, `create`, `delete`, `limit`, etc.)
- not collide with built-in field names, case-insensitively (`title`, `status`, `priority`, `tags`, `dependsOn`, etc.)
- not be `true` or `false` (reserved boolean literals)

Collision checks are case-insensitive: `Status` and `STATUS` both collide with the built-in `status`.

## Type coercion rules

When custom field values are read from frontmatter YAML, they are coerced to the expected type:

| Field type    | Accepted YAML values                          | Coercion behavior                                |
|---------------|-----------------------------------------------|--------------------------------------------------|
| `text`        | string                                        | pass-through                                     |
| `enum`        | string                                        | case-insensitive match against allowed values; stored in canonical casing |
| `integer`     | integer or decimal number                     | integer pass-through; decimal accepted only if it represents a whole number (e.g. `3.0` → `3`, but `1.5` → error) |
| `boolean`     | `true` / `false`                              | pass-through                                     |
| `datetime`    | timestamp or date string                      | native YAML timestamps pass through; strings parsed as RFC3339, with fallback to `YYYY-MM-DD` |
| `stringList`  | YAML list of strings                          | each element must be a string                    |
| `taskIdList`  | YAML list of strings                          | each element coerced to uppercase, whitespace trimmed, empty entries dropped |

## Enum domain isolation

Each enum field maintains its own independent set of allowed values. Two enum fields never share a domain, even if their values happen to overlap.

This isolation is enforced at three levels:

- **assignment**: `set severity = category` is rejected even if both are enum fields
- **comparison**: `where severity = category` is rejected (comparing different enum domains)
- **in-expression**: string literals in an `in` list are validated against the specific enum field's allowed values

Enum comparison is case-insensitive: `category = "Backend"` matches a stored `"backend"`.

## Validation rules

Custom fields follow the same validation pipeline as built-in fields:

- **type compatibility**: assignments and comparisons are type-checked (e.g. you cannot assign a string to an integer field, or compare an enum field with an integer literal)
- **enum value validation**: string literals assigned to or compared against an enum field must be in that field's allowed values
- **ordering**: custom fields of orderable types (`text`, `integer`, `boolean`, `datetime`, `enum`) can appear in `order by` clauses; list types (`stringList`, `taskIdList`) are not orderable

## Persistence and round-trip behavior

Custom fields are stored in task file frontmatter alongside built-in fields. When a task is saved:

- custom fields appear after built-in fields, sorted alphabetically by name
- values that look ambiguous in YAML (e.g. a text field containing `"true"`, `"42"`, or `"2026-05-15"`) are quoted to prevent YAML type coercion from corrupting them on reload

A save-then-load cycle preserves custom field values exactly. This holds as long as:

- the field definitions in `workflow.yaml` have not changed between save and load
- enum values use canonical casing (enforced automatically by coercion)
- timestamps round-trip through RFC3339 format

## Schema evolution and stale data

When `workflow.yaml` changes — fields renamed, enum values added or removed, field types changed — existing task files may contain values that no longer match the current schema. tiki handles this gracefully:

### Removed fields

If a frontmatter key no longer matches any registered custom field, it is preserved as an **unknown field**. Unknown fields survive load-save round-trips: they are written back to the file exactly as found. This allows manual cleanup or re-registration without data loss.

### Stale enum values

If a task file contains an enum value that is no longer in the field's allowed values list (e.g. `severity: critical` after `critical` was removed from the enum), the value is **demoted to an unknown field** with a warning. The task still loads and remains visible in views. The stale value is preserved in the file for repair.

### Type mismatches

If a value cannot be coerced to the field's current type (e.g. a text value `"not_a_number"` in a field that was changed to `integer`), the same demotion-to-unknown behavior applies: the task loads, the value is preserved, a warning is logged.

### General principle

tiki reads leniently and writes strictly. On load, unrecognized or incompatible values are preserved rather than rejected. On save, values are validated against the current schema.

## Template defaults

Custom fields in `new.md` templates follow the same coercion and validation rules as task files. If a template contains a value that cannot be coerced (e.g. a type mismatch), the invalid field is dropped with a warning and the template otherwise loads normally.

Template custom field values are copied into new tasks created via `create` statements or the new-task UI flow.

## Query behavior

Custom fields behave identically to built-in fields in ruki queries:

- usable in `where`, `order by`, `set`, `create`, and `select` field lists
- support `is empty` / `is not empty` checks
- support `in` / `not in` for list membership
- list-type fields support `+` (append) and `-` (remove) operations
- quantifiers (`any ... where`, `all ... where`) work on custom `taskIdList` fields

### Unset list fields

Unset custom list fields (`stringList`, `taskIdList`) behave the same as empty built-in list fields (`tags`, `dependsOn`). They return an empty list, not an absent value. This means:

- `"x" in labels` evaluates to `false` (not an error)
- `labels + ["new"]` produces `["new"]` (not an error)
- `labels is empty` evaluates to `true`

## Missing-field semantics

When a custom field has never been set on a task, ruki returns the typed zero value:

| Field type    | Zero value         |
|---------------|--------------------|
| `text`        | `""` (empty string)|
| `integer`     | `0`                |
| `boolean`     | `false`            |
| `datetime`    | zero time          |
| `enum`        | `""` (empty string)|
| `stringList`  | `[]` (empty list)  |
| `taskIdList`  | `[]` (empty list)  |

Setting a field to `empty` removes it entirely, making it indistinguishable from a field that was never set.

### Distinguishability by type

**Enum fields** preserve the missing-vs-set distinction: `""` is not a valid enum member, so `category is empty` only matches tasks where the field was never set (or was cleared). A task with `category = "frontend"` is never empty.

**Boolean and integer fields** do not preserve this distinction: `false` and `0` are both the zero value and the `empty` value. If you need to tell "never set" from "explicitly false" or "explicitly zero", use an enum field with named values (e.g. `yes` / `no`) instead.

### Worked examples

Suppose `blocked` is a custom boolean field:

| Query                              | never set | `false` | `true` |
|------------------------------------|-----------|---------|--------|
| `select where blocked = true`      | no        | no      | yes    |
| `select where blocked = false`     | yes       | yes     | no     |
| `select where blocked is empty`    | yes       | yes     | no     |
| `select where blocked is not empty`| no        | no      | yes    |

Suppose `category` is a custom enum field with values `[frontend, backend, infra]`:

| Query                                | never set | `"frontend"` |
|--------------------------------------|-----------|--------------|
| `select where category = "frontend"` | no        | yes          |
| `select where category is empty`     | yes       | no           |
| `select where category is not empty` | no        | yes          |
