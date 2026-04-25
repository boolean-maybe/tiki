# Customization Examples

- [Assign to me](#assign-to-me--plugin-action)
- [Add tag to task](#add-tag-to-task--plugin-action)
- [Custom status + reject action](#custom-status--reject-action)
- [Implement with Claude Code](#implement-with-claude-code--pipe-action)
- [Search all tikis](#search-all-tikis--single-lane-plugin)
- [Quick assign](#quick-assign--lane-based-assignment)
- [Stale task detection](#stale-task-detection--time-trigger--plugin)
- [My tasks](#my-tasks--user-scoped-plugin)
- [Recent ideas](#recent-ideas--good-or-trash)
- [Auto-delete stale tasks](#auto-delete-stale-tasks--time-trigger)
- [Priority triage](#priority-triage--five-lane-plugin)
- [Sprint board](#sprint-board--custom-enum-lanes)
- [Severity triage](#severity-triage--custom-enum-filter--action)
- [Subtasks in epic](#subtasks-in-epic--custom-taskidlist--quantifier-trigger)
- [By topic](#by-topic--tag-based-lanes)
- [Bulk cleanup](#bulk-cleanup--delete-without-selection)
- [AI-gated action](#ai-gated-action--require)
- [Link to / add tiki to epic](#link-to--add-tiki-to-epic--choose-action)

## Assign to me — global plugin action

Shortcut key that sets the selected task's assignee to the current git user. Defined under `views.actions`, this shortcut is available in all tiki plugin views.

```yaml
views:
  actions:
    - key: "a"
      label: "Assign to me"
      action: update where id = id() set assignee=user()
```

The same format works as a per-plugin action (under a plugin's `actions:` key) if you only want it in a specific view.

## Add tag to task — plugin action

Appends a tag to the selected task's tag list without removing existing tags.

```yaml
actions:
  - key: "t"
    label: "Tag my_project"
    action: update where id = id() set tags=tags + ["my_project"]
```

## Custom status + reject action

Define a custom "rejected" status, then add a plugin action on the Backlog view to reject tasks.

```yaml
statuses:
  - key: rejected
    label: Rejected
    emoji: "🚫"
    done: true
```

```yaml
- name: Backlog
  key: "F3"
  lanes:
    - name: Backlog
      filter: select where status = "backlog" order by priority
  actions:
    - key: "r"
      label: "Reject"
      action: update where id = id() set status="rejected"
```

## Implement with Claude Code — pipe action

Shortcut key that pipes the selected task's title and description to Claude Code for implementation.

```yaml
actions:
  - key: "i"
    label: "Implement"
    action: >
      select title, description where id = id()
      | run("claude -p 'Implement this: $1. Details: $2'")
```

## Search all tikis — single-lane plugin

A plugin with one unfiltered lane shows every task. Press `/` to search across all of them.

```yaml
- name: All
  key: "F5"
  lanes:
    - name: All
      columns: 4
      filter: select order by updatedAt desc
```

## Quick assign — lane-based assignment

Three lanes split tasks by assignee. Moving a task into Alice's or Bob's lane auto-assigns it.

```yaml
- name: Team
  key: "F6"
  lanes:
    - name: Unassigned
      filter: select where assignee is empty order by priority
    - name: Alice
      filter: select where assignee = "alice" order by priority
      action: update where id = id() set assignee="alice"
    - name: Bob
      filter: select where assignee = "bob" order by priority
      action: update where id = id() set assignee="bob"
```

## Stale task detection — time trigger + plugin

A daily trigger tags in-progress tasks that haven't been updated in a week. A dedicated plugin shows all flagged tasks.

```yaml
triggers:
  - description: flag stale in-progress tasks
    ruki: >
      every 1day
        update where status = "inProgress" and now() - updatedAt > 7day
               and "attention" not in tags
          set tags=tags + ["attention"]
```

```yaml
- name: Attention
  key: "F7"
  lanes:
    - name: Needs Attention
      columns: 4
      filter: select where "attention" in tags order by updatedAt
```

## My tasks — user-scoped plugin

Shows only tasks assigned to the current git user.

```yaml
- name: My Tasks
  key: "F8"
  lanes:
    - name: My Tasks
      columns: 4
      filter: select where assignee = user() order by priority
```

## Recent ideas — good or trash?

Two-lane plugin to review recent ideas and trash the ones you don't need. Moving to Trash swaps the "idea" tag for "trash".

```yaml
- name: Recent Ideas
  description: "Review recent"
  key: "F9"
  lanes:
    - name: Recent Ideas
      columns: 3
      filter: select where "idea" in tags and now() - createdAt < 7day order by createdAt desc
    - name: Trash
      columns: 1
      filter: select where "trash" in tags order by updatedAt desc
      action: update where id = id() set tags=tags - ["idea"] + ["trash"]
```

## Auto-delete stale tasks — time trigger

Deletes backlog tasks that were created over 3 months ago and haven't been updated in 2 months.

```yaml
triggers:
  - description: auto-delete stale backlog tasks
    ruki: >
      every 1day
        delete where status = "backlog"
                     and now() - createdAt > 3month
                     and now() - updatedAt > 2month
```

## Priority triage — five-lane plugin

One lane per priority level. Moving a task between lanes reassigns its priority.

```yaml
- name: Priorities
  key: "F10"
  lanes:
    - name: Critical
      filter: select where priority = 1 order by updatedAt desc
      action: update where id = id() set priority=1
    - name: High
      filter: select where priority = 2 order by updatedAt desc
      action: update where id = id() set priority=2
    - name: Medium
      filter: select where priority = 3 order by updatedAt desc
      action: update where id = id() set priority=3
    - name: Low
      filter: select where priority = 4 order by updatedAt desc
      action: update where id = id() set priority=4
    - name: Minimal
      filter: select where priority = 5 order by updatedAt desc
      action: update where id = id() set priority=5
```

## Sprint board — custom enum lanes

Uses a custom `sprint` enum field. Lanes per sprint; moving a task between lanes reassigns it. The third lane catches unplanned backlog tasks.

Requires:

```yaml
fields:
  - name: sprint
    type: enum
    values: [sprint-7, sprint-8, sprint-9]
```

```yaml
- name: Sprint Board
  key: "F9"
  lanes:
    - name: Current Sprint
      filter: select where sprint = "sprint-7" and status != "done" order by priority
      action: update where id = id() set sprint="sprint-7"
    - name: Next Sprint
      filter: select where sprint = "sprint-8" order by priority
      action: update where id = id() set sprint="sprint-8"
    - name: Unplanned
      filter: select where sprint is empty and status = "backlog" order by priority
      action: update where id = id() set sprint=empty
```

## Severity triage — custom enum filter + action

Lanes per severity level. The last lane combines two values with `or`. A per-plugin action lets you mark a task as trivial without moving it.

Requires:

```yaml
fields:
  - name: severity
    type: enum
    values: [critical, major, minor, trivial]
```

```yaml
- name: Severity
  key: "F10"
  lanes:
    - name: Critical
      filter: select where severity = "critical" order by updatedAt desc
      action: update where id = id() set severity="critical"
    - name: Major
      filter: select where severity = "major" order by updatedAt desc
      action: update where id = id() set severity="major"
    - name: Minor & Trivial
      columns: 2
      filter: >
        select where severity = "minor" or severity = "trivial"
        order by severity, priority
      action: update where id = id() set severity="minor"
  actions:
    - key: "t"
      label: "Trivial"
      action: update where id = id() set severity="trivial"
```

## Subtasks in epic — custom taskIdList + quantifier trigger

A `subtasks` field on parent tasks tracks their children (inverse of `dependsOn`). A trigger auto-completes the parent when every subtask is done. The plugin shows open vs. completed parents.

Requires:

```yaml
fields:
  - name: subtasks
    type: taskIdList
```

```yaml
triggers:
  - description: close parent when all subtasks are done
    ruki: >
      every 5min
        update where subtasks is not empty
                     and status != "done"
                     and all subtasks where status = "done"
          set status="done"
```

```yaml
- name: Epics
  key: "F11"
  lanes:
    - name: In Progress
      filter: >
        select where subtasks is not empty
                     and status != "done"
        order by priority
    - name: Completed
      columns: 1
      filter: >
        select where subtasks is not empty
                     and status = "done"
        order by updatedAt desc
```

## By topic — tag-based lanes

Split tasks into lanes by tag. Useful for viewing work across domains at a glance.

```yaml
- name: By Topic
  key: "F11"
  lanes:
    - name: Frontend
      columns: 2
      filter: select where "frontend" in tags order by priority
    - name: Backend
      columns: 2
      filter: select where "backend" in tags order by priority
```

## Bulk cleanup — delete without selection

A bulk action that deletes all done tasks without needing a selection. Because this action does not use `id()`, it has no `id` requirement and stays enabled even when nothing is selected.

```yaml
actions:
  - key: "X"
    label: "Delete all done"
    action: delete where status = "done"
```

## AI-gated action — require

An action that only enables when an AI agent is configured and a task is selected. The `ai` requirement is checked against `config.yaml`'s `ai.agent` setting. The `id` requirement is auto-inferred from `id()` usage — listing it explicitly is allowed but redundant.

```yaml
actions:
  - key: "c"
    label: "Chat about task"
    action: select title, description where id = id() | run("claude -p 'Discuss: $1. Details: $2'")
    require: ["ai"]
```

## Link to / add tiki to epic — choose() action

From the Backlog view opens a Quick Select picker showing only epics that the current tiki is not already linked to, and adds it to the chosen epic's list.
From the Roadmap view, pick any non-epic tiki that is not already linked to the selected epic, and add it.

```yaml
- name: Backlog
  actions:
    - key: "e"
      label: "Link to epic"
      action: update where id = choose(select where type = "epic" and id() not in dependsOn) set dependsOn = dependsOn + id()
- name: Roadmap
  actions:
    - key: "l"
      label: "Add tiki to epic"
      action: update where id = id() set dependsOn = dependsOn + choose(select where type != "epic" and id not in target.dependsOn)
```

A tiki is linked to an epic by adding its id to the epic's `dependsOn` list. Each filter hides
epics/tikis where the link already exists:

- Backlog: `id() not in dependsOn` — the candidate epic's own `dependsOn` doesn't already
  contain this tiki.
- Roadmap: `id not in target.dependsOn` — the candidate tiki's id is not already in the
  selected epic's `dependsOn`.
