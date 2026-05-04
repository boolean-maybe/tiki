---
name: doki
description: view, create, and update plain managed documents (docs, notes, prompts) under .doc/
allowed-tools: Read, Grep, Glob, Edit, Write, Bash(tiki exec:*), Bash(git add:*), Bash(git rm:*)
---

# doki

A `doki` is a **plain managed document** in the unified `.doc/` workspace — a Markdown file with
frontmatter that carries **only** `id:` (and typically `title:`), with no workflow fields. Use plain
documents for docs, notes, prompts, design pages, and wiki content that should not appear on board views.

- Every managed document has a bare 6-character uppercase alphanumeric `id` (e.g. `Q9V3LM`) stored in the
  `id:` frontmatter field. Identity is in the frontmatter, not the filename.
- Documents live under `.doc/` at any depth — organize freely via subdirectories.
- Plain documents are reachable by id and by path, rendered in wiki/detail/markdown views, and invisible
  to workflow views.
- If a document needs workflow metadata (`status`, `priority`, etc.), use the `tiki` skill instead — or
  assign a workflow field to this document, which promotes it to a workflow document.

## Frontmatter shape

A plain document looks like this:

```md
---
id: Q9V3LM
title: Architecture overview
---

# System architecture

Entry points, component boundaries, and deployment diagrams.
```

`id:` is required. `title:` is recommended. Custom fields (declared under `fields:` in
`workflow.yaml`) are allowed. Workflow fields (`status`, `type`, `priority`, `points`, `dependsOn`)
are absent — that is what makes it a plain document.

## Creating a plain document

Plain documents are file-system objects first; you can create them directly:

1. Generate a bare 6-char uppercase alphanumeric id (e.g. `Q9V3LM`) and verify it is not in use.
2. Write the file at `.doc/<ID>.md` (or any subpath like `.doc/architecture/<ID>.md`).
3. Include at minimum `id:` and `title:` in frontmatter.
4. Stage the file with git: `git add <path>`.

Alternatively, `tiki exec 'create title="..."'` creates a workflow document. To create a plain document
non-interactively, either:

- Write the file directly (preferred for general docs).
- Pipe through `tiki` in a project whose active workflow has no `default: true` status — in that case
  captured input becomes a plain document.

## Finding plain documents

Find by id:
```sh
tiki exec 'select id, title, filepath where id = "Q9V3LM"'
```

List all plain documents (those with no explicit workflow metadata on disk). Use the `has()` presence
builtin — `is empty` does **not** match absent workflow fields under Phase 5 presence-aware semantics,
so `where status is empty` silently returns zero plain documents.

The document classifier treats any of **nine** frontmatter keys as workflow: `status`, `type`,
`priority`, `points`, `tags`, `dependsOn`, `due`, `recurrence`, `assignee`. A plain document has
**none** of them — so the correct plain-doc probe negates the full set:

```sh
tiki exec '
  select id, title, filepath
  where not has(status) and not has(type) and not has(priority) and not has(points)
    and not has(tags) and not has(dependsOn) and not has(due) and not has(recurrence)
    and not has(assignee)
'
```

Checking fewer keys is unsafe: a document with only `priority:` or `dependsOn:` in its frontmatter is
still workflow-capable and would be misreported as plain.

Search by title (substring). The `in` operator with a string right-hand side performs a substring check;
`contains` is not a ruki keyword:

```sh
tiki exec 'select id, title, filepath where "architecture" in title'
```

The `filepath` synthetic field returns the document's absolute path — handy when you need to open or edit
the file directly.

## Editing content

Plain-document bodies are edited directly with Read/Write/Edit tools at the path returned by `filepath`.
The frontmatter id must be preserved unchanged on edit. Moving or renaming the file is fine — the id
follows the document.

Cross-reference other managed documents inside the body with wikilinks:

```markdown
See also [[ABC123]] for the workflow task.
See [Architecture overview](../architecture/OVERVIEW.md) for the related doc.
```

Wikilinks resolve through the document index and survive file moves. Unknown ids render as
`[[ABC123]] *(not found)*` so broken references stay visible.

## Important

- Always use bare ids (`Q9V3LM`), never the legacy `TIKI-` prefix form
- `id:` is immutable — never change it on an existing document
- Stage new files with `git add` and removals with `git rm`
- Never commit without user permission
