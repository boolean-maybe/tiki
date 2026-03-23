# Configuration

## Configuration files

- `config-dir/config.yaml` main configuration file
- `config-dir/workflow.yaml` plugins/view configuration
- `config-dir/new.md` new tiki template - will be used when a new tiki is created

## Configuration directories

`tiki` uses platform-standard directories for configuration while keeping tasks and documentation project-local:

**User Configuration** (global settings, plugins, templates):
- **Linux**: `~/.config/tiki` (or `$XDG_CONFIG_HOME/tiki`)
- **macOS**: `~/.config/tiki` (preferred) or `~/Library/Application Support/tiki` (fallback)
- **Windows**: `%APPDATA%\tiki`

Files stored here:
- `config.yaml` - User-global configuration
- `new.md` - Custom task template
- Plugin definition files (e.g., custom plugin YAML files)

**User Cache** (temporary data):
- **Linux**: `~/.cache/tiki` (or `$XDG_CACHE_HOME/tiki`)
- **macOS**: `~/Library/Caches/tiki`
- **Windows**: `%LOCALAPPDATA%\tiki`

**Project-Local** (tasks and documentation, in git):
- `.doc/tiki/` - Task files (tikis)
- `.doc/doki/` - Documentation files (dokis)
- `.doc/tiki/config.yaml` - Project-specific config (overrides user config)

**Configuration Priority** (first found wins):
1. Project config directory (`.doc/tiki/config.yaml`) - highest priority
2. User config directory (`~/.config/tiki/config.yaml`)
3. Current directory (`./config.yaml` - development only)

**Environment Variables**:
- `XDG_CONFIG_HOME` - Override config directory location (all platforms)
- `XDG_CACHE_HOME` - Override cache directory location (all platforms)

Example: To use a custom config location on macOS:
```bash
export XDG_CONFIG_HOME=~/my-config
tiki  # Will use ~/my-config/tiki/ for configuration
```

### config.yaml

Example `config.yaml` with available settings:

```yaml
# Header settings
header:
  visible: true             # Show/hide header: true, false

# Tiki settings
tiki:
  maxPoints: 10             # Maximum story points for tasks
  maxImageRows: 40          # Maximum rows for inline images (Kitty protocol)

# Logging settings
logging:
  level: error              # Log level: "debug", "info", "warn", "error"

# Plugin settings are defined in their YAML file but can be overridden here
Kanban:
  view: expanded            # Default board view: "compact", "expanded"
Backlog:
  view: compact

# Appearance settings
appearance:
  theme: auto               # Theme: "auto" (detect from terminal), "dark", "light"
  gradientThreshold: 256    # Minimum terminal colors for gradient rendering
                            # Options: 16, 256, 16777216 (truecolor)
                            # Gradients disabled if terminal has fewer colors
                            # Default: 256 (works well on most terminals)
  codeBlock:
    theme: dracula           # Chroma syntax theme for code blocks
                             # Examples: "dracula", "monokai", "catppuccin-macchiato"
    background: "#282a36"    # Code block background color (hex or ANSI e.g. "236")
    border: "#6272a4"        # Code block border color (hex or ANSI e.g. "244")
```

### workflow.yaml

For detailed instructions see [Customization](plugin.md)

Example `workflow.yaml`:

```yaml
statuses:
  - key: backlog
    label: Backlog
    emoji: "📥"
    default: true
  - key: ready
    label: Ready
    emoji: "📋"
    active: true
  - key: in_progress
    label: "In Progress"
    emoji: "⚙️"
    active: true
  - key: review
    label: Review
    emoji: "👀"
    active: true
  - key: done
    label: Done
    emoji: "✅"
    done: true

views:
  - name: Kanban
    foreground: "#87ceeb"
    background: "#25496a"
    key: "F1"
    lanes:
      - name: Ready
        filter: status = 'ready' and type != 'epic'
        action: status = 'ready'
      - name: In Progress
        filter: status = 'in_progress' and type != 'epic'
        action: status = 'in_progress'
      - name: Review
        filter: status = 'review' and type != 'epic'
        action: status = 'review'
      - name: Done
        filter: status = 'done' and type != 'epic'
        action: status = 'done'
    sort: Priority, CreatedAt
  - name: Backlog
    foreground: "#5fff87"
    background: "#0b3d2e"
    key: "F3"
    lanes:
      - name: Backlog
        columns: 4
        filter: status = 'backlog' and type != 'epic'
    actions:
      - key: "b"
        label: "Add to board"
        action: status = 'ready'
    sort: Priority, ID
  - name: Recent
    foreground: "#f4d6a6"
    background: "#5a3d1b"
    key: Ctrl-R
    lanes:
      - name: Recent
        columns: 4
        filter: NOW - UpdatedAt < 24hours
    sort: UpdatedAt DESC
  - name: Roadmap
    foreground: "#e2e8f0"
    background: "#2a5f5a"
    key: "F4"
    lanes:
      - name: Now
        columns: 1
        filter: type = 'epic' AND status = 'ready'
        action: status = 'ready'
      - name: Next
        columns: 1
        filter: type = 'epic' AND status = 'backlog' AND priority = 1
        action: status = 'backlog', priority = 1
      - name: Later
        columns: 2
        filter: type = 'epic' AND status = 'backlog' AND priority > 1
        action: status = 'backlog', priority = 2
    sort: Priority, Points DESC
    view: expanded
  - name: Help
    type: doki
    fetcher: internal
    text: "Help"
    foreground: "#bcbcbc"
    background: "#003399"
    key: "?"
  - name: Docs
    type: doki
    fetcher: file
    url: "index.md"
    foreground: "#ff9966"
    background: "#2b3a42"
    key: "F2"
```