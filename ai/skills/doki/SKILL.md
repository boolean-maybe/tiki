---
name: doki
description: "View, create, update, and delete dokis — project documentation files stored as Markdown in .doc/doki/. Use when the user asks to manage, list, read, edit, or remove project documentation entries or wiki pages."
allowed-tools: Read, Grep, Glob, Edit, Write, Bash(git add:*), Bash(git rm:*)
---

# Doki

A doki is a Markdown file stored in the project `.doc/doki/` directory, forming a wiki-style documentation system. Dokis are organized hierarchically using subdirectories.

If `.doc/doki/` does not exist, prompt the user before creating it.

## File structure

```
.doc/doki/
├── getting-started.md
├── architecture.md
└── guides/
    ├── setup.md
    └── deployment.md
```

- Filenames: lowercase, hyphen-separated (e.g., `getting-started.md`)
- No YAML frontmatter required — dokis are plain Markdown
- Subdirectories organize topics hierarchically

## View

List available dokis:

```sh
ls .doc/doki/
```

Read a specific doki:

```sh
# Use the Read tool on the file path
.doc/doki/getting-started.md
```

## Create

1. Confirm `.doc/doki/` exists (create if user approves)
2. Choose a descriptive, hyphen-separated filename
3. Write Markdown content to `.doc/doki/<name>.md`
4. Stage with `git add .doc/doki/<name>.md`

Example:

```markdown
# Deployment Guide

Steps to deploy the application to production.

## Prerequisites
...
```

## Update

1. Read the existing doki with the Read tool
2. Apply changes with the Edit tool
3. Stage with `git add .doc/doki/<name>.md`

## Delete

1. Confirm deletion with the user
2. Remove with `git rm .doc/doki/<name>.md`

## Features

- Supports cross-document links between dokis
- Renders images and mermaid diagrams in the TUI
- Navigation: `Tab` to browse, `Enter` to open, `Alt+Left/Right` to go back/forward
- All dokis are tracked in git version control
