# Examples

## Table of contents

- [Overview](#overview)
- [Simple statements](#simple-statements)
- [Conditions and lists](#conditions-and-lists)
- [Functions and dates](#functions-and-dates)
- [Before triggers](#before-triggers)
- [After triggers](#after-triggers)
- [Invalid examples](#invalid-examples)

## Overview

The examples show common patterns, useful combinations, and a few edge cases.

## Simple statements

Select all tikis:

```sql
select
```

Select with a basic filter:

```sql
select where status = "done" and priority <= 2
```

Select specific fields:

```sql
select title, status
select id, title where status = "done"
select * where priority <= 2
select title, status where "bug" in tags order by priority
```

Select with ordering:

```sql
select order by priority
select where status = "done" order by updatedAt desc
select where "bug" in tags order by priority asc, createdAt desc
```

Create a tiki:

```sql
create title="Fix login" priority=2 status="ready" tags=["bug"]
```

Update matching tikis:

```sql
update where status = "ready" and "sprint-3" in tags set status="cancelled"
```

Delete matching tikis:

```sql
delete where status = "cancelled" and "old" in tags
```

## Conditions and lists

Check whether a tag is present:

```sql
select where "bug" in tags
```

Check whether one tiki depends on another:

```sql
select where id in dependsOn
```

Check whether status is one of several values:

```sql
select where status in ["done", "cancelled"]
```

Check whether dependencies match a condition:

```sql
select where dependsOn any status != "done"
select where dependsOn all status = "done"
```

Boolean grouping:

```sql
select where not (status = "done" or priority = 1)
```

## Functions and dates

Count matching tikis:

```sql
select where count(select where status = "done") >= 1
```

Current user:

```sql
select where assignee = user()
```

Compare timestamps:

```sql
select where updatedAt < now()
select where updatedAt - createdAt > 1day
```

Calculate a date from recurrence:

```sql
create title="x" due=next_date(recurrence)
```

Add to or remove from a list:

```sql
create title="x" tags=tags + ["needs-triage"]
create title="x" dependsOn=dependsOn - ["TIKI-ABC123"]
```

Use `call(...)` in a value:

```sql
create title=call("echo hi")
```

## Before triggers

Block completion when dependencies remain open:

```sql
before update where new.status = "done" and dependsOn any status != "done" deny "cannot complete tiki with open dependencies"
```

Require a description for high-priority work:

```sql
before update where new.priority <= 2 and new.description is empty deny "high priority tikis need a description"
```

Limit how many in-progress tikis someone can have:

```sql
before update where new.status = "in progress" and count(select where assignee = new.assignee and status = "in progress") >= 3 deny "WIP limit reached for this assignee"
```

## After triggers

Auto-assign urgent new work:

```sql
after create where new.priority <= 2 and new.assignee is empty update where id = new.id set assignee="booleanmaybe"
```

Create the next recurring tiki:

```sql
after update where new.status = "done" and old.recurrence is not empty create title=old.title priority=old.priority tags=old.tags recurrence=old.recurrence due=next_date(old.recurrence) status="ready"
```

Clear recurrence on the completed source tiki:

```sql
after update where new.status = "done" and old.recurrence is not empty update where id = old.id set recurrence=empty
```

Clean up reverse dependencies on delete:

```sql
after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]
```

Run a command after an update:

```sql
after update where new.status = "in progress" and "claude" in new.tags run("claude -p 'implement tiki " + old.id + "'")
```

## Invalid examples

Unknown field:

```sql
select where foo = "bar"
```

Wrong field value type:

```sql
create title="x" priority="high"
```

Using the wrong operator with status:

```sql
select where status < "done"
```

Using `any` on the wrong kind of field:

```sql
select where tags any status = "done"
```

Using `old.` where it is not allowed:

```sql
select where old.status = "done"
```

A list with mixed value types:

```sql
select where status in ["done", 1]
```

Trigger is missing the right action:

```sql
after update where new.status = "done" deny "no"
```

Non-string `run(...)` command:

```sql
after update run(1 + 2)
```

Ordering by a non-orderable field:

```sql
select order by tags
select order by dependsOn
```

Order by inside a subquery:

```sql
select where count(select where status = "done" order by priority) >= 1
```
