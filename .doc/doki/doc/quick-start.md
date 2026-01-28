# Quick start

## Markdown viewer

`tiki my-markdownfile` to view, edit and navigate markdown files in terminal.
All vim-like pager commands are supported in addition to:
- `Tab/Enter` to select and load a link in the document
- `e` to edit it in your favorite editor

## File and issue management

`cd` into your **git** repo and run `tiki`.
Move your tiki around the board with `Shift ←/Shift →`.
Make sure to press `?` for help.
Press `F1` to open a sample doc root. Follow links with `Tab/Enter`

## AI skills
You will be prompted to install skills for
- [Claude Code](https://code.claude.com)
- [Codex](https://openai.com/codex)
- [Opencode](https://opencode.ai)

if you choose to you can mention `tiki` in your prompts to create/find/edit your tikis
![Claude](assets/claude.png)

Happy tikking!

# tiki
Keep your tickets in your pockets!

`tiki` refers to a task or a ticket (hence tiki) stored in your **git** repo

- like a ticket it can have a status, priority, assignee, points, type and multiple tags attached to it
- they are essentially just Markdown files and you can use full Markdown syntax to describe a story or a bug
- they are stored in `.doc/tiki` subdirectory and are **git**-controlled - they are added to **git** when they are created,
  removed when they are done and the entire history is preserved in **git** repo
- because they are in **git** they can be perfectly synced up to the state of your repo or a branch
- you can use either the `tiki` CLI tool or any of the AI coding assistant to work with your tikis

# doki
Store your notes in remotes!

`doki` refers to any file in Markdown format that is stored in the `.doc/doki` subdirectory of the **git** repo.

- like tikis they are **git**-controlled and can be maintained in perfect sync with the repo state
- `tiki` CLI tool allows creating multiple doc roots like: Documentation, Brainstorming, Prompts etc.
- it also allows viewing and navigation (follow links)

# tiki TUI tool

`tiki` TUI tool allows creating, viewing, editing and deleting tikis as well as creating custom plugins to
view any selection, for example, Recent tikis, Architecture docs, Saved prompts, Security review, Future Roadmap
Read more by pressing `?` for help

# AI skills

`tiki` adds optional [agent skills](https://agentskills.io/home) to the repo upon initialization
If installed you can:

- work with [Claude Code](https://code.claude.com), [Codex](https://openai.com/codex), [Opencode](https://opencode.ai) by simply mentioning `tiki` or `doki` in your prompts
- create, find, modify and delete tikis using AI
- create tikis/dokis directly from Markdown files
- Refer to tikis or dokis when implementing with AI-assisted development - `implement tiki xxxxxxx`
- Keep a history of prompts/plans by saving prompts or plans with your repo