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

## Precedence

`tiki` looks for configuration in three locations, from least specific to most specific:

1. **User config directory** (platform-specific, see [above](#configuration-directories)) - your personal defaults
2. **Project config directory** (`.doc/`) - shared with the team via git
3. **Current working directory** (`./`) - local overrides, useful during development

The single highest-priority file wins — no merging across files. This means each project can have its own workflow, statuses, and views that differ from your personal defaults. A design-team project might use statuses like "Draft / Review / Approved" while an engineering project uses "Backlog / In Progress / Done" — each defined in their own `.doc/workflow.yaml`.

### config.yaml

The single highest-priority `config.yaml` found is loaded. Values not specified in that file fall back to built-in defaults (not inherited from lower-priority files).

Search order: user config dir → `.doc/config.yaml` (project) → cwd. Last match wins.

### new.md (task template)

`new.md` follows the same pattern — the single highest-priority file found wins. If a project provides `.doc/new.md`, it completely replaces the user-level template. If no `new.md` is found anywhere, a built-in embedded template is used.

Search order: user config dir → `.doc/new.md` (project) → cwd. Last match wins.

### workflow.yaml

The single highest-priority `workflow.yaml` found is loaded. All workflow-backed sections (statuses, types, views,
global actions, fields, triggers) come from that one file. Lower-priority files are ignored entirely.
See [Workflow format versions](workflow-format.md) for schema evolution.

Search order: user config dir → `.doc/workflow.yaml` (project) → cwd. Last match wins.

- Missing `statuses:` in the winning file is an error.
- Missing `types:` in the winning file is an error.
- Missing `views:` or explicit empty `views: { plugins: [] }` means no views.
- Missing `fields:` means no custom fields.
- Missing `triggers:` means no triggers.

Global actions defined in `views.actions` are appended to each tiki plugin's action list; per-plugin actions with the same key take precedence.

Actions can declare `require:` — a list of context attributes needed for the action to be enabled. Actions with unmet 
requirements are visible but greyed out. See [Action requirements](customization/customization.md#action-requirements) for details.

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
    - key: "A"
      label: "Assign to..."
      action: update where id = id() set assignee=input()
      input: string
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
        - key: "e"
          label: "Link to epic"
          action: update where id = choose(select where type = "epic") set dependsOn = dependsOn + id()
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
      view: expanded              # Board view: "compact" or "expanded"
      actions:
        - key: "l"
          label: "Add tiki to epic"
          action: update where id = id() set dependsOn = dependsOn + choose(select where type != "epic")
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