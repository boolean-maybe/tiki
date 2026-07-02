# Command line options

## Usage

```
tiki [command] [options]
```

Running `tiki` with no arguments launches the TUI over the current directory. There is no project setup
step — `tiki` recursively reads every `.md` file under the cwd, skipping `.git/`, dotted directories, and
paths matched by `.gitignore`/`.tikiignore`. See [Installation](install.md#scan-scope) for the full scan
model.

## Commands

### exec

Execute a [ruki](ruki/index.md) query and exit.

```bash
tiki exec [--format table|json] [--] '<ruki-statement>'
```

| Option | Description |
|---|---|
| `--format <table\|json>` | Output format. `table` (default) prints human-readable text; `json` emits compact JSON |
| `--` | End-of-options marker. Use when the statement starts with `-` (e.g. a `--` ruki line comment) |

Examples:
```bash
tiki exec 'select where status = "ready" order by priority'
tiki exec 'update where id = "ABC123" set status="done"'

# JSON output for scripting
tiki exec --format json 'select id, title where status = "ready"'
tiki exec --format=json 'count(select where assignee = user())'

# statement that starts with a `--` ruki line comment
tiki exec -- '-- backlog count
count(select where status != "done")'
```

### workflow

Manage workflow configuration files.

#### workflow reset

Reset configuration files to their defaults.

```bash
tiki workflow reset [target] [--scope]
```

**Targets** (omit to reset all files):
- `config` — config.yaml
- `workflow` — workflow.yaml

**Scopes** (default: `--current`):
- `--global` — user config directory
- `--current` — current working directory (the scan root)
- `--local` — deprecated alias for `--current` (the project tier and the cwd tier are the same directory now)

For `--global`, workflow.yaml is overwritten with the default. config.yaml is deleted (built-in defaults take over).

For `--current`, files are deleted so the next tier in the [precedence chain](config.md#precedence) takes effect.

```bash
# restore all global config to defaults
tiki workflow reset --global

# remove the cwd workflow override (falls back to global)
tiki workflow reset workflow --current

# remove the cwd config override
tiki workflow reset config --current
```

#### workflow install

Install a workflow from a name, local file, or URL. Writes `workflow.yaml` into the
scope directory, overwriting any existing file.

```bash
tiki workflow install <source> [--scope]
```

**Sources:** bundled name (`kanban`, `todo`, `bug-tracker`), file path (`./custom.yaml`),
or URL (`https://example.com/workflow.yaml`).

**Scopes** (default: `--current`):
- `--global` — user config directory
- `--current` — current working directory (the scan root)
- `--local` — deprecated alias for `--current`

```bash
# install the kanban workflow globally
tiki workflow install kanban --global

# install from a local file into the current directory
tiki workflow install ./custom.yaml --current

# install from a URL
tiki workflow install https://example.com/workflow.yaml --global
```

#### workflow describe

Print a workflow's description. Reads the top-level `description` field from the workflow YAML.
Prints nothing and exits 0 if the workflow has no description field.

```bash
tiki workflow describe <source>
```

**Sources:** embedded name, file path, or URL (same as `workflow install`).

**Examples:**

```bash
# preview the todo workflow before installing it
tiki workflow describe todo

# describe a local workflow file
tiki workflow describe ./custom.yaml

# describe a remote workflow
tiki workflow describe https://example.com/workflow.yaml
```

### demo

Launch the demo project. The demo files are extracted into a `tiki-demo/` directory in the current working directory.

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

When stdin is piped and no positional arguments are given, tiki creates a tiki from the input. The
first line becomes the title; the rest becomes the description.

```bash
echo "Fix the login bug" | tiki
tiki < bug-report.md
```

What fields the new tiki carries is decided by the active workflow:

- If the workflow declares a status with `default: true` (as kanban, todo, and bug-tracker do),
  captured input gets the default status, type, priority, and points filled in. The result appears
  on board/list views whose lane filters match.
- If the workflow has no `default: true` status, captured input is saved with only `id` and `title`
  in the frontmatter. It is reachable by id and file path; views that filter on workflow-declared fields
  via `has(...)` will skip it.

See [Quick capture](quick-capture.md) for more examples.

## Flags

| Flag | Description |
|---|---|
| `--help`, `-h` | Show usage information |
| `--version`, `-v` | Show version, commit, and build date |
| `--log-level <level>` | Set log level: `debug`, `info`, `warn`, `error` |
