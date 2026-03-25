# Image display requirements

## terminal

Images are rendered using the [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/).
The terminal emulator must support this protocol for images to be displayed.

Terminals known to support it include:

- [Kitty](https://sw.kovidgoyal.net/kitty/)
- [WezTerm](https://wezfurlong.org/wezterm/)
- [Ghostty](https://ghostty.org/)
- [iTerm2](https://iterm2.com/) 3.6 and later

In unsupported terminals, images are replaced with their alt text.

## SVG images

Rendering SVG files requires [`resvg`](https://github.com/linebender/resvg) to be installed and available in `PATH`.

```
# macOS
brew install resvg

# Cargo
cargo install resvg
```

If `resvg` is not found, SVG images are not displayed.

## Mermaid diagrams

Rendering Mermaid fenced code blocks requires [`mmdc`](https://github.com/mermaid-js/mermaid-cli) (Mermaid CLI) to be installed and available in `PATH`.

```
npm install -g @mermaid-js/mermaid-cli
```

If `mmdc` is not found, Mermaid blocks are displayed as plain code blocks.

## Graphviz diagrams

Rendering fenced code blocks tagged `` ```dot `` or `` ```graphviz `` requires [`dot`](https://graphviz.org/) (from the Graphviz suite) to be installed and available in `PATH`.

```
# macOS
brew install graphviz

# Debian / Ubuntu
sudo apt install graphviz

# Fedora
sudo dnf install graphviz
```

If `dot` is not found, Graphviz blocks are displayed as plain code blocks.
