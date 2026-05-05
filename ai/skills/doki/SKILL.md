---
name: doki
description: view, create, and update notes-only tikis (docs, notes, prompts) under .doc/
allowed-tools: Read, Grep, Glob, Edit, Write, Bash(tiki exec:*), Bash(git add:*), Bash(git rm:*)
---

# doki

A `doki` is a tiki that carries no schema-known fields — just `id:` and (typically) `title:`
in its frontmatter. Use this skill for notes, design pages, prompts, and wiki content that should not
appear on board or list views. There is no separate "plain document" entity in the model; a doki is
just a tiki with no `status`, `type`, `priority`, etc.

- Every tiki has a bare 6-character uppercase alphanumeric `id` (e.g. `Q9V3LM`) stored in the
  `id:` frontmatter field. Identity is in the frontmatter, not the filename.
- Tikis live under `.doc/` at any depth — organize freely via subdirectories.
- A notes-only tiki is reachable by id and by path, rendered in wiki/detail/markdown views, and falls
  outside any view whose lane filter requires schema-known fields (typical board and list lanes).
- If a tiki needs tracking metadata (`status`, `priority`, etc.), use the `tiki` skill instead — or just
  assign a field on this one. There is no separate "promotion" event; setting `status` simply adds a
  `status:` key to the frontmatter.

## Frontmatter shape

A notes-only tiki looks like this:

```md
---
id: Q9V3LM
title: Architecture overview
---

# System architecture

Entry points, component boundaries, and deployment diagrams.
```

`id:` is required. `title:` is recommended. Custom fields (declared under `fields:` in
`workflow.yaml`) are allowed. Schema-known fields (`status`, `type`, `priority`, `points`, `dependsOn`,
…) are absent — that is what keeps the tiki out of board views.

## Creating a notes-only tiki

Notes-only tikis are file-system objects first; you can create them directly:

1. Generate a bare 6-char uppercase alphanumeric id (e.g. `Q9V3LM`) and verify it is not in use.
2. Write the file at `.doc/<ID>.md` (or any subpath like `.doc/architecture/<ID>.md`).
3. Include at minimum `id:` and `title:` in frontmatter.
4. Stage the file with git: `git add <path>`.

Alternatively, `tiki exec 'create title="..."'` creates a tiki using the workflow's defaults, which
typically includes `status`/`type`/etc. To create a notes-only tiki non-interactively, either:

- Write the file directly (preferred for general docs).
- Pipe through `tiki` in a project whose active workflow has no `default: true` status — in that case
  captured input lands with only `id:` and `title:` set.

## Finding notes-only tikis

Find by id:
```sh
tiki exec 'select id, title, filepath where id = "Q9V3LM"'
```

List notes-only tikis — those with none of the schema-known fields set. Use the `has()`
presence builtin: `not has(<field>)` is **true** only when the key is missing from frontmatter. (`is
empty` treats absent fields as empty too, so `where status is empty` matches both "absent" and
"present but blank" — `not has(status)` is the precise probe for "key not declared".)

For a status-filtered board, `not has(status)` alone matches every tiki the board skips:

```sh
tiki exec 'select id, title, filepath where not has(status)'
```

Use the full nine-field negation only when you specifically want "tiki carries no schema-known
fields at all" — for example, when scoping a backup or migration of pure notes:

```sh
tiki exec '
  select id, title, filepath
  where not has(status) and not has(type) and not has(priority) and not has(points)
    and not has(tags) and not has(dependsOn) and not has(due) and not has(recurrence)
    and not has(assignee)
'
```

Search by title (substring). The `in` operator with a string right-hand side performs a substring check;
`contains` is not a ruki keyword:

```sh
tiki exec 'select id, title, filepath where "architecture" in title'
```

The `filepath` synthetic field returns the tiki's absolute path — handy when you need to open or edit
the file directly.

## Editing content

Notes-only tiki bodies are edited directly with Read/Write/Edit tools at the path returned by `filepath`.
The frontmatter id must be preserved unchanged on edit. Moving or renaming the file is fine — the id
follows the tiki.

Cross-reference other tikis inside the body with wikilinks:

```markdown
See also [[ABC123]] for the related task.
See [Architecture overview](../architecture/OVERVIEW.md) for the related doc.
```

Wikilinks resolve through the tiki index and survive file moves. Unknown ids render as
`[[ABC123]] *(not found)*` so broken references stay visible.

## Important

- Always use bare ids (`Q9V3LM`), never the legacy `TIKI-` prefix form
- `id:` is immutable — never change it on an existing tiki
- Stage new files with `git add` and removals with `git rm`
- Never commit without user permission
