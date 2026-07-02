# Quick start

## Markdown viewer

`tiki my-markdownfile` to view, edit and navigate markdown files in terminal.
All vim-like pager commands are supported in addition to:
- `Tab/Enter` to select and load a link in the document
- `e` to edit it in your favorite editor

## File and issue management

`cd` into a directory of Markdown and run `tiki` — there is no setup step.
Or check out a demo project and play around:
```
cd /tmp && tiki demo
```

Move a tiki around the board with `Shift ←/Shift →`.
Press `?` to open the Action Palette — it lists all available actions with their shortcuts.
Press `F2` to open the Docs view and follow links with `Tab/Enter`.

## AI skills
Copy the bundled skill into your AI tool's skills directory to enable
- [Claude Code](https://code.claude.com)
- [Gemini CLI](https://github.com/google-gemini/gemini-cli)
- [Codex](https://openai.com/codex)
- [Opencode](https://opencode.ai)

once installed, mention documents by id in your prompts to create, find, or edit them.
See [AI collaboration](ai.md) for the file location and per-tool target paths.
![Claude](assets/claude.png)

Happy tikking!

# The tiki model

`tiki` is a viewer over the current directory. Every item `tiki` touches is a Markdown file with
YAML frontmatter and a bare `id` — a **tiki**:

```md
---
id: ABC123
title: Implement search
status: backlog
priority: medium-high
---

Markdown body.
```

- Tikis live anywhere under the current directory — the folder structure is yours to organize. Identity
  is in the `id` frontmatter field, not in the file path.
- A tiki carrying workflow-declared fields (`status`, `type`, `priority`, `points`, etc.) appears on
  every view whose lane filter matches those fields.
- A tiki with only `id` and `title` is reachable by id or path, rendered by wiki and detail views,
  and falls outside views that require workflow fields.
- Plain Markdown with no frontmatter `id` is fine too — it is wiki-viewable by path and never warned about.
- When the directory is a git repository, `tiki` reads history (commit times, authorship) but never writes
  to git — staging and committing stay entirely in your hands.

See [Document format](tiki-format.md) for the full frontmatter reference.

# Using tiki

`tiki` exposes its workspace in two ways:

- The **CLI** — `tiki exec '<ruki-statement>'` runs queries and updates non-interactively;
  `echo ... | tiki` captures pipes as documents.
- The **TUI** — run `tiki` with no arguments in a directory of Markdown. Lets you create, view, edit, and
  delete documents, and compose custom views (Recent, Architecture notes, Saved prompts, Security review,
  Future roadmap, …) via `workflow.yaml`. Press `?` for the Action Palette.

# AI skills

`tiki` ships optional [agent skills](https://agentskills.io/home) you can copy into your AI tool (see
[AI collaboration](ai.md)). Once installed you can:

- work with [Claude Code](https://code.claude.com),
  [Gemini CLI](https://github.com/google-gemini/gemini-cli), [Codex](https://openai.com/codex), and
  [Opencode](https://opencode.ai) by mentioning documents or ids in your prompts
- create, find, modify, and delete documents using AI
- create documents directly from Markdown files
- reference documents by id when implementing with AI-assisted development — `implement ABC123`
- keep a history of prompts/plans by saving them as documents alongside your repo
