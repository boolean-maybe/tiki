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

### new.md (task template)

`new.md` is searched in the same three locations but is **not merged** — the single highest-priority file found wins. If a project provides `.doc/new.md`, it completely replaces the user-level template. If no `new.md` is found anywhere, a built-in embedded template is used.

Search order: user config dir → `.doc/new.md` (project) → cwd. Last match wins.

### workflow.yaml merging

`workflow.yaml` is searched in all three locations. Files that exist are loaded and merged sequentially.

Search order: user config dir (base) → `.doc/workflow.yaml` (project) → cwd (highest priority).

**Statuses** — last file with a `statuses:` section wins (complete replacement). A project that defines its own statuses fully replaces the user-level defaults.

**Views (plugins)** — merged by name across files. The user config is the base; project and cwd files override individual fields:
- Non-empty fields in the override replace the base (description, key, view mode)
- Non-empty arrays in the override replace the entire base array (lanes, actions)
- Empty/zero fields in the override are ignored — the base value is kept
- Views that only exist in the override are appended

**Global plugin actions** (`views.actions`) — merged by key across files. If two files define a global action with the same key, the later file's action wins. Global actions are appended to each tiki plugin's action list; per-plugin actions with the same key take precedence.

A project only needs to define the views or fields it wants to change. Everything else is inherited from your user config.

To disable all user-level views for a project, create a `.doc/workflow.yaml` with an explicitly empty views list:

```yaml
views:
  plugins: []
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
  theme: auto               # Theme: "auto" (detect from terminal), "dark", "light",
                            # or a named theme: "dracula", "tokyo-night", "gruvbox-dark",
                            # "catppuccin-mocha", "solarized-dark", "nord", "monokai",
                            # "one-dark", "catppuccin-latte", "solarized-light",
                            # "gruvbox-light", "github-light"
  gradientThreshold: 256    # Minimum terminal colors for gradient rendering
                            # Options: 16, 256, 16777216 (truecolor)
                            # Gradients disabled if terminal has fewer colors
                            # Default: 256 (works well on most terminals)

# AI agent integration
ai:
  agent: claude              # AI tool for chat: "claude", "gemini", "codex", "opencode"
                             # Enables AI collaboration features
                             # Omit or leave empty to disable
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
  actions:
    - key: "a"
      label: "Assign to me"
      action: update where id = id() set assignee=user()
  plugins:
    - name: Kanban
      description: "Move tiki to new status, search, create or delete"
      key: "F1"
      lanes:
        - name: Ready
          filter: select where status = "ready" and type != "epic" order by priority, createdAt
          action: update where id = id() set status="ready"
        - name: In Progress
          filter: select where status = "inProgress" and type != "epic" order by priority, createdAt
          action: update where id = id() set status="inProgress"
        - name: Review
          filter: select where status = "review" and type != "epic" order by priority, createdAt
          action: update where id = id() set status="review"
        - name: Done
          filter: select where status = "done" and type != "epic" order by priority, createdAt
          action: update where id = id() set status="done"
    - name: Backlog
      description: "Tasks waiting to be picked up, sorted by priority"
      key: "F3"
      lanes:
        - name: Backlog
          columns: 4
          filter: select where status = "backlog" and type != "epic" order by priority, id
      actions:
        - key: "b"
          label: "Add to board"
          action: update where id = id() set status="ready"
    - name: Recent
      description: "Tasks changed in the last 24 hours, most recent first"
      key: Ctrl-R
      lanes:
        - name: Recent
          columns: 4
          filter: select where now() - updatedAt < 24hour order by updatedAt desc
    - name: Roadmap
      description: "Epics organized by Now, Next, and Later horizons"
      key: "F4"
      lanes:
        - name: Now
          columns: 1
          width: 25
          filter: select where type = "epic" and status = "ready" order by priority, points desc
          action: update where id = id() set status="ready"
        - name: Next
          columns: 1
          width: 25
          filter: select where type = "epic" and status = "backlog" and priority = 1 order by priority, points desc
          action: update where id = id() set status="backlog" priority=1
        - name: Later
          columns: 2
          width: 50
          filter: select where type = "epic" and status = "backlog" and priority > 1 order by priority, points desc
          action: update where id = id() set status="backlog" priority=2
      view: expanded
    - name: Docs
      description: "Project notes and documentation files"
      type: doki
      fetcher: file
      url: "index.md"
      key: "F2"

triggers:
  - description: block completion with open dependencies
    ruki: >
      before update
        where new.status = "done" and new.dependsOn any status != "done"
        deny "cannot complete: has open dependencies"
  - description: tasks must pass through review before completion
    ruki: >
      before update
        where new.status = "done" and old.status != "review"
        deny "tasks must go through review before marking done"
  - description: remove deleted task from dependency lists
    ruki: >
      after delete
        update where old.id in dependsOn set dependsOn=dependsOn - [old.id]
  - description: clean up completed tasks after 24 hours
    ruki: >
      every 1day
        delete where status = "done" and updatedAt < now() - 1day
  - description: tasks must have an assignee before starting
    ruki: >
      before update
        where new.status = "inProgress" and new.assignee is empty
        deny "assign someone before moving to in-progress"
  - description: auto-complete epics when all child tasks finish
    ruki: >
      after update
        where new.status = "done" and new.type != "epic"
        update where type = "epic" and new.id in dependsOn and dependsOn all status = "done"
        set status="done"
  - description: cannot delete tasks that are actively being worked
    ruki: >
      before delete
        where old.status = "inProgress"
        deny "cannot delete an in-progress task — move to backlog or done first"
```