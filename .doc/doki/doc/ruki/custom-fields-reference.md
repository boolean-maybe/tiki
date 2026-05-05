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

Custom field definitions come from the single highest-priority `workflow.yaml` (see [Configuration: Precedence](../config.md#precedence)). A missing `fields:` section means no custom fields are registered.

Fields are sorted by name for deterministic ordering and registered into the field catalog alongside built-in fields. Once registered, custom fields are available for ruki parsing, validation, and execution.

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
| `stringList`  | YAML list of strings                          | strings only; trim, drop empty, dedupe |
| `taskIdList`  | YAML list of strings                          | uppercase IDs; trim, drop empty, dedupe |

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

## Field defaults

Custom fields can declare a `default:` value in `workflow.yaml`. Default values are validated
against the field's type and enum constraints during workflow load — invalid defaults are hard
errors. Valid defaults are copied into new tasks created via `create` statements or the
new-task UI flow. Fields without a `default:` key start empty.

## Query behavior

Custom fields behave identically to built-in fields in ruki queries:

- usable in `where`, `order by`, `set`, `create`, and `select` field lists
- support `is empty` / `is not empty` checks
- support `in` / `not in` for list membership
- list-type fields support `+` (set union) and `-` (remove) operations
- quantifiers (`any ... where`, `all ... where`) work on custom `taskIdList` fields

### Unset list fields

Unset custom list fields (`stringList`, `taskIdList`) are absent — `has(labels)` is false until the
key is written. For most operations they behave as if empty:

- `"x" in labels` evaluates to `false` (not an error)
- `labels + ["new"]` produces `["new"]` (not an error); the assignment then writes `labels: [new]`
  to frontmatter, after which `has(labels)` becomes true
- `labels is empty` evaluates to `true` (absent is treated as empty)
- `labels = empty` evaluates to `true`

But `labels = []` (equality with a concrete empty list literal) is **false** on absent fields, since
that is a comparison against a value, not against `empty`. List arithmetic also does **not** delete
the key when the result is empty — `set labels = labels - ["only-tag"]` writes `labels: []` and
leaves `has(labels)` true. Use `set labels = empty` to actually remove the key.

## Missing-field semantics

Custom fields are **presence-aware** — a field is either present (its key appears in the tiki's
frontmatter) or absent. Comparisons against absent custom fields follow the same rules as
schema-known absent fields; see [Absent fields in semantics.md](semantics.md#absent-fields) for the
full set. The key facts for custom fields:

- `where <field> = <concrete-value>` is **false** on absent fields. `blocked = false` does **not**
  match tikis that never set `blocked`; only an explicit `blocked: false` matches.
- `where <field> != <concrete-value>` is **true** on absent fields. `blocked != true` matches both
  "explicitly false" and "never set".
- `where <field> is empty` and `where <field> = empty` are **true** on absent fields (the empty
  literal treats absent as empty).
- `where has(<field>)` is the only predicate that distinguishes "key present" from "key absent",
  regardless of value.

Setting a field to `empty` deletes it from frontmatter, so subsequent loads see it as absent.

### Distinguishability by type

**Enum fields** preserve the missing-vs-set distinction even through `is empty`: `""` is not a valid
enum member, so the only way `category is empty` is true is if the field is absent (or was cleared
to `empty`). A tiki with `category = "frontend"` is never empty.

**Boolean and integer fields** do not preserve the distinction through `is empty`: `false` and `0`
are zero values, so an explicit `blocked: false` and an absent `blocked` both satisfy `is empty`. To
tell "never set" from "explicitly false" or "explicitly zero", use `has(blocked)` — or model the
field as an enum with named values (e.g. `yes` / `no`).

### Worked examples

Suppose `blocked` is a custom boolean field:

| Query                              | never set | `false` | `true` |
|------------------------------------|-----------|---------|--------|
| `select where blocked = true`      | no        | no      | yes    |
| `select where blocked = false`     | no        | yes     | no     |
| `select where blocked != false`    | yes       | no      | yes    |
| `select where blocked is empty`    | yes       | yes     | no     |
| `select where blocked is not empty`| no        | no      | yes    |
| `select where has(blocked)`        | no        | yes     | yes    |
| `select where not has(blocked)`    | yes       | no      | no     |

Suppose `category` is a custom enum field with values `[frontend, backend, infra]`:

| Query                                | never set | `"frontend"` |
|--------------------------------------|-----------|--------------|
| `select where category = "frontend"` | no        | yes          |
| `select where category is empty`     | yes       | no           |
| `select where category is not empty` | no        | yes          |
