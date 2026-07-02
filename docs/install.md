# Installation

## Mac OS and Linux
```bash
curl -fsSL https://raw.githubusercontent.com/boolean-maybe/tiki/main/install.sh | bash
```


## Mac OS via brew
```bash
brew install boolean-maybe/tap/tiki
```

## Windows
```powershell
# Windows PowerShell
iwr -useb https://raw.githubusercontent.com/boolean-maybe/tiki/main/install.ps1 | iex
```

## Manual install

Download the latest distribution from the [releases page](https://github.com/boolean-maybe/tiki/releases)
and simply copy the `tiki` executable to any location and make it available via `PATH`

## Build from source

```bash
GOBIN=$HOME/.local/bin go install github.com/boolean-maybe/tiki@latest
```

## Verify installation
```bash
tiki --version
```

## Run it

There is no project setup step. Run `tiki` in any directory that contains Markdown files:

```bash
cd /path/to/your/repo
tiki
```

### Scan scope

The current working directory is the scan root. `tiki` recursively loads every `.md` file beneath it,
keying documents by their frontmatter `id`. The scan skips:

- `.git/` and any other dotted directory
- every path matched by a `.gitignore` at the scan root
- every path matched by a `.tikiignore` at the scan root (same gitignore syntax — tiki-only exclusions)

When the directory is a git repository, `tiki` reads commit times and authorship automatically; it never
writes to git. To try a custom workflow, drop a `workflow.yaml` in the directory — see
[Configuration](config.md#workflowyaml). To enable AI skills, copy the bundled skill manually — see
[AI collaboration](ai.md).

# Terminal Requirements

`tiki` CLI tool works best with modern terminal emulators that support:
- **256-color or TrueColor (24-bit)** support
- **UTF-8 encoding** for proper character display
- Standard ANSI escape sequences

## Recommended Terminals
- **macOS**: iTerm2, kitty, Ghostty, Alacritty, or default Terminal.app
- **Linux**: kitty, Ghostty, Alacritty, GNOME Terminal, Konsole, or any xterm-256color compatible terminal
- **Windows**: Windows Terminal, ConEmu, or Alacritty

## Terminal Configuration
For best results, ensure your terminal is set to:
- TERM environment variable: `xterm-256color` or better (e.g., `xterm-truecolor`)
- UTF-8 encoding enabled

If colors don't display correctly, try setting:
```bash
export TERM=xterm-256color
```