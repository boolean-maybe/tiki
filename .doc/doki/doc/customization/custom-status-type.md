# Custom Statuses and Types

`status` and `type` are ordinary enum fields declared in `workflow.yaml fields:`. They have no
special semantics in the runtime — only the meaning you encode via the enum values you declare.
This page covers the validation rules that apply to any enum field, with `status` and `type` as
examples.

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
      - value: done
        label: Done
        emoji: "✅"
  - name: type
    type: enum
    values:
      - value: story
        label: Story
        emoji: "🌀"
        default: true
      - value: bug
        label: Bug
        emoji: "💥"
```

### Shared Rules

These rules apply to every enum field:

| Rule | Detail |
|---|---|
| Label defaults to value | When `label` is omitted, the value is used as the label. |
| Emoji trimmed | Leading/trailing whitespace is stripped from emoji values. |
| Unique display strings | Each entry must produce a unique `"Label Emoji"` display. Duplicates are rejected. |
| At least one entry | An empty list is invalid. |
| Duplicate values rejected | Two entries with the same value are invalid. |
| Unknown keys rejected | Only documented metadata keys are allowed in each entry. |
| At most one `default: true` | The default value is the creation default for the field. |

Valid keys in an enum value entry: `value`, `label`, `emoji`, `default`.

The legacy `active:` and `done:` flags on enum values are no longer accepted — they were
status-specific concepts that the runtime no longer recognizes. If you want a visual cue for
"in-progress" or "terminal" states, use the `emoji:` field on each value (e.g. ✅ on the value
that represents completion).

### Required Sections

`status` and `type` are not built-in. If a workflow declares them in `fields:`, they behave like
any other enum field. If your workflow doesn't declare them, no `status` or `type` semantics
exist for that workflow — frontmatter values for those keys round-trip as unknown.

All workflow-backed sections come from the single highest-priority `workflow.yaml`.
See [Configuration: Precedence](../config.md#precedence).

## Failure Behavior

### Invalid Configuration

| Scenario | Behavior |
|---|---|
| Empty list | Error |
| Empty/whitespace label | Error |
| Duplicate display string | Error |
| Unknown metadata key in entry (e.g. `active:` / `done:`) | Error with migration message |
| Multiple `default: true` | Error |

### Invalid Saved Tasks

- An enum value not declared in `workflow.yaml` is demoted to "stale" on load and round-trips
  verbatim, so the user can see the bad value and run `tiki repair` rather than silently lose it.
- Save-time validation (the mutation gate) rejects writes whose enum values don't match the
  declared set.

### Cross-Reference Errors

If the active workflow file defines values that don't match the views, actions, or triggers in the
same file, startup fails with a configuration error. There is no silent view-skipping or automatic
remapping.

## Pre-Init Rules

Calling enum helpers (`task.ParseType()`, `task.AllTypes()`, `task.DefaultType()`,
`task.ParseStatus()`, etc.) before `config.LoadWorkflowFields()` returns empty/zero values without
panicking. Helpers fall back to "no enum field configured" semantics when the catalog is empty.
