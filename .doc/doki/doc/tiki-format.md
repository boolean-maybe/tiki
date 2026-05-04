# Document format

`tiki` manages a single Markdown workspace under `.doc/`. Every file under that tree is a **managed document**:
a Markdown file with YAML frontmatter, a required bare `id`, and arbitrary body content. Workflow fields are
optional — a document becomes a board/list item only when it explicitly declares workflow metadata.

## Workspace layout

Managed documents live anywhere under `.doc/`. Folder structure is user-controlled organization, not identity.

```
.doc/
├── workflow.yaml           # workflow/view configuration (excluded from scan)
├── config.yaml             # appearance/display config (excluded from scan)
├── ABC123.md               # a task (has status/type/priority frontmatter)
├── DEF456.md               # a plain doc (only id + title)
├── architecture/
│   ├── X7K2QM.md           # a design note
│   └── diagrams/
│       └── 9LP4ZR.md       # a diagram page
└── assets/                 # shared images, svgs, binary assets
    └── logo.png
```

Rules:

- `.doc/**/*.md` is scanned recursively and loaded as managed documents.
- `.doc/workflow.yaml` and `.doc/config.yaml` are configuration files, not documents.
- Non-Markdown files (images, svgs, binary assets) are not loaded as documents; keep them under `.doc/assets/`
  or alongside the documents that reference them.
- New documents from `tiki init`, `tiki demo`, piped input, and `ruki create` default to `.doc/<ID>.md`.
  You are free to move them into subdirectories afterward — the `id` is stable.

## Required frontmatter

Every managed document must declare an `id`. Missing or invalid ids cause the loader to reject the file and
report it in diagnostics.

```yaml
---
id: ABC123
title: Implement search
---

Markdown body.
```

- **`id`** — required. A bare 6-character uppercase alphanumeric string matching `^[A-Z0-9]{6}$`.
  No `TIKI-` prefix, no slug, globally unique across `.doc/`. Legacy prefixed ids are not recognized;
  edit them to the bare form manually if you are migrating old content.
- **`title`** — required for workflow documents; optional (but recommended) for plain docs. Non-empty,
  max 200 characters.

## Workflow fields (optional)

A managed document participates in workflow views (board, list) when it declares explicit workflow
fields in frontmatter. Absence is preserved: a document without `status` has no status, not a defaulted one.
Default values apply only in explicit creation paths — `ruki create`, piped capture into a workflow-capture
project, sample task generation, and the TUI's create action.

```yaml
---
id: X7F4K2
title: Fix login bug
type: bug
status: inProgress
assignee: alex
priority: 2
points: 5
tags:
  - frontend
  - urgent
dependsOn:
  - ABC123
  - DEF456
due: 2026-04-01
recurrence: 0 0 * * MON
---

Describe what is broken and how to reproduce it.
```

### `title`

String. Required for workflow documents. Non-empty, max 200 characters.

```yaml
title: Implement user authentication
```

### `type`

Optional. Must match a type key defined in `workflow.yaml` under `types:`. When absent, the document is either
a plain doc (no workflow) or receives the default `type` on explicit creation. Default types ship as `story`,
`bug`, `spike`, `epic`; each can have its own label and emoji. Aliases are not supported — use the canonical key.

```yaml
type: bug
```

### `status`

Optional. Must match a status key defined in your project's `workflow.yaml`. When absent, the document does
not appear on board/list views. On explicit creation, the status marked `default: true` is applied.

```yaml
status: inProgress
```

### `priority`

Optional. Stored as an integer 1–5 (1 = highest, 5 = lowest). Default on explicit creation: `3`.

In the TUI, priority is displayed as text with a color indicator:

| Value | TUI label | Emoji |
|-------|-----------|-------|
| 1 | High | 🔴 |
| 2 | Medium High | 🟠 |
| 3 | Medium | 🟡 |
| 4 | Medium Low | 🔵 |
| 5 | Low | 🟢 |

Text aliases are accepted on create (case-insensitive, hyphens/underscores/spaces as separators):
`high` = 1, `medium-high` = 2, `medium` = 3, `medium-low` = 4, `low` = 5.

```yaml
priority: 2
```

### `points`

Optional integer. Range: `0` to `maxPoints` (configurable via `tiki.maxPoints`, default max is 10).
`0` means unestimated. Values outside the valid range default to `maxPoints / 2` (typically 5).

```yaml
points: 3
```

### `assignee`

Optional free-form string, typically a username. Default: empty.

```yaml
assignee: alex
```

### `tags`

Optional string list. Arbitrary labels attached to a document.
Values are trimmed, empty entries are dropped, and duplicates are removed.
Default: empty. Both YAML list formats are accepted:

```yaml
# block list
tags:
  - frontend
  - urgent

# inline list
tags: [frontend, urgent]
```

### `dependsOn`

Optional string list. Each entry must be a bare 6-character uppercase document id referencing an existing
document. Ids are uppercased, empty entries are dropped, and duplicates are removed. A dependency means
this document is blocked by the listed documents. Default: empty.

```yaml
# block list
dependsOn:
  - ABC123
  - DEF456

# inline list
dependsOn: [ABC123, DEF456]
```

Legacy `TIKI-ABC123`-style references are rejected by validation — edit them to the bare `ABC123` form.

### `due`

Optional date in `YYYY-MM-DD` format (date-only, no time component).
Represents when the document should be completed by. Empty string or omitted field means no due date.

```yaml
due: 2026-04-01
```

### `recurrence`

Optional cron string specifying a recurrence pattern. Displayed as English in the TUI.
This is metadata only — it does not auto-create documents on completion.

Supported patterns:

| Cron | Display |
|------|---------|
| (empty) | None |
| `0 0 * * *` | Daily |
| `0 0 * * MON` | Weekly on Monday |
| `0 0 * * TUE` | Weekly on Tuesday |
| `0 0 * * WED` | Weekly on Wednesday |
| `0 0 * * THU` | Weekly on Thursday |
| `0 0 * * FRI` | Weekly on Friday |
| `0 0 * * SAT` | Weekly on Saturday |
| `0 0 * * SUN` | Weekly on Sunday |
| `0 0 1 * *` | Monthly |

```yaml
recurrence: 0 0 * * MON
```

## Custom fields

Custom fields declared under `workflow.yaml` `fields:` appear alongside built-in frontmatter and are
round-tripped through save/load. Unknown frontmatter keys that are not registered custom fields are preserved
as-is, so workflow schema changes do not lose data. See [Custom fields](customization/custom-fields.md).

## Derived fields

These fields are not stored in the file:

- `created by`
- `created at`
- `updated at`

`created at` / `updated at` are derived from git history (commit times) with file mtime as a fallback when
git is disabled or the file is uncommitted. `created by` is populated from git authorship when available;
for uncommitted files or no-git mode it falls back to the current `tiki` identity (configured `identity.name`
or `identity.email` → git user → OS account name).

## Plain documents

A managed document with only `id` and `title` in its frontmatter is a **plain document**: a note, page, or
reference. Plain documents:

- are rendered by markdown, wiki, and detail views
- do not appear on board/list views
- can still be queried by `ruki` — predicates on absent workflow fields evaluate false, so most
  workflow queries skip plain documents naturally. To filter *by presence*, use the `has(...)` builtin.
  A document is classified as workflow when **any** of nine frontmatter keys is present: `status`,
  `type`, `priority`, `points`, `tags`, `dependsOn`, `due`, `recurrence`, `assignee`. So the full
  workflow-vs-plain split is:

  ```
  -- workflow documents
  select where has(status) or has(type) or has(priority) or has(points) or has(tags)
            or has(dependsOn) or has(due) or has(recurrence) or has(assignee)

  -- plain documents
  select where not has(status) and not has(type) and not has(priority) and not has(points)
            and not has(tags) and not has(dependsOn) and not has(due) and not has(recurrence)
            and not has(assignee)
  ```

  Checking only `has(status)` would misclassify documents that declare only `priority:` or
  `dependsOn:` — all nine keys promote a document to workflow-capable.

  `is empty` does **not** match absent workflow fields on plain documents — always use `has(...)` for
  presence tests on built-in workflow fields.
- can be promoted to workflow documents by assigning a workflow field —
  `update where id = "ABC123" set status = "backlog"` makes the document workflow-capable

```yaml
---
id: QR9XV2
title: Architecture overview
---

# System architecture

Entry points, component boundaries, and deployment diagrams.
```

## Cross-document links

Reference another document by its bare id inside double brackets:

```markdown
See the [[ABC123]] for background.
```

Wikilinks resolve through the document index, so they survive file moves and renames. Unknown ids render as
a literal `[[ABC123]]` with a `*(not found)*` marker — broken references stay visible instead of silently
disappearing.

Plain Markdown links to relative paths also work inside bodies; use them for anchored links to specific
sections within a document.
