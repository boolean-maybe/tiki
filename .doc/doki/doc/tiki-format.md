# Tiki format

`tiki` manages a single Markdown workspace under `.doc/`. Every file under that tree is a **tiki**:
a Markdown file with YAML frontmatter, a required bare `id`, and arbitrary body content. Beyond `id`
and `title`, frontmatter is an open field map — a tiki participates in board/list views when its fields
satisfy that view's `select` filter, typically via `has(...)` checks.

## Workspace layout

Tikis live anywhere under `.doc/`. Folder structure is user-controlled organization, not identity.

```
.doc/
├── workflow.yaml           # workflow/view configuration (excluded from scan)
├── config.yaml             # appearance/display config (excluded from scan)
├── ABC123.md               # a tiki carrying status/type/priority fields
├── DEF456.md               # a tiki with only id + title
├── architecture/
│   ├── X7K2QM.md           # a design note tiki
│   └── diagrams/
│       └── 9LP4ZR.md       # a diagram-page tiki
└── assets/                 # shared images, svgs, binary assets
    └── logo.png
```

Rules:

- `.doc/**/*.md` is scanned recursively and loaded as tikis.
- `.doc/workflow.yaml` and `.doc/config.yaml` are configuration files, not tikis.
- Non-Markdown files (images, svgs, binary assets) are not loaded as tikis; keep them under `.doc/assets/`
  or alongside the tikis that reference them.
- New tikis from `tiki init`, `tiki demo`, piped input, and `ruki create` default to `.doc/<ID>.md`.
  You are free to move them into subdirectories afterward — the `id` is stable.

## Required frontmatter

Every tiki must declare an `id`. Missing or invalid ids cause the loader to reject the file and
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
- **`title`** — optional on disk; the loader accepts a missing or empty title and stores it as `""`.
  Required and non-empty (max 200 characters) on **create** and **update** paths via `tiki exec` —
  ruki rejects assignments that would leave the title blank. Recommended on every tiki, since the
  title is the display label across all views.

## Schema-known fields (optional)

Beyond `id` and `title`, frontmatter is an open field map. The fields below are **schema-known** —
the workflow registers their types so they get coercion, validation, and special UI treatment. They
are all optional. Absence is preserved on disk: a tiki with no `status:` key has no status, not a
defaulted one. Default values apply only on explicit creation — `ruki create`, piped capture into a
project that has a default status, sample-tiki generation, and the TUI's create action.

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

### `type`

Optional. Must match a type key defined in `workflow.yaml` under `types:`. When absent, the tiki has
no type; on explicit creation, the type marked `default: true` (or the first type) is applied.
Default types ship as `story`, `bug`, `spike`, `epic`; each can have its own label and emoji. Aliases
are not supported — use the canonical key.

```yaml
type: bug
```

### `status`

Optional. Must match a status key defined in your project's `workflow.yaml`. When absent, the tiki
falls outside any view whose lane filter requires `status` (most board and list views). On explicit
creation, the status marked `default: true` is applied.

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

Custom fields declared under `workflow.yaml` `fields:` appear alongside schema-known frontmatter and are
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

## Sparse tikis (notes, docs, prompts)

A tiki with only `id` and `title` in its frontmatter is just a tiki with no schema-known fields set —
useful as a note, page, or reference. There is no separate "plain document" class in the model; the
absence of fields is the whole story. Such a tiki:

- is rendered by markdown, wiki, and detail views
- falls outside any view whose lane filter requires schema-known fields (typical board and list lanes)
- is queryable by `ruki` like any other tiki. Comparisons against absent fields are well-defined:
  `where status = "done"` evaluates `false` for tikis with no `status`, and `where status != "done"`
  evaluates `true`. Use `has(<field>)` when you need to filter explicitly by presence — for example,
  `select where has(status)` lists every tiki that carries a status, regardless of value.
- gains a field whenever you assign one. `update where id = "ABC123" set status = "backlog"` simply
  writes `status: backlog` into the frontmatter. There is no special "promotion" event — fields are
  added and removed like any other map entry.

```yaml
---
id: QR9XV2
title: Architecture overview
---

# System architecture

Entry points, component boundaries, and deployment diagrams.
```

## Cross-tiki links

Reference another tiki by its bare id inside double brackets:

```markdown
See the [[ABC123]] for background.
```

Wikilinks resolve through the tiki index, so they survive file moves and renames. Unknown ids render as
a literal `[[ABC123]]` with a `*(not found)*` marker — broken references stay visible instead of silently
disappearing.

Plain Markdown links to relative paths also work inside bodies; use them for anchored links to specific
sections within a tiki.
