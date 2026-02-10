# Customization

tiki TUI app is much like a lego - everything is a customizable view. Here is, for example,
how Backlog is defined:

```text
        name: Backlog
        foreground: "#5fff87"
        background: "#005f00"
        key: "F3"
        lanes:
          - name: Backlog
            columns: 4
            filter: status = 'backlog'
        actions:
          - key: "b"
            label: "Add to board"
            action: status = 'ready'
        sort: Priority, ID
```
that translates to - show all tikis in the status `backlog`, sort by priority and then by ID arranged visually in 4 columns in a single lane. 
The `actions` section defines a keyboard shortcut `b` that moves the selected tiki to the board by setting its status to `ready`
You define the name, caption colors, hotkey, tiki filter and sorting. Save this into a yaml file and add this line:

```text
        plugins:
            - file: my-plugin.yaml
```

to the `config.yaml` file in the config directory

Likewise the documentation is just a plugin:

```text
        name: Documentation
        type: doki
        fetcher: file
        url: "index.md"
        foreground: "#ff9966"
        background: "#993300"
        key: "F1"
```

that translates to - show `index.md` file located under `.doc/doki`
installed in the same way

## Multi-lane plugin

Backlog is a pretty simple plugin in that it displays all tikis in a single lane. Multi-lane tiki plugins offer functionality
similar to that of the board. You can define multiple lanes per view and move tikis around with Shift-Left/Shift-Right
much like in the board. You can create a multi-lane plugin by defining multiple lanes in its definition and assigning
actions to each lane. An action defines what happens when you move a tiki into the lane. Here is a multi-lane plugin
definition that roughly mimics the board:

```yaml
name: Custom
foreground: "#5fff87"
background: "#005f00"
key: "F4"
sort: Priority, Title
lanes:
  - name: Ready
    columns: 1
    filter: status = 'ready'
    action: status = 'ready'
  - name: In Progress
    columns: 1
    filter: status = 'in_progress'
    action: status = 'in_progress'
  - name: Review
    columns: 1
    filter: status = 'review'
    action: status = 'review'
  - name: Done
    columns: 1
    filter: status = 'done'
    action: status = 'done'
```

## Plugin actions

In addition to lane actions that trigger when moving tikis between lanes, you can define plugin-level actions 
that apply to the currently selected tiki via a keyboard shortcut. These shortcuts are displayed in the header when the plugin is active.

```yaml
actions:
  - key: "b"
    label: "Add to board"
    action: status = 'ready'
  - key: "a"
    label: "Assign to me"
    action: assignee = CURRENT_USER
```

Each action has:
- `key` - a single printable character used as the keyboard shortcut
- `label` - description shown in the header
- `action` - an action expression (same syntax as lane actions, see below)

When the shortcut key is pressed, the action is applied to the currently selected tiki. 
For example, pressing `b` in the Backlog plugin changes the selected tiki's status to `ready`, effectively moving it to the board.

## Action expression

The `action: status = 'backlog'` statement in a plugin is an action to be run when a tiki is moved into the lane. Here `=`
means `assign` so status is assigned `backlog` when the tiki is moved. Likewise you can manipulate tags using `+-` (add)
or `-=` (remove) expressions. For example, `tags += [idea, UI]` adds `idea` and `UI` tags to a tiki

### Supported Fields

- `status` - set workflow status (case-insensitive)
- `type` - set task type: `story`, `bug`, `spike`, `epic` (case-insensitive)
- `priority` - set numeric priority (1-5)
- `points` - set numeric points (0 or positive, up to max points)
- `assignee` - set assignee string
- `tags` - add/remove tags (list)

### Operators

- `=` assigns a value to `status`, `type`, `priority`, `points`, `assignee`
- `+=` adds tags, `-=` removes tags
- multiple operations are separated by commas: `status=done, tags+=[moved]`

### Literals

- strings can be quoted (`'in_progress'`, `"alex"`) or bare (`done`, `alex`)
- use quotes when the value has spaces
- integers are used for `priority` and `points`
- tag lists use brackets: `tags += [ui, frontend]`
- `CURRENT_USER` assigns the current git user to `assignee`
- example: `assignee = CURRENT_USER`

## Filter expression

The `filter: status = 'backlog'` statement in a plugin is a filter expression that determines which tikis appear in the view.

### Supported Fields

You can filter on these task fields:
- `id` - Task identifier (e.g., 'TIKI-m7n2xk')
- `title` - Task title text (case-insensitive)
- `type` - Task type: 'story', 'bug', 'spike', or 'epic' (case-insensitive)
- `status` - Workflow status (case-insensitive)
- `assignee` - Assigned user (case-insensitive)
- `priority` - Numeric priority value
- `points` - Story points estimate
- `tags` (or `tag`) - List of tags (case-insensitive)
- `createdAt` - Creation timestamp
- `updatedAt` - Last update timestamp

All string comparisons are case-insensitive.

### Operators

- **Comparison**: `=` (or `==`), `!=`, `>`, `>=`, `<`, `<=`
- **Logical**: `AND`, `OR`, `NOT` (precedence: NOT > AND > OR)
- **Membership**: `IN`, `NOT IN` (check if value in list using `[val1, val2]`)
- **Grouping**: Use parentheses `()` to control evaluation order

### Literals and Special Values

**Special expressions**:
- `CURRENT_USER` - Resolves to the current git user (works in comparisons and IN lists)
- `NOW` - Current timestamp

**Time expressions**:
- `NOW - UpdatedAt` - Time elapsed since update
- `NOW - CreatedAt` - Time since creation
- Duration units: `min`/`minutes`, `hour`/`hours`, `day`/`days`, `week`/`weeks`, `month`/`months`
- Examples: `2hours`, `14days`, `3weeks`, `60min`, `1month`
- Operators: `+` (add), `-` (subtract or compute duration)

**Special tag semantics**:
- `tags IN ['ui', 'frontend']` matches if ANY task tag matches ANY list value
- This allows intersection testing across tag arrays

### Examples

```text
# Multiple statuses
status = 'ready' OR status = 'in_progress'

# With tags
tags IN ['frontend', 'urgent']

# High priority bugs
type = 'bug' AND priority = 0

# Features and ideas assigned to me
(type = 'feature' OR tags IN ['idea']) AND assignee = CURRENT_USER

# Unassigned large tasks
assignee = '' AND points >= 5

# Recently created tasks not in backlog
(NOW - CreatedAt < 2hours) AND status != 'backlog'
```

## Sorting

The `sort` field determines the order in which tikis appear in the view. You can sort by one or more fields, and control the direction (ascending or descending).

### Sort Syntax

```text
sort: Field1, Field2 DESC, Field3
```

### Examples

```text
# Sort by creation time descending (recent first), then priority, then title
sort: CreatedAt DESC, Priority, Title
```