# Validation And Errors

## Table of contents

- [Overview](#overview)
- [Validation layers](#validation-layers)
- [Structural statement and trigger errors](#structural-statement-and-trigger-errors)
- [Field and qualifier errors](#field-and-qualifier-errors)
- [Type and operator errors](#type-and-operator-errors)
- [Enum and list errors](#enum-and-list-errors)
- [Built-in and subquery errors](#built-in-and-subquery-errors)

## Overview

This page explains the errors you can get in Ruki. It covers syntax errors, unknown fields, type mismatches, invalid enum values, unsupported operators, and invalid trigger structure.

## Validation layers

Ruki has two distinct failure stages:

1. Parse-time failures
2. Validation-time failures

Parse-time failures happen when the input does not fit the grammar at all.

Examples:

```sql
drop where id = 1
update set status="done"
delete id = "x"
after update select
```

Validation-time failures happen after parsing, once the AST is checked against schema and semantic rules.

Examples:

```sql
select where foo = "bar"
create title="x" priority="high"
select where status < "done"
before update where new.status = "done"
```

## Structural statement and trigger errors

Statements:

- `create` must have at least one assignment
- `update` must have at least one assignment in `set`
- duplicate assignments to the same field are rejected

Triggers:

- `before` triggers must have `deny`
- `before` triggers must not have action or `run(...)`
- `after` triggers must have an action or `run(...)`
- `after` triggers must not have `deny`
- trigger actions must not be `select`

Examples:

```sql
before update where new.status = "done" update where id = old.id set status="done"
after update where new.status = "done" deny "no"
before update where new.status = "done"
after update where new.status = "done"
```

## Field and qualifier errors

Unknown field errors:

```sql
select where foo = "bar"
create title="x" foo="bar"
```

Qualifier misuse:

- `old.` and `new.` are invalid in standalone statements
- `old.` is invalid in create-trigger contexts
- `new.` is invalid in delete-trigger contexts
- both are invalid inside quantifier bodies

Examples:

```sql
select where old.status = "done"
create title=old.title
after create where old.status = "done" update where id = new.id set status="done"
before delete where new.status = "done" deny "x"
before update where dependsOn any old.status = "done" deny "blocked"
```

## Type and operator errors

Comparison mismatches:

```sql
select where priority = "high"
select where status = title
select where type = assignee
```

Unsupported operators:

```sql
select where title < "hello"
select where status < "done"
select where recurrence < recurrence
select where contains("a", "b") < contains("c", "d")
```

Invalid assignment types:

```sql
create title="x" priority="high"
create title="x" assignee=42
create title="x" status=title
update where id="x" set title=status
```

Invalid binary expressions:

```sql
create title="x" priority=1 + "a"
select where due = 2026-03-25 + 2026-03-20
create title="x" dependsOn=dependsOn + status
create title="x" dependsOn=dependsOn + tags
```

## Enum and list errors

Unknown enum values:

```sql
select where status = "nonexistent"
select where type = "nonexistent"
create title="x" status="nonexistent"
create title="x" type="nonexistent"
```

Invalid enum list membership:

```sql
select where status in ["done", "bogus"]
select where type in ["bug", "bogus"]
```

List strictness:

- list literals must be homogeneous
- `list<string>` fields reject non-string elements
- the special `list<ref>` assignment path accepts string-literal lists, but not arbitrary string fields or mixed element expressions

Invalid examples:

```sql
select where status in ["done", 1]
create title="x" tags=[1, 2]
create title="x" dependsOn=["TIKI-ABC123", title]
select where "bug" in title
select where status in dependsOn
select where tags any status = "done"
```

## Built-in and subquery errors

Unknown function:

```sql
select where foo(1) = 1
```

Argument count errors:

```sql
select where now(1) = now()
select where count() >= 1
select where contains("a") = "b"
select where user(1) = "bob"
```

Argument type errors:

```sql
select where blocks(priority) is empty
select where contains(1, "a") = contains("a", "b")
create title=call(42)
create title="x" due=next_date(42)
```

Subquery restrictions:

- only `count(...)` accepts a subquery
- bare subqueries elsewhere are rejected
- `count(...)` validates the subquery body recursively

Examples:

```sql
select where count(select where status = "done") >= 1
select where count(select where nosuchfield = "x") >= 1
select where select = 1
create title=select
```

