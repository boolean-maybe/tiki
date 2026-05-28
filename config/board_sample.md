---
id: XXXXXX
title: Welcome to tiki-land!
type: story
status: inbox
priority: medium-high
points: 7
tags:
  - info
  - ideas
  - setup
---

# Hello! こんにちは

`tikis` are a lightweight issue-tracking and project management tool
check it out: https://github.com/boolean-maybe/tiki

***

## Features
- [x] stored in git and always in sync
- [x] built-in terminal UI
- [x] AI native
- [x] rich **Markdown** format

![Markdown](assets/markdown.png)
## Git managed

`tikis` (short for tickets) are just **Markdown** files in your repository

```
/projects/my-app
├── .doc/
│   ├── workflow.yaml
│   ├── onboarding.md
│   ├── auth-refactor.md
│   ├── architecture.md
│   ├── ideas.md
│   └── assets/
│       └── markdown.png
├── src/
│   ├── components/
│   │   ├── Header.tsx
│   │   ├── Footer.tsx
│   │   └── README.md
│   └── ...
├── README.md
├── package.json
└── LICENSE
```

Filenames are free — identity lives in the `id:` frontmatter field.

## Built-in terminal UI

A built-in `tiki` command displays a nice Scrum/Kanban board and a searchable Backlog view

| Ready  | In progress | Waiting | Completed |
|--------|-------------|---------|-----------|
| Task 1 | Task 1      |         | Task 3    |
| Task 4 | Task 5      |         |           |
| Task 6 |             |         |           |

## AI native

since they are simple **Markdown** files they can also be easily manipulated via AI. For example, you can
use Claude Code with skills to search, create, view, update and delete `tikis`

> hey Claude show me tiki M7N2XK
> change it from story to a bug
> and set priority to high


## Rich Markdown format

Since a tiki description is in **Markdown** you can use all of its rich formatting options

1. Headings
1. Emphasis
   - bold
   - italic
1. Lists
1. Links
1. Blockquotes

You can also add a code block:

```python
def calculate_average(numbers):
    if not numbers:
        return 0
    return sum(numbers) / len(numbers)
```

Happy tiking!