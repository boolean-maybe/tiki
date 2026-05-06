# Custom Statuses and Types

Statuses and types are user-configurable enum fields in `workflow.yaml`.
Both follow the same structural rules with a few differences noted below.

## Configuration

### YAML Shape

```yaml
fields:
  - name: status
    type: enum
    values:
      - value: backlog
        label: Backlog
        emoji: "📥"
        default: true
      - value: inProgress
        label: "In Progress"
        emoji: "⚙️"
        active: true
      - value: done
        label: Done
        emoji: "✅"
        done: true
  - name: type
    type: enum
    values:
      - value: story
        label: Story
        emoji: "🌀"
      - value: bug
        label: Bug
        emoji: "💥"
```

### Shared Rules

These rules apply identically to both built-in enum fields:

| Rule | Detail |
|---|---|
| Canonical values | Values must already be in canonical form. Non-canonical values are rejected with a suggestion. |
| Label defaults to value | When `label` is omitted, the value is used as the label. |
| Empty labels rejected | Explicitly empty or whitespace-only labels are invalid. |
| Emoji trimmed | Leading/trailing whitespace is stripped from emoji values. |
| Unique display strings | Each entry must produce a unique `"Label Emoji"` display. Duplicates are rejected. |
| At least one entry | An empty list is invalid. |
| Duplicate values rejected | Two entries with the same canonical value are invalid. |
| Unknown keys rejected | Only documented metadata keys are allowed in each entry. |

### Status-Only Keys

Statuses support additional boolean flags that types do not:

| Key | Required | Description |
|---|---|---|
| `active` | no | Marks a status as active (in-progress work). |
| `default` | at most one | The status assigned to newly created tikis. Optional for notes-only projects. |
| `done` | exactly one | The terminal status representing completion. |

Valid keys in a status value entry: `value`, `label`, `emoji`, `active`, `default`, `done`.

### Type-Only Behavior

Types support one optional boolean flag:

| Key | Required | Description |
|---|---|---|
| `default` | no | The type assigned to newly created tikis. At most one allowed; first type is fallback. |

Valid keys in a type value entry: `value`, `label`, `emoji`, `default`.

### Key Normalization

Status and type values use different normalization rules:

- **Status values** use camelCase. Splits on `_`, `-`, ` `, and camelCase boundaries, then reassembles as
  camelCase.
  Examples: `"in_progress"` -> `"inProgress"`, `"In Progress"` -> `"inProgress"`.

- **Type values** are lowercased with all separators stripped.
  Examples: `"My-Type"` -> `"mytype"`, `"some_thing"` -> `"something"`.

Values in `workflow.yaml` must already be in their canonical form. Input normalization (from user queries,
ruki expressions, etc.) still applies at lookup time.

### Required Sections

All workflow-backed sections come from the single highest-priority `workflow.yaml`.
See [Configuration: Precedence](../config.md#precedence).

- Missing `fields:` entry `name: status` in the winning file is an error.
- Missing `fields:` entry `name: type` in the winning file is an error.

There are no built-in fallbacks for either section.

## Failure Behavior

### Invalid Configuration

| Scenario | Behavior |
|---|---|
| Empty list | Error |
| Non-canonical value | Error with suggested canonical form |
| Empty/whitespace label | Error |
| Duplicate display string | Error |
| Unknown metadata key in entry | Error |
| Missing `default: true` (statuses) | OK — notes-only project; new captures save with only `id` and `title` |
| Missing `done: true` (statuses) | Error |
| Multiple `default: true` (statuses) | Error |
| Multiple `done: true` (statuses) | Error |
| Multiple `default: true` (types) | Error |

### Invalid Saved Tasks

- A tiki with a missing or unknown `type` fails to load and is skipped.
- On single-task reload (`ReloadTask`), an invalid file causes the task to be removed from memory.

### Cross-Reference Errors

If the active workflow file defines types that don't match the views, actions, or triggers in the same file,
startup fails with a configuration error. There is no silent view-skipping or automatic remapping.

## Pre-Init Rules

Calling type or status helpers (`task.ParseType()`, `task.AllTypes()`, `task.DefaultType()`,
`task.ParseStatus()`, etc.) before `config.LoadWorkflowRegistries()` is a programmer error and panics.
