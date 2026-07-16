# Markdown viewer

![Markdown viewer demo](markdown-viewer.gif)
see [requirements](image-requirements.md) for supported terminals, SVG and diagrams support

## Open Markdown
`tiki` can be used as a navigable Markdown viewer. A Markdown file can be opened via:

- local file
```
tiki my-file.md
```

- HTTP link
```
tiki https://github.com/boolean-maybe/tiki/blob/main/testdata/go-concurrency.md
```

- From STDIN
```
echo "# Markdown" | tiki -
```

- README from GitHub/GitLab
```
tiki github.com/boolean-maybe/tiki
```

press `q` to quit

## Browse markdown files with `Ctrl-O`

From anywhere in the app press `Ctrl-O` to open a file tree of every `.md` file under the current
directory. The overlay is a filterable, collapsible directory tree:

- type to fuzzy-filter by path; matching files stay visible and their parent folders auto-expand
- `↑/↓` move, `→` expands a folder, `←` collapses it, `Ctrl-U` clears the filter
- `Enter` on a folder toggles it; `Enter` on a file opens it in the markdown viewer
- `Esc` closes the overlay and restores focus to the previous view

Clearing the filter restores whatever folders you had manually expanded before you started typing.

## Open image files

Likewise, you can open image files:
```
tiki my-file.png
```
or 

```
tiki https://bellard.org/bpg/2.png
```

## Navigate links

with a Markdown file open press `Tab/Shift-Tab` to select next/previous link in the file
then press Enter to load the linked file or go to a linked section within the same file
to go back/forward in history use `Left/Right` or `Alt-Left/Alt-Right`

## Pager commands

`tiki` supports the most common `vim`-like commands:

Vertical Navigation
- Line Down: j, Down, Enter
- Line Up: k, Up
- Page Down: Ctrl-F, PageDown
- Page Up: Ctrl-B, PageUp
- Top: g, Home
- Bottom: G, End

Horizontal Navigation
- Left: h, Left
- Right: l, Right


## Edit and save

Press `e` to edit the raw Markdown source file in editor
