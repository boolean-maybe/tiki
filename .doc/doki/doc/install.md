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