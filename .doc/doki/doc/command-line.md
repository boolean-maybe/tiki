# Command line options

## Usage

```
tiki [command] [options]
```

Running `tiki` with no arguments launches the TUI in an initialized project.

## Commands

### init

Initialize a tiki project in the current git repository. Creates the `.doc/tiki/` directory structure for task storage.

```bash
tiki init
```

### exec

Execute a [ruki](ruki/index.md) query and exit. Requires an initialized project.

```bash
tiki exec '<ruki-statement>'
```

Examples:
```bash
tiki exec 'select where status = "ready" order by priority'
tiki exec 'update where id = "TIKI-ABC123" set status="done"'
```

### workflow

Manage workflow configuration files.

#### workflow reset

Reset configuration files to their defaults.

```bash
tiki workflow reset [target] [--scope]
```

**Targets** (omit to reset all three files):
- `config` — config.yaml
- `workflow` — workflow.yaml
- `new` — new.md (task template)

**Scopes** (default: `--local`):
- `--global` — user config directory
- `--local` — project config directory (`.doc/`)
- `--current` — current working directory

For `--global`, workflow.yaml and new.md are overwritten with embedded defaults. config.yaml is deleted (built-in defaults take over).

For `--local` and `--current`, files are deleted so the next tier in the [precedence chain](config.md#precedence-and-merging) takes effect.

```bash
# restore all global config to defaults
tiki workflow reset --global

# remove project workflow overrides (falls back to global)
tiki workflow reset workflow --local

# remove cwd config override
tiki workflow reset config --current
```

#### workflow install

Install a named workflow from the tiki repository. Downloads `workflow.yaml` and `new.md` into the scope directory, overwriting any existing files.

```bash
tiki workflow install <name> [--scope]
```

**Scopes** (default: `--local`):
- `--global` — user config directory
- `--local` — project config directory (`.doc/`)
- `--current` — current working directory

```bash
# install the sprint workflow globally
tiki workflow install sprint --global

# install the kanban workflow for the current project
tiki workflow install kanban --local
```

#### workflow describe

Fetch a workflow's description from the tiki repository and print it to stdout.
Reads the top-level `description` field of the named workflow's `workflow.yaml`.
Prints nothing and exits 0 if the workflow has no description field.

```bash
tiki workflow describe <name>
```

**Examples:**

```bash
# preview the todo workflow before installing it
tiki workflow describe todo

# check what bug-tracker is for
tiki workflow describe bug-tracker
```

### demo

Clone the demo project and launch the TUI. If the `tiki-demo` directory already exists it is reused.

```bash
tiki demo
```

### sysinfo

Display system and terminal environment information useful for troubleshooting.

```bash
tiki sysinfo
```

## Markdown viewer

`tiki` doubles as a standalone markdown and image viewer. Pass a file path or URL as the first argument.

```bash
tiki file.md
tiki https://github.com/user/repo/blob/main/README.md
tiki image.png
echo "# Hello" | tiki -
```

See [Markdown viewer](markdown-viewer.md) for navigation and keybindings.

## Piped input

When stdin is piped and no positional arguments are given, tiki creates a task from the input. The first line becomes the title; the rest becomes the description.

```bash
echo "Fix the login bug" | tiki
tiki < bug-report.md
```

See [Quick capture](quick-capture.md) for more examples.

## Flags

| Flag | Description |
|---|---|
| `--help`, `-h` | Show usage information |
| `--version`, `-v` | Show version, commit, and build date |
| `--log-level <level>` | Set log level: `debug`, `info`, `warn`, `error` |
