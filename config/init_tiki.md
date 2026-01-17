---
id: TIKI-xxxxxx
title: Welcome to tiki-land!
type: story
status: todo
priority: 0
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

## Git managed

`tikis` (short for tickets) are just **Markdown** files in your repository

🌳 /projects/my-app
├─ 📁 .doc
│  └─ 📁 tiki
│     ├─ 📝 tiki-k3x9m2.md
│     ├─ 📝 tiki-7wq4na.md
│     ├─ 📝 tiki-p8j1fz.md
│     └─ 📝 tiki-5r2bvh.md
├─ 📁 src
│  ├─ 📁 components
│  │  ├─ 📜 Header.tsx
│  │  ├─ 📜 Footer.tsx
│  │  └─ 📝 README.md
├─ 📝 README.md
├─ 📋 package.json
└─ 📄 LICENSE

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

> hey Claude show me a tiki TIKI-m7n2xk
> change it from story to a bug
> and assign priority 1


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