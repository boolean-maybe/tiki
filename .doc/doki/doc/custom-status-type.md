# Custom Statuses and Types

Statuses and types are user-configurable via `workflow.yaml`.
Both follow the same structural rules with a few differences noted below.

## Configuration

### YAML Shape

```yaml
statuses:
  - key: backlog
    label: Backlog
    emoji: "📥"
    default: true
  - key: inProgress
    label: "In Progress"
    emoji: "⚙️"
    active: true
  - key: done
    label: Done
    emoji: "✅"
    done: true

types:
  - key: story
    label: Story
    emoji: "🌀"
  - key: bug
    label: Bug
    emoji: "💥"
```

### Shared Rules

These rules apply identically to both `statuses:` and `types:`:

| Rule | Detail |
|---|---|
| Canonical keys | Keys must already be in canonical form. Non-canonical keys are rejected with a suggested canonical form. |
| Label defaults to key | When `label` is omitted, the key is used as the label. |
| Empty labels rejected | Explicitly empty or whitespace-only labels are invalid. |
| Emoji trimmed | Leading/trailing whitespace is stripped from emoji values. |
| Unique display strings | Each entry must produce a unique `"Label Emoji"` display. Duplicates are rejected. |
| At least one entry | An empty list is invalid. |
| Duplicate keys rejected | Two entries with the same canonical key are invalid. |
| Unknown keys rejected | Only documented keys are allowed in each entry. |

### Status-Only Keys

Statuses support additional boolean flags that types do not:

| Key | Required | Description |
|---|---|---|
| `active` | no | Marks a status as active (in-progress work). |
| `default` | exactly one | The status assigned to newly created tasks. |
| `done` | exactly one | The terminal status representing completion. |

Valid keys in a status entry: `key`, `label`, `emoji`, `active`, `default`, `done`.

### Type-Only Behavior

Types have no boolean flags. The first configured type is used as the creation default.

Valid keys in a type entry: `key`, `label`, `emoji`.

### Key Normalization

Status and type keys use different normalization rules:

- **Status keys** use camelCase. Splits on `_`, `-`, ` `, and camelCase boundaries, then reassembles as camelCase.
  Examples: `"in_progress"` -> `"inProgress"`, `"In Progress"` -> `"inProgress"`.

- **Type keys** are lowercased with all separators stripped.
  Examples: `"My-Type"` -> `"mytype"`, `"some_thing"` -> `"something"`.

Keys in `workflow.yaml` must already be in their canonical form. Input normalization (from user queries, ruki expressions, etc.) still applies at lookup time.

### Required Sections

All workflow-backed sections come from the single highest-priority `workflow.yaml` (see [Configuration: Precedence](config.md#precedence)).

- Missing `statuses:` in the winning file is an error.
- Missing `types:` in the winning file is an error.

There are no built-in fallbacks for either section.

## Failure Behavior

### Invalid Configuration

| Scenario | Behavior |
|---|---|
| Empty list | Error |
| Non-canonical key | Error with suggested canonical form |
| Empty/whitespace label | Error |
| Duplicate display string | Error |
| Unknown key in entry | Error |
| Missing `default: true` (statuses) | Error |
| Missing `done: true` (statuses) | Error |
| Multiple `default: true` (statuses) | Error |
| Multiple `done: true` (statuses) | Error |

### Invalid Saved Tasks

- A tiki with a missing or unknown `type` fails to load and is skipped.
- On single-task reload (`ReloadTask`), an invalid file causes the task to be removed from memory.

### Invalid Templates

- Missing `type` in a template defaults to the first configured type.
- Invalid non-empty `type` in a template is a hard error; creation is aborted.

### Cross-Reference Errors

If the active workflow file defines types that don't match the views, actions, or triggers in the same file, startup fails with a configuration error. There is no silent view-skipping or automatic remapping.

## Pre-Init Rules

Calling type or status helpers (`task.ParseType()`, `task.AllTypes()`, `task.DefaultType()`, `task.ParseStatus()`, etc.) before `config.LoadWorkflowRegistries()` is a programmer error and panics.
