# tiki

`tiki` is a simple and lightweight way to keep your tasks, prompts, documents, ideas, scratchpads in your project **git** repo

![Intro](assets/intro.png)

Markdown is the new go-to format for everything, it's simple, efficient, human and AI native - project management, 
documentation, brainstorming ideas, incomplete implementations, AI prompts and plans and what not are saved as Markdown files. 
Stick them in your repo. Keep around for as long as you need. Find them back in **git** history. Make issues out of them
and take them through an agile lifecycle. `tiki` helps you save and organize these files:

- Standalone Markdown viewer - view and edit Markdown files, navigate to local/external/GitHub/GitLab links, edit and save
- Keep, search, view and version Markdown files in the git repo
- Wiki-style documentation with multiple entry points
- Keep a to-do list with priorities, status, assignee and size
- Issue management with Kanban/Scrum style board and burndown chart
- Plugin-first architecture - user-defined plugins with filters and actions like Backlog, Recent, Roadmap
- AI skills to enable [Claude Code](https://code.claude.com), [Codex](https://openai.com/codex), [Opencode](https://opencode.ai) work with natural language commands like
  "create a tiki from @my-file.md"
  "mark tiki ABC123 as complete"

## Installation

### Mac OS and Linux
```bash
curl -fsSL https://raw.githubusercontent.com/boolean-maybe/tiki/main/install.sh | bash
```


### Mac OS via brew
```bash
brew install boolean-maybe/tap/tiki
```

### Windows
```powershell
# Windows PowerShell
iwr -useb https://raw.githubusercontent.com/boolean-maybe/tiki/main/install.ps1 | iex
```

### Manual install

Download the latest distribution from the [releases page](https://github.com/boolean-maybe/tiki/releases) 
and simply copy the `tiki` executable to any location and make it available via `PATH`

### Verify installation
```bash
tiki --version
```

## Terminal Requirements

`tiki` CLI tool works best with modern terminal emulators that support:
- **TrueColor (24-bit color)** for gradient rendering
- **UTF-8 encoding** for proper character display
- Standard ANSI escape sequences

### Recommended Terminals
- **macOS**: iTerm2, kitty, Alacritty, or default Terminal.app
- **Linux**: kitty, Alacritty, GNOME Terminal, Konsole, or any xterm-256color compatible terminal
- **Windows**: Windows Terminal, ConEmu, or Alacritty

### Terminal Configuration
For best results, ensure your terminal is set to:
- TERM environment variable: `xterm-256color` or better (e.g., `xterm-truecolor`)
- UTF-8 encoding enabled

If colors don't display correctly, try setting:
```bash
export TERM=xterm-256color
```

## Quick start

### Markdown viewer

`tiki my-markdownfile` to view, edit and navigate markdown files in terminal.
All vim-like pager commands are supported in addition to:
- `Tab/Enter` to select and load a link in the document
- `e` to edit it in your favorite editor

### File and issue management

`cd` into your **git** repo and run `tiki`.
Move your tiki around the board with `Shift ←/Shift →`.
Make sure to press `?` for help. 
Press `F1` to open a sample doc root. Follow links with `Tab/Enter`

### AI skills
You will be prompted to install skills for 
- [Claude Code](https://code.claude.com)
- [Codex](https://openai.com/codex)
- [Opencode](https://opencode.ai)

if you choose to you can mention `tiki` in your prompts to create/find/edit your tikis
![Claude](assets/claude.png)

Happy tikking! 

## tiki
Keep your tickets in your pockets!

`tiki` refers to a task or a ticket (hence tiki) stored in your **git** repo

- like a ticket it can have a status, priority, assignee, points, type and multiple tags attached to it
- they are essentially just Markdown files and you can use full Markdown syntax to describe a story or a bug
- they are stored in `.doc/tiki` subdirectory and are **git**-controlled - they are added to **git** when they are created,
removed when they are done and the entire history is preserved in **git** repo
- because they are in **git** they can be perfectly synced up to the state of your repo or a branch
- you can use either the `tiki` CLI tool or any of the AI coding assistant to work with your tikis

## doki
Store your notes in remotes!

`doki` refers to any file in Markdown format that is stored in the `.doc/doki` subdirectory of the **git** repo. 

- like tikis they are **git**-controlled and can be maintained in perfect sync with the repo state
- `tiki` CLI tool allows creating multiple doc roots like: Documentation, Brainstorming, Prompts etc.
- it also allows viewing and navigation (follow links)

## tiki CLI tool

`tiki` CLI tool allows creating, viewing, editing and deleting tikis as well as creating custom plugins to 
view any selection, for example, Recent tikis, Architecture docs, Saved prompts, Security review, Future Roadmap
Read more by pressing `?` for help 

## AI skills

`tiki` adds optional [agent skills](https://agentskills.io/home) to the repo upon initialization
If installed you can:

- work with [Claude Code](https://code.claude.com), [Codex](https://openai.com/codex), [Opencode](https://opencode.ai) by simply mentioning `tiki` or `doki` in your prompts
- create, find, modify and delete tikis using AI
- create tikis/dokis directly from Markdown files
- Refer to tikis or dokis when implementing with AI-assisted development - `implement tiki xxxxxxx`
- Keep a history of prompts/plans by saving prompts or plans with your repo

## Feedback

Feedback is always welcome! Whether you have an improvement request, a feature suggestion
or just chat:
- use GitHub issues to submit and issue or a feature request
- use GitHub discussions for everything else

to contribute:
[Contributing](CONTRIBUTING.md)

## Badges

![Build Status](https://github.com/boolean-maybe/tiki/actions/workflows/go.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/boolean-maybe/tiki)](https://goreportcard.com/report/github.com/boolean-maybe/tiki)
[![Go Reference](https://pkg.go.dev/badge/github.com/boolean-maybe/tiki.svg)](https://pkg.go.dev/github.com/boolean-maybe/tiki)