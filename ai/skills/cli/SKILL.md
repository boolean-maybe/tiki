---
name: cli
description: "Initialize tiki projects, manage workflow configurations, and create view plugins. Use when the user asks to set up a new project with tiki init, install or reset workflows, or define kanban board and documentation plugins in workflow.yaml."
allowed-tools: Read, Grep, Glob, Edit, Write, Bash(tiki init:*), Bash(tiki workflow:*), Bash(tiki demo:*)
---

# CLI Utilities

Manage tiki project setup, workflow configuration, and view plugins through the tiki CLI.

## Commands

| Command | Purpose |
|---------|---------|
| `tiki init` | Initialize a new tiki project |
| `tiki workflow` | Manage workflow configurations |
| `tiki demo` | Clone and demo a sample project |
| `tiki exec` | Execute ruki statements (see the `tiki` skill) |

## Initialize a project

```sh
tiki init
```

Creates project structure, initializes a git repo, generates sample tasks, and optionally installs AI skills (claude, gemini, etc.).

## Manage workflows

```sh
tiki workflow install <name>    # install a workflow
tiki workflow reset             # reset to defaults
tiki workflow describe          # show current workflow
```

Use the `--scope` flag to target `global`, `local`, or `current` scope.

## Create a view plugin

View plugins are custom workspace views defined in `workflow.yaml` under the `plugins` key. Two types exist:

### Tiki plugin (kanban/task board)

Defines lanes with ruki filter expressions, actions, and hotkey activation.

```yaml
plugins:
  - name: sprint-board
    description: "Current sprint kanban"
    type: tiki
    activate: Alt+1
    lanes:
      - name: To Do
        filter: 'status = "ready"'
      - name: In Progress
        filter: 'status = "in-progress"'
      - name: Done
        filter: 'status = "done"'
```

### Doki plugin (documentation view)

Provides markdown navigation with images and mermaid diagram support.

```yaml
plugins:
  - name: docs
    description: "Project documentation"
    type: doki
    activate: Alt+2
```

## Important

- Plugin definitions live in `workflow.yaml` under the `plugins` key
- Each plugin needs a unique `name` and `activate` hotkey
- Lanes support ruki filter expressions for dynamic content
- Actions within lanes can trigger ruki statements
