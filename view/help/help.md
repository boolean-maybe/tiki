# About

tiki is a lightweight issue-tracking, project management and knowledge base tool that uses git repo 
to store issues, stories and documentation.

- tiki uses Markdown files stored in `tiki` format under `.doc/tiki` subdirectory of a git repo
to track issues, stories or epics. Press `Tab` then `Enter` to select this link: [tiki](tiki.md) and read about `tiki` format
- Project-related documentation is stored under `.doc/doki` also in Markdown format. They can be linked/back-linked
for easier navigation. 

>Since they are stored in git they are automatically versioned and can be perfectly synced to the current
state of the repo or its git branch. Also, all past versions and deleted items remain in git history of the repo

## Board

Board is a simple Kanban-style board where tikis can be moved around with `Shift-Right` and `Shift-Left`
As tikis are moved their status changes correspondingly. 
Tikis can be opened for viewing or editing or searched by title. 

To quickly capture an idea - hit `n` in the board or any tiki view, type in the title and press Enter
You can also edit its status, type and other fields, or open the source file directly for editing in your favorite editor


## Documentation

Documentation is essentially a Wiki-style knowledge base stored alongside the project files
Documentation and various other files such as prompts can also be stored under git version control
The documentation can be organized using Markdown links and navigated in the `tiki` cli using Tab/Shift-Tab and Enter

## AI

Since Markdown is an AI-native format issues and documentation can easily be created and maintained using AI tools.
tiki can optionally install skills to enable AI tools such as `claude`, `codex` or `opencode` to understand its
format. Try:

>create a tiki from @my-markdown.md with title "Fix UI bug"

or:

>mark tiki ABC123 as complete

## Customization

Press `Tab` then `Enter` to read [customize](view.md) to understand how to customize or extend tiki with your own plugins

## Configuration

tiki can be configured via `config.yaml` file stored in the same directory where executable is installed

## Header

- Context help showing keyboard shortcuts for the current view
- Various statistics - tiki count, git branch and current user name
- Burndown chart - number of incomplete tikis remaining