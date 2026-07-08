---
name: tiki
description: view, create, update, delete tikis (tasks, docs, notes) and manage dependencies via ruki
allowed-tools: Read, Grep, Glob, Write, WebFetch, Bash(tiki exec:*), Bash(git log:*), Bash(git blame:*), Bash(jq:*)
---

# tiki

A `tiki` is a Markdown file in the project workspace — the current working directory — identified by a
bare `id` in its frontmatter and carrying an open field map. Only `id` and `title` are intrinsic;
**every other field is defined by the `workflow.yaml` that applies to the current project**. There are no
hard-coded workflow fields — the names, types, allowed values, and defaults below are those of the
built-in workflow, which is what a project gets when no `workflow.yaml` overrides it. A different
resolved workflow declares a different field set. All fields beyond `id` and `title` are optional.

The applicable `workflow.yaml` is resolved by the project's lookup chain (user config
`~/.config/tiki/workflow.yaml`, then a `workflow.yaml` in the cwd, last match wins; a built-in workflow
when neither exists). **The field names, types, values, and defaults in this skill describe that
built-in workflow — they are examples, not guarantees.** The project you are working in may define
different ones. Before relying on any field or value below, read the applicable `workflow.yaml` (check the
cwd and `~/.config/tiki/` for one; if neither exists the built-in applies and this skill's tables hold).

- A tiki's identity is its `id:` frontmatter field, independent of the filename: a bare 6-character
  uppercase alphanumeric value (e.g. `X7F4K2`). Locate, reference, and update a tiki by this `id`, not by
  its filename — a file may be renamed or moved without changing which tiki it is. The `id:` value must be
  exactly 6 characters of `[A-Z0-9]`; anything else (wrong length, lowercase, or a prefix/separator such
  as `x7f4k2` or `task-1234`) fails to load and must be edited to the valid form manually.
- Tikis are read from and written to the current working directory — the cwd is the scan/write root, and
  no setup step is needed to start using it. Creating a tiki (via `tiki exec`, below) writes a new file
  named after a slug of its title: `<cwd>/<slug>.md` (e.g. title "Fix Login Bug" → `fix-login-bug.md`),
  with a numeric suffix on collision (`fix-login-bug-2.md`, …). Tikis may live at any depth under the cwd
  and can be moved into subfolders without breaking references (`[[ID]]` wikilinks and `dependsOn` entries
  are id-based). A `.doc/` subdirectory, if the project has one, is also scanned.
- Use this skill for every tiki, whether it carries workflow-declared fields (in the built-in workflow:
  `status:`, `type:`, `priority:`, `points:`, `dependsOn:`, …) or is a notes-only document with just
  `id:` and `title:`. Workflow tikis appear on board / list views; notes-only tikis are reachable by id
  and path, render in wiki and detail views, and fall outside any view whose lane filter requires
  workflow-declared fields. To turn a notes-only tiki into a tracked one, just assign a workflow field
  (e.g. `set status = "..."`) — that writes the key into its frontmatter and it starts appearing on the
  relevant views.

All CRUD operations go through `tiki exec '<ruki-statement>'`. `ruki` is the query-and-mutation language
`tiki exec` accepts: SQL-like `select` / `create` / `update` / `delete` statements over tikis. The
statements in this skill cover routine work; run `tiki exec --help` for the invocation reference. For full
`ruki` syntax (operators, builtins, custom-field and extraction rules), fetch the language reference at
https://github.com/boolean-maybe/tiki/blob/main/docs/ruki/index.md and the pages it links.
Running a statement handles validation, triggers, file persistence, and git staging automatically. Never
manually edit tiki files or run `git add` / `git rm` for tracked workflow tikis — `tiki exec` does it
all. (Notes-only tikis with no workflow fields can be edited directly with Read/Write/Edit; the
frontmatter `id:` must be preserved unchanged. Resolve their path from the id via
`select filepath where id = "..."` rather than assuming a filename.)

## Field reference

### Intrinsic fields (every project)

These seven fields exist regardless of workflow and cannot be redeclared by a `workflow.yaml`:

| Field | Type | Notes |
|---|---|---|
| `id` | `id` | immutable, auto-generated bare 6-char uppercase alphanumeric (e.g. `X7F4K2`) |
| `title` | `string` | required on create |
| `description` | `string` | markdown body content |
| `createdBy` | `string` | immutable |
| `createdAt` | `timestamp` | immutable |
| `updatedAt` | `timestamp` | immutable |
| `filepath` | `string` | synthetic — the tiki's absolute path on disk |

### Workflow fields — EXAMPLE ONLY, not guaranteed

> ⚠️ **The table below is an example, not a specification.** Every field in it — including its name,
> type, allowed values, and default — is declared by a `workflow.yaml`, and **the project you are working
> in may declare entirely different fields, values, and defaults, or none of these at all.** These rows
> show only what the **built-in** workflow happens to declare, included so the `ruki` examples elsewhere
> in this skill are concrete. **Never assume a field or value from this table exists in the current
> project — read its `workflow.yaml` first** (see the lookup chain above) to find the real set. Naming a
> field the workflow does not declare (e.g. `assignee` under a workflow that has no such field) is a hard
> `unknown field` parse error in `select`, `where`, and `set` alike — not an empty result — so reference
> only the intrinsic fields plus the fields this project's workflow actually declares.

| Field (built-in example) | Type (example) | Built-in values (example) |
|---|---|---|
| `status` | enum | `inbox`, `ready`, `inProgress`, `done` (default `inbox`) |
| `type` | enum | `story` (default), `bug`, `spike` |
| `priority` | enum | `"high"`, `"medium-high"`, `"medium"` (default), `"medium-low"`, `"low"` — pass the key as a string |
| `points` | enum | `"1"`, `"3"` (default), `"7"`, `"11"` — pass the key as a string |
| `assignee` | string | free text |
| `tags` | list&lt;string&gt; | e.g. `["bug", "urgent"]` |
| `dependsOn` | list&lt;ref&gt; | bare tiki ids, e.g. `["ABC123", "DEF456"]` |
| `due` | date | `YYYY-MM-DD` (a date, not a timestamp) — compare against date literals, not `now()` |
| `recurrence` | recurrence | set with a constructor: `daily()`, `weekly("monday")` (full lowercase day name), `monthly(1)` (day 1–31). Stored as cron; a raw cron string is rejected on assignment |

The `ruki` examples throughout this skill use these built-in values (e.g. `status="done"`,
`priority="high"`) purely as illustrations — substitute the values your project's `workflow.yaml` actually
defines. The built-in workflow also guards some status transitions with triggers — see the Update
section.

## Query

```sh
tiki exec 'select'                                                    # all tikis under the cwd
tiki exec 'select where has(status)'                                  # tikis carrying a status
tiki exec 'select title, status'                                      # field projection
tiki exec 'select id, title where status = "done"'                    # filter
tiki exec 'select where "bug" in tags order by priority'              # tag filter + sort
tiki exec 'select where has(due) and due < 2026-07-15'                # due before a date (guard with has)
tiki exec 'select where dependsOn any status != "done"'               # blocked tasks
tiki exec 'select where assignee = user()'                            # my tasks
```

Output is an ASCII table (use `--format json` for scripting). To read a tiki's full markdown body,
either project it via `select description where id = "ABC123"` or read the file directly — get the path
with `select filepath where id = "ABC123"`.

**Scope note.** Bare `select` iterates every tiki under the cwd, including those with no workflow-declared
fields. Predicates on absent fields are well-defined: `where status = "done"` is false for tikis with
no status, and `where status != "done"` is true. Use `has(<field>)` to scope explicitly when needed —
for example, `select where has(status)` lists only tikis that carry a status, regardless of value.

## Create

```sh
tiki exec 'create title="Fix login"'                                             # minimal
tiki exec 'create title="Fix login" priority="medium-high" status="ready" tags=["bug"]'      # full
tiki exec 'create title="Review" due=2026-04-01 + 2day'                          # date arithmetic
tiki exec 'create title="Sprint review" recurrence=weekly("monday")'             # recurrence (constructor)
```

Output: `created <ID>` (bare 6-char id).
Defaults: `status` from the `default: true` status in `workflow.yaml`, `type` from the `default: true`
type (or the first type), `priority` from the `default: true` priority value
(typically `"medium"` in the built-in workflow).

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
tiki exec 'update where id = "X7F4K2" set priority="high"'                 # priority
tiki exec 'update where status = "ready" set status="inProgress"'          # bulk update
tiki exec 'update where id = "X7F4K2" set tags=tags + ["urgent"]'          # add tag
tiki exec 'update where id = "X7F4K2" set due=2026-04-01'                  # set due date
tiki exec 'update where id = "X7F4K2" set due=empty'                       # clear due date
tiki exec 'update where id = "X7F4K2" set recurrence=weekly("monday")'     # set recurrence
tiki exec 'update where id = "X7F4K2" set recurrence=empty'                # clear recurrence
```

Output: `updated N tikis`. `set status=` accepts only values the applicable `workflow.yaml` declares (the
built-in workflow's are `inbox`, `ready`, `inProgress`, `done`). A workflow may also guard transitions
with triggers, which report `updated 0 tikis (1 failed)` with a reason when they refuse. The built-in
workflow enforces two such guards: a tiki must have an `assignee` before it can move to `inProgress`, and
must be `inProgress` before it can move to `done`. Satisfy the precondition first (an assignee and the
status change can be set in the same statement), or read the failure reason and act on it.

Assigning a field writes that key into the tiki's frontmatter — this is also how a notes-only tiki
becomes a tracked one (`set status = "..."`).

### Implement

When asked to implement a task and the user approves implementation, advance its status to whatever the
applicable `workflow.yaml` uses for work-in-progress (the built-in workflow's is `inProgress`, which also
requires an assignee — set both together):
```sh
tiki exec 'update where id = "X7F4K2" set assignee="me" status="inProgress"'
```

## Delete

```sh
tiki exec 'delete where id = "X7F4K2"'                          # by ID
tiki exec 'delete where status = "inbox"'                        # bulk (any workflow status value)
```

Output: `deleted N tikis`. Delete works on any tiki regardless of which fields it carries.

## Dependencies

```sh
# view a document's dependencies
tiki exec 'select id, title, status, dependsOn where id = "X7F4K2"'

# find what depends on a document (reverse lookup)
tiki exec 'select id, title where dependsOn any id = "X7F4K2"'

# find blocked tasks (any dependency not done)
tiki exec 'select id, title where dependsOn any status != "done"'

# add a dependency (bare id)
tiki exec 'update where id = "X7F4K2" set dependsOn=dependsOn + ["ABC123"]'

# remove a dependency
tiki exec 'update where id = "X7F4K2" set dependsOn=dependsOn - ["ABC123"]'
```

`dependsOn` entries must each be a valid id — 6 characters of `[A-Z0-9]` (e.g. `ABC123`) — and must
reference a tiki that exists, or the update is rejected.

## Provenance

`ruki` does not access git history. Use git commands for authorship questions — but first resolve the
tiki's path from its id, since files can live anywhere under the cwd:

```sh
# get the tiki's path
path=$(tiki exec --format json 'select filepath where id = "X7F4K2"' | jq -r '.[0].filepath')

# who created this tiki
git log --follow --diff-filter=A -- "$path"

# who last edited this tiki
git blame "$path"
```

Created timestamp and author are also available via:
```sh
tiki exec 'select createdAt, createdBy where id = "X7F4K2"'
```

## Important

- `tiki exec` handles `git add` and `git rm` automatically — never do manual git staging for tikis
- Never commit without user permission
- An id is exactly 6 characters, uppercase letters and digits only (e.g. `X7F4K2`); a value of any other
  shape is rejected. Ids are assigned by tiki on create — obtain one from a query or `created` output,
  never invent or reformat it
- Exit codes: 0 = ok, 2 = usage error, 3 = startup failure, 4 = query error
