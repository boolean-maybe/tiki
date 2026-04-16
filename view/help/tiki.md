# tiki format

First of all, you just navigated to a linked file. To go back press `Left` arrow or `Alt-Left`
To go forward press `Right` arrow or `Alt-Right`

Keep your tickets in your pockets!

`tiki` refers to a task or a ticket (hence tiki) stored in your **git** repo

- like a ticket it can have a status, priority, assignee, points, type and multiple tags attached to it
- they are essentially just Markdown files and you can use full Markdown syntax to describe a story or a bug
- they are stored in `.doc/tiki` subdirectory and are **git**-controlled - they are added to **git** when they are created,
  removed when they are done and the entire history is preserved in **git** repo
- because they are in **git** they can be perfectly synced up to the state of your repo or a branch
- you can use either the `tiki` CLI tool or any of the AI coding assistant to work with your tikis

## tiki format

Tiki stores tickets (aka tikis) and documents (aka dokis) in the git repo along with code
They are stored under `.doc` directory and are supposed to be checked-in/versioned along with all other files

The `.doc/` directory contains two main subdirectories:
- **doki/**: Documentation files (wiki-style markdown pages)
- **tiki/**: Task files (kanban style tasks with YAML frontmatter)

## Directory Structure

```
.doc/
├── doki/
│   ├── index.md
│   ├── page2.md
│   ├── page3.md
│   └── sub/
│       └── page4.md
└── tiki/
    ├── tiki-k3x9m2.md
    ├── tiki-7wq4na.md
    ├── tiki-p8j1fz.md
    └── ...
```


## Tiki files

Tiki files are saved in `.doc/tiki` directory and can be managed via:

- `tiki` cli
- AI tools such as `claude`, `gemini`, `codex` or `opencode`
- manually

A tiki is made of its frontmatter that includes all fields related to a tiki status and types and its description
in Markdown format

```text
        ---
        title: Sample title
        type: story
        status: backlog
        assignee: booleanmaybe
        priority: 3
        points: 10
        tags:
            - UX
            - test
        dependsOn:
            - TIKI-ABC123
            - TIKI-DEF456
        due: 2026-04-01
        recurrence: 0 0 * * MON
        ---
        
        This is the description of a tiki in Markdown:
        
        # Tests
        Make sure all tests pass
        
        ## Integration tests
        Integration test cases
```

### Fields

#### title

**Required.** String. The name of the tiki. Must be non-empty, max 200 characters.

```yaml
title: Implement user authentication
```

#### type

Optional string. Defaults to the first type defined in `workflow.yaml`.

Valid values are the type keys defined in the `types:` section of `workflow.yaml`.
Default types: `story`, `bug`, `spike`, `epic`. Each type can have a label and emoji
configured in `workflow.yaml`. Aliases are not supported; use the canonical key.

```yaml
type: bug
```

#### status

Optional string. Must match a status defined in your project's `workflow.yaml`.
Default: the status marked `default: true` in the workflow (typically `backlog`).

If the value doesn't match any status in the workflow, it falls back to the default.

```yaml
status: in_progress
```

#### priority

Optional. Stored in the file as an integer 1–5 (1 = highest, 5 = lowest). Default: `3`.

In the TUI, priority is displayed as text with a color indicator:

| Value | TUI label | Emoji |
|-------|-----------|-------|
| 1 | High | 🔴 |
| 2 | Medium High | 🟠 |
| 3 | Medium | 🟡 |
| 4 | Medium Low | 🔵 |
| 5 | Low | 🟢 |

Text aliases are accepted when creating tikis (case-insensitive, hyphens/underscores/spaces as separators):
`high` = 1, `medium-high` = 2, `medium` = 3, `medium-low` = 4, `low` = 5.

```yaml
priority: 2
```

#### points

Optional integer. Range: `0` to `maxPoints` (configurable via `tiki.maxPoints`, default max is 10).
`0` means unestimated. Values outside the valid range default to `maxPoints / 2` (typically 5).

```yaml
points: 3
```

#### assignee

Optional string. Free-form text, typically a username. Default: empty.

```yaml
assignee: booleanmaybe
```

#### tags

Optional string list. Arbitrary labels attached to a tiki. Empty and whitespace-only strings are filtered out.
Default: empty. Both YAML list formats are accepted:

```yaml
# block list
tags:
    - frontend
    - urgent

# inline list
tags: [frontend, urgent]
```

#### dependsOn

Optional string list. Each entry must be a valid tiki ID in `TIKI-XXXXXX` format (6-character alphanumeric suffix)
referencing an existing tiki. IDs are automatically uppercased. A dependency means this tiki is blocked by the listed tikis.
Default: empty. Both YAML list formats are accepted:

```yaml
# block list
dependsOn:
    - TIKI-ABC123
    - TIKI-DEF456

# inline list
dependsOn: [TIKI-ABC123, TIKI-DEF456]
```

#### due

Optional date in `YYYY-MM-DD` format (date-only, no time component).
Represents when the task should be completed by. Empty string or omitted field means no due date.

```yaml
due: 2026-04-01
```

#### recurrence

Optional cron string specifying a recurrence pattern. Displayed as English in the TUI.
This is metadata-only — it does not auto-create tasks on completion.

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

### Derived fields

Fields such as:
- `created by`
- `created at`
- `updated at`

are not stored and are calculated from git - the time and git user who created a tiki or the time it was last modified


## Doki files

Documents are any file in a Markdown format saved under `.doc/doki` directory. They can be organized in subdirectory
tree and include links between them or to external Markdown files