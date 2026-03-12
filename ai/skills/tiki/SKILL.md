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
