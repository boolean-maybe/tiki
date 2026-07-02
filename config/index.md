# Welcome

This is your project's wiki — a tree of Markdown documents tracked alongside your
code. Press `Tab/Shift-Tab` to move between links and `Enter` to follow them.

- [What is tiki](#what-is-tiki)
- [How content is organized](#how-content-is-organized)
- [Adding views](#adding-views)
- [Markdown reference](#markdown-reference)

![Markdown](assets/markdown.png)

## What is tiki

`tiki` is a terminal workspace for a single Markdown directory. Every file under
`.doc/` is a **document** — identified by a bare 6-character `id` in its
frontmatter. Documents with workflow fields (`status`, `priority`, …) show up on
boards and lists; documents with only `id` and `title` act as wiki pages.

Everything is stored in git, so history, blame, and branches work the same way
they do for code.

## How content is organized

```text
my-project/
├── .doc/
│   ├── workflow.yaml      # views, fields, actions
│   ├── index.md           # this file
│   ├── onboarding.md      # a workflow tiki
│   ├── architecture.md    # a wiki page
│   └── assets/
│       └── markdown.png
└── src/
```

The directory layout under `.doc/` is yours — flat or nested, organized however
suits your project. Filenames are free; identity lives in the frontmatter `id:`,
so files can be renamed or moved without breaking [[wikilinks]] or `dependsOn`
references.

Follow this [link to a second page](linked.md) to see cross-document navigation.

## Adding views

Wiki entry points are declared in `.doc/workflow.yaml` and bound to a key:

```yaml
views:
  - name: brainstorm
    label: Brainstorm
    kind: wiki
    path: brainstorm.md
    key: "F6"
```

Restart `tiki` and `F6` opens the new view.

## Markdown reference

### Headings

# H1
## H2
### H3

### Emphasis

**bold**, *italic*, ***bold italic***

### Lists

- Item 1
- Item 2
  - Nested
1. First
2. Second

### Code

Inline `code` and fenced blocks:

```python
def hello():
    print("hello, tiki")
```

### Tables

| Field | Type | Notes |
|---|---|---|
| id | string | bare 6-char id |
| title | string | required |

### Tasks

- [x] Done
- [ ] Not done
