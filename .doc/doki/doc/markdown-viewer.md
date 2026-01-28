# Markdown viewer

## Open Markdown
`tiki` can be used as a navigable Markdown viewer. A Markdown file can be opened via:

- local file
```
tiki my-file.md
```

- HTTP link
```
tiki https://raw.githubusercontent.com/mxstbr/markdown-test-file/refs/heads/master/TEST.md
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

## Navigate links

with a Markdown file open press `Tab/Shift-Tab` to select next/previous link in the file
then press Enter to load the linked file
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