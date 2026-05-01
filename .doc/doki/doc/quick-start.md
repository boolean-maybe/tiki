# Quick start

## Markdown viewer

`tiki my-markdownfile` to view, edit and navigate markdown files in terminal.
All vim-like pager commands are supported in addition to:
- `Tab/Enter` to select and load a link in the document
- `e` to edit it in your favorite editor

## File and issue management

Run `tiki init my-directory` to initialize a project, then `cd my-directory` and run `tiki` to start.
Or check out a demo project and play around:
```
cd /tmp && tiki demo
```

Move a task around the board with `Shift ←/Shift →`.
Press `?` to open the Action Palette — it lists all available actions with their shortcuts.
Press `F2` to open the Docs view and follow links with `Tab/Enter`.

## AI skills
You will be prompted to install skills for
- [Claude Code](https://code.claude.com)
- [Gemini CLI](https://github.com/google-gemini/gemini-cli)
- [Codex](https://openai.com/codex)
- [Opencode](https://opencode.ai)

once installed, mention documents by id in your prompts to create, find, or edit them.
![Claude](assets/claude.png)

Happy tikking!

# The managed document model

`tiki` manages a single Markdown workspace under `.doc/`. Everything `tiki` touches is a Markdown file with
YAML frontmatter and a required bare `id`:

```md
---
id: ABC123
title: Implement search
status: backlog
priority: 2
---

Markdown body.
```

- Documents live anywhere under `.doc/` — the folder structure is yours to organize. Identity is in the
  `id` frontmatter field, not in the file path.
- **Workflow documents** carry `status`, `type`, `priority`, or `points` and show up on board/list views.
- **Plain documents** carry only `id` and `title` (and optional custom fields). They are reachable by id
  or path, rendered by wiki and detail views, and invisible to workflow views.
- Everything is git-controlled — added, updated, and removed along with your code. History is preserved.

See [Document format](tiki-format.md) for the full frontmatter reference.

# Using tiki

`tiki` exposes its workspace in two ways:

- The **CLI** — `tiki exec '<ruki-statement>'` runs queries and updates non-interactively;
  `echo ... | tiki` captures pipes as documents.
- The **TUI** — run `tiki` with no arguments in an initialized project. Lets you create, view, edit, and
  delete documents, and compose custom views (Recent, Architecture notes, Saved prompts, Security review,
  Future roadmap, …) via `workflow.yaml`. Press `?` for the Action Palette.

# AI skills

`tiki` adds optional [agent skills](https://agentskills.io/home) to the repo upon initialization.
If installed you can:

- work with [Claude Code](https://code.claude.com),
  [Gemini CLI](https://github.com/google-gemini/gemini-cli), [Codex](https://openai.com/codex), and
  [Opencode](https://opencode.ai) by mentioning documents or ids in your prompts
- create, find, modify, and delete documents using AI
- create documents directly from Markdown files
- reference documents by id when implementing with AI-assisted development — `implement ABC123`
- keep a history of prompts/plans by saving them as documents alongside your repo
