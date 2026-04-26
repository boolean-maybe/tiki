# Command line options

## Usage

```
tiki [command] [options]
```

Running `tiki` with no arguments launches the TUI in an initialized project.

## Commands

### init

Initialize a tiki project. Creates the `.doc/tiki/` and `.doc/doki/` directory structures.

If the target directory does not exist, it is created. If the directory is not a git repository, `git init` is run
automatically, unless `store.git` is set to `false` (see [Configuration](config.md)).

```
tiki init [directory] [-w|--workflow <source>] [--ai-skill <list>] [--samples] [-n|--non-interactive]
```

| Option | Description |
|---|---|
| `directory` | Target directory (default: current directory) |
| `-w`, `--workflow <source>` | Install a workflow (embedded name, file path, or URL) |
| `--ai-skill <list>` | AI skills to install, comma-separated (e.g. `claude,gemini`) |
| `--samples` | Create bundled sample tasks (non-interactive mode only) |
| `-n`, `--non-interactive` | Skip prompts, use only flags and defaults |

**Sample tasks** are created if:
- Interactive mode with default workflow: samples are created automatically
- Non-interactive mode: samples are created only with `--samples`

```bash
# interactive init with AI skill selection
tiki init

# initialize a subdirectory (creates dir, and git repo if store.git is enabled)
tiki init my-project

# install a bundled workflow by name
tiki init -w todo

# initialize a subdirectory with a bundled workflow
tiki init -w kanban my-project

# install from a local file
tiki init -w ./custom-workflow.yaml

# install from a URL
tiki init -w https://example.com/workflow.yaml

# fully non-interactive
tiki init -n --ai-skill claude,gemini --samples
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

**Targets** (omit to reset all files):
- `config` — config.yaml
- `workflow` — workflow.yaml

**Scopes** (default: `--local`):
- `--global` — user config directory
- `--local` — project config directory (`.doc/`)
- `--current` — current working directory

For `--global`, workflow.yaml is overwritten with the default. config.yaml is deleted (built-in defaults take over).

For `--local` and `--current`, files are deleted so the next tier in the [precedence chain](config.md#precedence) takes effect.

```bash
# restore all global config to defaults
tiki workflow reset --global

# remove project workflow overrides (falls back to global)
tiki workflow reset workflow --local

# remove cwd config override
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

**Scopes** (default: `--local`):
- `--global` — user config directory
- `--local` — project config directory (`.doc/`)
- `--current` — current working directory

```bash
# install the kanban workflow globally
tiki workflow install kanban --global

# install from a local file
tiki workflow install ./custom.yaml --local

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
