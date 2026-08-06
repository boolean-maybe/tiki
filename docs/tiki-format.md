# Tiki format

`tiki` is a viewer over the current directory. Every `.md` file beneath the cwd that carries a frontmatter
`id` is a **tiki**: a Markdown file with YAML frontmatter, a bare `id`, and arbitrary body content. Beyond
`id` and `title`, frontmatter is an open field map — a tiki participates in board/list views when its
fields satisfy that view's `select` filter, typically via `has(...)` checks. A `.md` file with no `id` is
not store-indexed but is still wiki-viewable by path — plain prose coexists with tikis, no warning.

## Workspace layout

Tikis live anywhere under the current directory. Folder structure is user-controlled organization, not
identity.

```
./                          # the scan root (current working directory)
├── workflow.yaml           # workflow/view configuration (excluded from scan)
├── config.yaml             # appearance/display config (excluded from scan)
├── ABC123.md               # a tiki carrying status/type/priority fields
├── DEF456.md               # a tiki with only id + title
├── notes.md                # plain Markdown, no id — wiki-viewable by path, not a tiki
├── architecture/
│   ├── X7K2QM.md           # a design note tiki
│   └── diagrams/
│       └── 9LP4ZR.md       # a diagram-page tiki
└── assets/                 # shared images, svgs, binary assets
    └── logo.png
```

Rules:

- `./**/*.md` is scanned recursively and loaded; files with a frontmatter `id` become tikis.
- `.git/`, dotted directories, and paths matched by `.gitignore`/`.tikiignore` at the scan root are skipped.
- `workflow.yaml` and `config.yaml` are configuration files, not tikis.
- Non-Markdown files (images, svgs, binary assets) are not loaded as tikis; keep them under `assets/`
  or alongside the tikis that reference them.
- New tikis from `tiki demo`, piped input, and `ruki create` are named `./<slug>.md`, where `<slug>` is
  a lowercase-kebab transliteration of the title (e.g. "Fix Login Bug" → `fix-login-bug.md`). On a name
  collision a numeric suffix is appended (`fix-login-bug-2.md`). The filename is not the source of the
  `id` — the frontmatter `id` is — so you are free to rename or move files afterward; the `id` is stable.

## Required frontmatter

A tiki must declare an `id` to be store-indexed. A file with no `id` is skipped silently (it is plain
wiki content, not a managed tiki — never a diagnostic). A file that *has* an `id` but malformed is still
reported in diagnostics.

```yaml
---
id: ABC123
title: Implement search
---

Markdown body.
```

- **`id`** — required. A bare 6-character uppercase alphanumeric string matching `^[A-Z0-9]{6}$`.
  No `TIKI-` prefix, no slug, globally unique across the scan root. Legacy prefixed ids are not recognized;
  edit them to the bare form manually if you are migrating old content.
- **`title`** — optional on disk; the loader accepts a missing or empty title and stores it as `""`.
  Required and non-empty (max 200 characters) on **create** and **update** paths via `tiki exec` —
  ruki rejects assignments that would leave the title blank. Recommended on every tiki, since the
  title is the display label across all views.

## Workflow-declared fields (optional)

Beyond `id` and `title`, frontmatter is an open field map. The fields below are **workflow-declared**
— the active `workflow.yaml fields:` lists them, so they get type coercion, validation, and any
typed UI treatment. They are all optional. Absence is preserved on disk: a tiki with no `status:`
key has no status, not a defaulted one. Default values apply only on explicit creation —
`ruki create`, piped capture, and the TUI's create action.

```yaml
---
id: X7F4K2
title: Fix login bug
type: bug
status: inProgress
assignee: alex
priority: medium-high
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

Optional. The bundled workflows declare `type` as an enum, so its value must match that workflow
declaration. On explicit creation, the value marked `default: true` is applied. The stock enum values
include `story`, `bug`, `spike`, and `epic`; each can have its own label and emoji. The field name has no
built-in behavior: changing its declared type changes validation and editing like any other workflow field.

```yaml
type: bug
```

### `status`

Optional. The bundled workflows declare `status` as an enum, so its value must match that workflow
declaration. When absent, the tiki falls outside any view whose lane filter requires `status`. On explicit
creation, the value marked `default: true` is applied. The field name has no built-in behavior: changing its
declared type changes validation and editing like any other workflow field.

```yaml
status: inProgress
```

### `priority`

Optional. Priority is an `enum` field whose values are defined in `workflow.yaml` — tiki has no built-in
priority scale. The stock kanban workflow ships with five string values, most to least urgent:

| Value | TUI label | Emoji |
|-------|-----------|-------|
| `high` | High | 🔴 |
| `medium-high` | Medium High | 🟠 |
| `medium` | Medium | 🟡 |
| `medium-low` | Medium Low | 🟢 |
| `low` | Low | 🔵 |

The default on explicit creation is `medium`. The value stored in frontmatter is the enum key
(a string), not a number. Unknown values are retained as stale data for manual repair.

```yaml
priority: medium-high
```

### `points`

Optional. Like `priority`, points is an `enum` field defined in `workflow.yaml` — there is no built-in
numeric scale or `maxPoints` setting. The stock kanban workflow ships with the estimation values
`1`, `3`, `7`, `11` (stored as strings), defaulting to `3`. Redefine the `points` field in your own
workflow to use a different scale.

```yaml
points: 3
```

### `assignee`

Optional free-form string, typically a username. Default: empty. The bundled workflows declare this field as
`type: user`, which changes the editor to a suggestion-enabled user picker but does not change the frontmatter
shape or ruki type.

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

The cron string above is the on-disk form. In `ruki` (workflow rules and the command bar) you author these values with
the `daily()`, `weekly("<weekday>")`, and `monthly(<day>)` constructors rather than typing cron by hand — see the ruki
built-in functions reference.

```yaml
recurrence: 0 0 * * MON
```

## Project-specific fields

Any field declared under `workflow.yaml fields:` is round-tripped through save/load with type
coercion. Unknown frontmatter keys that the workflow does not declare are preserved as-is, so
workflow schema changes do not lose data. See [Custom fields](customization/custom-fields.md).

## Derived fields

These fields are not stored in the file:

- `created by`
- `created at`
- `updated at`

`created at` / `updated at` are derived from git history (commit times) with file mtime as a fallback when
the scan root is not a git repository or the file is uncommitted. `created by` is populated from git
authorship when available; for uncommitted files or a non-repo scan root it falls back to the current
`tiki` identity (configured `identity.name` or `identity.email` → git user → OS account name).

## Sparse tikis (notes, docs, prompts)

A tiki with only `id` and `title` in its frontmatter is just a tiki with no workflow-declared
fields set — useful as a note, page, or reference. There is no separate "plain document" class in
the model; the absence of fields is the whole story. Such a tiki:

- is rendered by markdown, wiki, and detail views
- falls outside any view whose lane filter requires workflow-declared fields (typical board and
  list lanes)
- is queryable by `ruki` like any other tiki. Comparisons against absent fields are well-defined:
  `where status = "done"` evaluates `false` for tikis with no `status`, and `where status != "done"`
  evaluates `true`. Use `has(<field>)` when you need to filter explicitly by presence — for example,
  `select where has(status)` lists every tiki that carries a status, regardless of value.
- gains a field whenever you assign one. `update where id = "ABC123" set status = "inbox"` simply
  writes `status: inbox` into the frontmatter. There is no special "promotion" event — fields are
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
