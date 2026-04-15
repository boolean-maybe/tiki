---
name: tiki
description: view, create, update, delete tikis and manage dependencies
allowed-tools: Read, Grep, Glob, Write, Bash(tiki exec:*), Bash(git log:*), Bash(git blame:*)
---

# tiki

A tiki is a task stored as a Markdown file in `.doc/tiki/`.
Filename: `tiki-<6char>.md` (lowercase) → ID: `TIKI-<6CHAR>` (uppercase).
Example: `tiki-x7f4k2.md` → `TIKI-X7F4K2`

All CRUD operations go through `tiki exec '<ruki-statement>'`.
This handles validation, triggers, file persistence, and git staging automatically.
Never manually edit tiki files or run `git add`/`git rm` — `tiki exec` does it all.

For full `ruki` syntax, see `.doc/doki/doc/ruki/`.

## Field reference

| Field | Type | Notes |
|---|---|---|
| `id` | `id` | immutable, auto-generated `TIKI-XXXXXX` |
| `title` | `string` | required on create |
| `description` | `string` | markdown body content |
| `status` | `status` | from `workflow.yaml`, default `backlog` |
| `type` | `type` | bug, feature, task, story, epic |
| `priority` | `int` | 1–5 (1=high, 5=low), default 3 |
| `points` | `int` | story points 1–10 |
| `assignee` | `string` | |
| `tags` | `list<string>` | |
| `dependsOn` | `list<ref>` | list of tiki IDs |
| `due` | `date` | `YYYY-MM-DD` format |
| `recurrence` | `recurrence` | cron: `0 0 * * *` (daily), `0 0 * * MON` (weekly), `0 0 1 * *` (monthly) |
| `createdBy` | `string` | immutable |
| `createdAt` | `timestamp` | immutable |
| `updatedAt` | `timestamp` | immutable |

Priority descriptions: 1=high, 2=medium-high, 3=medium, 4=medium-low, 5=low.
Valid statuses are configurable — check `statuses:` in `~/.config/tiki/workflow.yaml`.

## Query

```sh
tiki exec 'select'                                                    # all tasks
tiki exec 'select title, status'                                      # field projection
tiki exec 'select id, title where status = "done"'                    # filter
tiki exec 'select where "bug" in tags order by priority'              # tag filter + sort
tiki exec 'select where due < now()'                                  # overdue
tiki exec 'select where due - now() < 7day'                           # due within 7 days
tiki exec 'select where dependsOn any status != "done"'               # blocked tasks
tiki exec 'select where assignee = user()'                            # my tasks
```

Output is an ASCII table. To read a tiki's full markdown body, use `Read` on `.doc/tiki/tiki-<id>.md`.

## Create

```sh
tiki exec 'create title="Fix login"'                                             # minimal
tiki exec 'create title="Fix login" priority=2 status="ready" tags=["bug"]'      # full
tiki exec 'create title="Review" due=2026-04-01 + 2day'                          # date arithmetic
tiki exec 'create title="Sprint review" recurrence="0 0 * * MON"'               # recurrence
```

Output: `created TIKI-XXXXXX`. Defaults: status from workflow.yaml (typically `backlog`), priority 3, type `story`.

### Create from file

When asked to create a tiki from a file:
1. Read the source file
2. Summarize its content into a short title
3. Use the file content as the description:
   ```sh
   tiki exec 'create title="Summary of file" description="<escaped content>"'
   ```
Escape double quotes in the content with backslash.

## Update

```sh
tiki exec 'update where id = "TIKI-X7F4K2" set status="done"'                   # status change
tiki exec 'update where id = "TIKI-X7F4K2" set priority=1'                       # priority
tiki exec 'update where status = "ready" set status="cancelled"'                  # bulk update
tiki exec 'update where id = "TIKI-X7F4K2" set tags=tags + ["urgent"]'           # add tag
tiki exec 'update where id = "TIKI-X7F4K2" set due=2026-04-01'                   # set due date
tiki exec 'update where id = "TIKI-X7F4K2" set due=empty'                        # clear due date
tiki exec 'update where id = "TIKI-X7F4K2" set recurrence="0 0 * * MON"'         # set recurrence
tiki exec 'update where id = "TIKI-X7F4K2" set recurrence=empty'                 # clear recurrence
```

Output: `updated N tasks`.

### Implement

When asked to implement a tiki and the user approves implementation, set its status to `review`:
```sh
tiki exec 'update where id = "TIKI-X7F4K2" set status="review"'
```

## Delete

```sh
tiki exec 'delete where id = "TIKI-X7F4K2"'                          # by ID
tiki exec 'delete where status = "cancelled"'                         # bulk
```

Output: `deleted N tasks`.

## Dependencies

```sh
# view a tiki's dependencies
tiki exec 'select id, title, status, dependsOn where id = "TIKI-X7F4K2"'

# find what depends on a tiki (reverse lookup)
tiki exec 'select id, title where "TIKI-X7F4K2" in dependsOn'

# find blocked tasks (any dependency not done)
tiki exec 'select id, title where dependsOn any status != "done"'

# add a dependency
tiki exec 'update where id = "TIKI-X7F4K2" set dependsOn=dependsOn + ["TIKI-ABC123"]'

# remove a dependency
tiki exec 'update where id = "TIKI-X7F4K2" set dependsOn=dependsOn - ["TIKI-ABC123"]'
```

## Provenance

`ruki` does not access git history. Use git commands for authorship questions:

```sh
# who created this tiki
git log --follow --diff-filter=A -- .doc/tiki/tiki-x7f4k2.md

# who last edited this tiki
git blame .doc/tiki/tiki-x7f4k2.md
```

Created timestamp and author are also available via:
```sh
tiki exec 'select createdAt, createdBy where id = "TIKI-X7F4K2"'
```

## Important

- `tiki exec` handles `git add` and `git rm` automatically — never do manual git staging for tikis
- Never commit without user permission
- Exit codes: 0 = ok, 2 = usage error, 3 = startup failure, 4 = query error
