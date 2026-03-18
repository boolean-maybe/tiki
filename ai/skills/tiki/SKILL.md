---
name: tiki
description: view, create, update, delete tikis and manage dependencies
allowed-tools: Read, Grep, Glob, Update, Edit, Write, WriteFile, Bash(git add:*), Bash(git rm:*)
---

# tiki

A tiki is a Markdown file in tiki format saved in the project `.doc/tiki` directory
with a name like `tiki-abc123.md` in all lower letters.
IMPORTANT! files are named in lowercase always
If this directory does not exist prompt user for creation

## tiki ID format

ID format: `TIKI-ABC123` where ABC123 is 6-char random alphanumeric
**Derived from filename, NOT stored in frontmatter**

Examples:
- Filename: `tiki-x7f4k2.md` → ID: `TIKI-X7F4K2`

## tiki format

A tiki format is Markdown with some requirements:

### frontmatter

```markdown
---
title: My ticket
type: story
status: backlog
priority: 3
points: 5
tags:
  - markdown
  - metadata
dependsOn:
  - TIKI-ABC123
due: 2026-04-01
recurrence: 0 0 * * MON
---
```

where fields can have these values:
- type: bug, feature, task, story, epic
- status: configurable via `workflow.yaml`. Default statuses: backlog, ready, in_progress, review, done.
  To find valid statuses for the current project, check the `statuses:` section in `~/.config/tiki/workflow.yaml`.
- priority: is any integer number from 1 to 5 where 1 is the highest priority. Mapped to priority description:
  - high: 1
  - medium-high: 2
  - medium: 3
  - medium-low: 4
  - low: 5
- points: story points from 1 to 10
- dependsOn: list of tiki IDs (TIKI-XXXXXX format) this task depends on
- due: due date in YYYY-MM-DD format (optional, date-only)
- recurrence: recurrence pattern in cron format (optional). Supported: `0 0 * * *` (daily), `0 0 * * MON` (weekly on Monday, etc.), `0 0 1 * *` (monthly)

### body

The body of a tiki is normal Markdown

if a tiki needs an attachment it is implemented as a normal markdown link to file syntax for example:
- Logs are attached [logs](mylogs.log)
- Here is the ![screenshot](screenshot.jpg "check out this box")
- Check out docs: <https://www.markdownguide.org>
- Contact: <user@example.com>

## Describe

When asked a question about a tiki find its file and read it then answer the question
If the question is who created this tiki or who updated it last - use the git username
For example:
- who created this tiki? use `git log --follow --diff-filter=A -- <file_path>` to see who created it
- who edited this tiki? use `git blame <file_path>` to see who edited the file

## View

`Created` timestamp is taken from git file creation if available else from the file creation timestamp.

`Author` is taken from git history as the git user who created the file.

## Creation

When asked to create a tiki:

- Generate a random 6-character alphanumeric ID (lowercase letters and digits)
- The filename should be lowercase: `tiki-abc123.md`
- If status is not specified use the default status from `~/.config/tiki/workflow.yaml` (typically `backlog`)
- If priority is not specified use 3
- If type is not specified - prompt the user or use `story` by default

Example: for random ID `x7f4k2`:
- Filename: `tiki-x7f4k2.md`
- tiki ID: `TIKI-X7F4K2`

### Create from file

if asked to create a tiki from Markdown or text file - create only a single tiki and use the entire content of the
file as its description. Title should be a short sentence summarizing the file content

#### git

After a new tiki is created `git add` this file.
IMPORTANT - only add, never commit the file without user asking and permitting

## Update

When asked to update a tiki - edit its file
For example when user says "set TIKI-ABC123 in progress" find its file and edit its frontmatter line from
`status: backlog` to `status: in progress`

### git

After a tiki is updated `git add` this file
IMPORTANT - only add, never commit the file without user asking and permitting

## Deletion

When asked to delete a tiki `git rm` its file
If for any reason `git rm` cannot be executed and the file is still there - delete the file

## Implement

When asked to implement a tiki and the user approves implementation change its status to `review` and `git add` it

## Dependencies

Tikis can declare dependencies on other tikis via the `dependsOn` frontmatter field.
Values are tiki IDs in `TIKI-XXXXXX` format. A dependency means this tiki is blocked by or requires the listed tikis.

### View dependencies

When asked about a tiki's dependencies, read its frontmatter `dependsOn` list.
For each dependency ID, read that tiki file to show its title, status, and assignee.
Highlight any dependencies that are not yet `done` -- these are blockers.

Example: "what blocks TIKI-X7F4K2?"
1. Read `.doc/tiki/tiki-x7f4k2.md` frontmatter
2. For each ID in `dependsOn`, read that tiki and report its status

### Find dependents

When asked "what depends on TIKI-ABC123?" -- grep all tiki files for that ID in their `dependsOn` field:
```
grep -l "TIKI-ABC123" .doc/tiki/*.md
```
Then read each match and check if TIKI-ABC123 appears in its `dependsOn` list.

### Add dependency

When asked to add a dependency (e.g. "TIKI-X7F4K2 depends on TIKI-ABC123"):
1. Verify the target tiki exists: check `.doc/tiki/tiki-abc123.md` exists
2. Read `.doc/tiki/tiki-x7f4k2.md`
3. Add `TIKI-ABC123` to the `dependsOn` list (create the field if missing)
4. Do not add duplicates
5. `git add` the modified file

### Remove dependency

When asked to remove a dependency:
1. Read the tiki file
2. Remove the specified ID from `dependsOn`
3. If `dependsOn` becomes empty, remove the field entirely (omitempty)
4. `git add` the modified file

### Validation

- Each value must be a valid tiki ID format: `TIKI-` followed by 6 alphanumeric characters
- Each referenced tiki must exist in `.doc/tiki/`
- Before adding, verify the target file exists; warn if it doesn't
- Circular dependencies: warn the user but do not block (e.g. A depends on B, B depends on A)

## Due Dates

Tikis can have a due date via the `due` frontmatter field.
The value must be in `YYYY-MM-DD` format (date-only, no time component).

### Set due date

When asked to set a due date (e.g. "set TIKI-X7F4K2 due date to April 1st 2026"):
1. Read `.doc/tiki/tiki-x7f4k2.md`
2. Add or update the `due` field with value `2026-04-01`
3. Format must be `YYYY-MM-DD` (4-digit year, 2-digit month, 2-digit day)
4. `git add` the modified file

### Remove due date

When asked to remove or clear a due date:
1. Read the tiki file
2. Remove the `due` field entirely (omitempty)
3. `git add` the modified file

### Query by due date

Users can filter tasks by due date in the TUI using filter expressions:
- `due = '2026-04-01'` - exact match
- `due < '2026-04-01'` - before date
- `due < NOW` - overdue tasks
- `due - NOW < 7day` - due within 7 days

### Validation

- Date must be in `YYYY-MM-DD` format
- Must be a valid calendar date (no Feb 30, etc.)
- Date-only (no time component)

## Recurrence

Tikis can have a recurrence pattern via the `recurrence` frontmatter field.
The value must be a supported cron pattern. Displayed as English in the TUI (e.g. "Weekly on Monday").
This is metadata-only — it does not auto-create tasks on completion.

### Set recurrence

When asked to set a recurrence (e.g. "set TIKI-X7F4K2 to recur weekly on Monday"):
1. Read `.doc/tiki/tiki-x7f4k2.md`
2. Add or update the `recurrence` field with value `0 0 * * MON`
3. `git add` the modified file

Supported patterns:
- `0 0 * * *` — Daily
- `0 0 * * MON` — Weekly on Monday (through SUN for other days)
- `0 0 1 * *` — Monthly

### Remove recurrence

When asked to remove or clear a recurrence:
1. Read the tiki file
2. Remove the `recurrence` field entirely (omitempty)
3. `git add` the modified file

### Validation

- Must be one of the supported cron patterns listed above
- Empty/omitted means no recurrence
