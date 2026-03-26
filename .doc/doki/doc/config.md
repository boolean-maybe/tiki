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
- `workflow.yaml` - Statuses and plugin/view definitions
- `new.md` - Custom task template

**Environment Variables**:
- `XDG_CONFIG_HOME` - Override config directory location (all platforms)
- `XDG_CACHE_HOME` - Override cache directory location (all platforms)

Example: To use a custom config location on macOS:
```bash
export XDG_CONFIG_HOME=~/my-config
tiki  # Will use ~/my-config/tiki/ for configuration
```

## Precedence and merging

`tiki` looks for configuration in three locations, from least specific to most specific:

1. **User config directory** (platform-specific, see [above](#configuration-directories)) - your personal defaults
2. **Project config directory** (`.doc/`) - shared with the team via git
3. **Current working directory** (`./`) - local overrides, useful during development

The more specific location always wins. This means each project can have its own workflow, statuses, and views that differ from your personal defaults. A design-team project might use statuses like "Draft / Review / Approved" while an engineering project uses "Backlog / In Progress / Done" — each defined in their own `.doc/workflow.yaml`.

### config.yaml merging

All `config.yaml` files found are merged together. A project config only needs to specify the values it wants to change — everything else is inherited from the user config. Missing values fall back to built-in defaults.

Search order: user config dir (base) → `.doc/config.yaml` (project) → cwd (highest priority).

### workflow.yaml merging

`workflow.yaml` is searched in all three locations. Files that exist are loaded and merged sequentially.

Search order: user config dir (base) → `.doc/workflow.yaml` (project) → cwd (highest priority).

**Statuses** — last file with a `statuses:` section wins (complete replacement). A project that defines its own statuses fully replaces the user-level defaults.

**Views (plugins)** — merged by name across files. The user config is the base; project and cwd files override individual fields:
- Non-empty fields in the override replace the base (description, key, colors, view mode)
- Non-empty arrays in the override replace the entire base array (lanes, actions, sort)
- Empty/zero fields in the override are ignored — the base value is kept
- Views that only exist in the override are appended

A project only needs to define the views or fields it wants to change. Everything else is inherited from your user config.

To disable all user-level views for a project, create a `.doc/workflow.yaml` with an explicitly empty views list:

```yaml
views: []
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
    description: "Move tiki to new status, search, create or delete"
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
    description: "Tasks waiting to be picked up, sorted by priority"
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
    description: "Tasks changed in the last 24 hours, most recent first"
    foreground: "#f4d6a6"
    background: "#5a3d1b"
    key: Ctrl-R
    lanes:
      - name: Recent
        columns: 4
        filter: NOW - UpdatedAt < 24hours
    sort: UpdatedAt DESC
  - name: Roadmap
    description: "Epics organized by Now, Next, and Later horizons"
    foreground: "#e2e8f0"
    background: "#2a5f5a"
    key: "F4"
    lanes:
      - name: Now
        columns: 1
        width: 25
        filter: type = 'epic' AND status = 'ready'
        action: status = 'ready'
      - name: Next
        columns: 1
        width: 25
        filter: type = 'epic' AND status = 'backlog' AND priority = 1
        action: status = 'backlog', priority = 1
      - name: Later
        columns: 2
        width: 50
        filter: type = 'epic' AND status = 'backlog' AND priority > 1
        action: status = 'backlog', priority = 2
    sort: Priority, Points DESC
    view: expanded
  - name: Help
    description: "Keyboard shortcuts, navigation, and usage guide"
    type: doki
    fetcher: internal
    text: "Help"
    foreground: "#bcbcbc"
    background: "#003399"
    key: "?"
  - name: Docs
    description: "Project notes and documentation files"
    type: doki
    fetcher: file
    url: "index.md"
    foreground: "#ff9966"
    background: "#2b3a42"
    key: "F2"
```