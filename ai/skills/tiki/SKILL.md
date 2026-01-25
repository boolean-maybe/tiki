---
name: tiki
description: view, create, update, delete tikis
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
---
```

where fields can have these values:
- type: bug, feature, task, story, epic
- status: backlog, ready, in_progress, review, done
- priority: is any integer number from 1 to 5 where 1 is the highest priority
- points: story points from 1 to 10

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

`Created` timestamp is taken from git file creation if available else from the file creation timestamp
`Author` is taken from git history as the git user who created the file

## Creation

When asked to create a tiki:

- Generate a random 6-character alphanumeric ID (lowercase letters and digits)
- The filename should be lowercase: `tiki-abc123.md`
- If status is not specified use `backlog`
- If priority is not specified use 3
- If type is not specified - prompt the user or use `story` by default

Example: for random ID `x7f4k2`:
- Filename: `tiki-x7f4k2.md`
- Derived ID: `TIKI-X7F4K2`

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