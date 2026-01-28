# Configuration

## Configuration files

- `config-dir/config.yaml` main configuration file
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
# Appearance settings
appearance:
  theme: auto               # Theme: "auto" (detect from terminal), "dark", "light"
  gradientThreshold: 256    # Minimum terminal colors for gradient rendering
                            # Options: 16, 256, 16777216 (truecolor)
                            # Gradients disabled if terminal has fewer colors
                            # Default: 256 (works well on most terminals)

# Logging settings
logging:
  level: error              # Log level: "debug", "info", "warn", "error"

# Board view settings
board:
  view: expanded            # Default board view: "compact", "expanded"

# Header settings
header:
  visible: true             # Show/hide header: true, false

# Tiki settings
tiki:
  maxPoints: 10             # Maximum story points for tasks
```