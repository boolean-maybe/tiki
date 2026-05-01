---
name: tiki
description: view, create, update, delete workflow documents (tasks) and manage dependencies via ruki
allowed-tools: Read, Grep, Glob, Write, Bash(tiki exec:*), Bash(git log:*), Bash(git blame:*)
---

# tiki

A `tiki` is a **workflow document** (task) in the unified `.doc/` workspace — a Markdown file with
frontmatter that declares workflow fields such as `status`, `type`, `priority`, and `points`.

- Every managed document has a bare 6-character uppercase alphanumeric `id` (e.g. `X7F4K2`) stored in the
  `id:` frontmatter field. The filename is not identity — the frontmatter is. Legacy `TIKI-` prefixed ids
  are not recognized: they fail to load and must be edited to the bare form manually.
- Documents live under `.doc/` at any depth. The default location for new documents is `.doc/<ID>.md`, but
  they can be moved into subfolders without breaking references (`[[ID]]` wikilinks and `dependsOn` entries
  are id-based).
- Use this skill for operations on workflow documents (anything with `status:`, `type:`, `priority:`, or
  `points:` in frontmatter). For plain documents (only `id:` and `title:`), use the `doki` skill.

All CRUD operations go through `tiki exec '<ruki-statement>'`. This handles validation, triggers, file
persistence, and git staging automatically. Never manually edit document files or run `git add` / `git rm`
— `tiki exec` does it all.

For full `ruki` syntax, see `.doc/doki/doc/ruki/`.

## Field reference

| Field | Type | Notes |
|---|---|---|
| `id` | `id` | immutable, auto-generated bare 6-char uppercase alphanumeric (e.g. `X7F4K2`) |
| `title` | `string` | required on create |
| `description` | `string` | markdown body content |
| `status` | `status` | from `workflow.yaml`, default from the status with `default: true` |
| `type` | `type` | bug, feature, task, story, epic — keys from `workflow.yaml` |
| `priority` | `int` | 1–5 (1=high, 5=low), default 3 |
| `points` | `int` | story points 0–10 (0 = unestimated) |
| `assignee` | `string` | |
| `tags` | `list<string>` | |
| `dependsOn` | `list<ref>` | list of document ids (bare form, e.g. `["ABC123", "DEF456"]`) |
| `due` | `date` | `YYYY-MM-DD` format |
| `recurrence` | `recurrence` | cron: `0 0 * * *` (daily), `0 0 * * MON` (weekly), `0 0 1 * *` (monthly) |
| `createdBy` | `string` | immutable |
| `createdAt` | `timestamp` | immutable |
| `updatedAt` | `timestamp` | immutable |
| `filepath` | `string` | synthetic — the document's absolute path on disk |

Priority descriptions: 1=high, 2=medium-high, 3=medium, 4=medium-low, 5=low.
Valid statuses and types come from the project's `workflow.yaml`.

## Query

```sh
tiki exec 'select'                                                    # all managed documents (plain + workflow)
# workflow documents — ANY of the 9 workflow frontmatter keys are present
tiki exec '
  select where has(status) or has(type) or has(priority) or has(points) or has(tags)
            or has(dependsOn) or has(due) or has(recurrence) or has(assignee)
'
tiki exec 'select title, status'                                      # field projection
tiki exec 'select id, title where status = "done"'                    # filter
tiki exec 'select where "bug" in tags order by priority'              # tag filter + sort
tiki exec 'select where due < now()'                                  # overdue
tiki exec 'select where due - now() < 7day'                           # due within 7 days
tiki exec 'select where dependsOn any status != "done"'               # blocked tasks
tiki exec 'select where assignee = user()'                            # my tasks
```

Output is an ASCII table (use `--format json` for scripting). To read a document's full markdown body,
either project it via `select description where id = "ABC123"` or read the file directly — get the path
with `select filepath where id = "ABC123"`.

**Scope note.** Bare `select` iterates every managed document — plain docs and workflow docs alike.
Predicates on absent workflow fields (e.g. `status = "backlog"`) evaluate false for plain docs, so most
workflow queries are naturally scoped. But for queries that could match plain docs unexpectedly (e.g.
projecting only `id, title`), scope explicitly.

The document classifier treats any of **nine** frontmatter keys as workflow: `status`, `type`,
`priority`, `points`, `tags`, `dependsOn`, `due`, `recurrence`, `assignee`. Checking only
`has(status)` would misclassify a document that uses `priority:` or `dependsOn:` alone, so the
classifier form chains all nine with `or`:

```
has(status) or has(type) or has(priority) or has(points) or has(tags)
or has(dependsOn) or has(due) or has(recurrence) or has(assignee)
```

Apply that clause when scoping bulk mutations or filters that need to match the classifier exactly.

## Create

```sh
tiki exec 'create title="Fix login"'                                             # minimal
tiki exec 'create title="Fix login" priority=2 status="ready" tags=["bug"]'      # full
tiki exec 'create title="Review" due=2026-04-01 + 2day'                          # date arithmetic
tiki exec 'create title="Sprint review" recurrence="0 0 * * MON"'                # recurrence
```

Output: `created <ID>` (bare 6-char id).
Defaults: `status` from the `default: true` status in `workflow.yaml`, `type` from the `default: true`
type (or the first type), `priority` 3.

### Create from file

When asked to create a task from a file:
1. Read the source file
2. Summarize its content into a short title
3. Use the file content as the description:
   ```sh
   tiki exec 'create title="Summary of file" description="<escaped content>"'
   ```
Escape double quotes in the content with backslash.

## Update

```sh
tiki exec 'update where id = "X7F4K2" set status="done"'                   # status change
tiki exec 'update where id = "X7F4K2" set priority=1'                      # priority
tiki exec 'update where status = "ready" set status="cancelled"'           # bulk update
tiki exec 'update where id = "X7F4K2" set tags=tags + ["urgent"]'          # add tag
tiki exec 'update where id = "X7F4K2" set due=2026-04-01'                  # set due date
tiki exec 'update where id = "X7F4K2" set due=empty'                       # clear due date
tiki exec 'update where id = "X7F4K2" set recurrence="0 0 * * MON"'        # set recurrence
tiki exec 'update where id = "X7F4K2" set recurrence=empty'                # clear recurrence
```

Output: `updated N documents`.

Assigning a workflow field (`status`, `priority`, `points`, `dependsOn`) to a plain document promotes it to
a workflow document. Use this when a note needs to become a trackable task.

### Implement

When asked to implement a task and the user approves implementation, set its status to `review`:
```sh
tiki exec 'update where id = "X7F4K2" set status="review"'
```

## Delete

```sh
tiki exec 'delete where id = "X7F4K2"'                          # by ID
tiki exec 'delete where status = "cancelled"'                    # bulk
```

Output: `deleted N documents`. Delete works on any document regardless of workflow participation.

## Dependencies

```sh
# view a document's dependencies
tiki exec 'select id, title, status, dependsOn where id = "X7F4K2"'

# find what depends on a document (reverse lookup)
tiki exec 'select id, title where "X7F4K2" in dependsOn'

# find blocked tasks (any dependency not done)
tiki exec 'select id, title where dependsOn any status != "done"'

# add a dependency (bare id)
tiki exec 'update where id = "X7F4K2" set dependsOn=dependsOn + ["ABC123"]'

# remove a dependency
tiki exec 'update where id = "X7F4K2" set dependsOn=dependsOn - ["ABC123"]'
```

`dependsOn` entries must be bare 6-char uppercase ids. The legacy `TIKI-ABC123` form is rejected.

## Provenance

`ruki` does not access git history. Use git commands for authorship questions — but first resolve the
document path from its id, since files can live anywhere under `.doc/`:

```sh
# get the document's path
path=$(tiki exec --format json 'select filepath where id = "X7F4K2"' | jq -r '.[0].filepath')

# who created this document
git log --follow --diff-filter=A -- "$path"

# who last edited this document
git blame "$path"
```

Created timestamp and author are also available via:
```sh
tiki exec 'select createdAt, createdBy where id = "X7F4K2"'
```

## Important

- `tiki exec` handles `git add` and `git rm` automatically — never do manual git staging for managed documents
- Never commit without user permission
- Always use bare ids (`X7F4K2`), never the legacy `TIKI-X7F4K2` form
- Exit codes: 0 = ok, 2 = usage error, 3 = startup failure, 4 = query error
