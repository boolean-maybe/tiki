# Trigger Ideas

Candidate triggers for the default workflow. Each represents a common workflow pattern.

- [WIP limit per assignee](#wip-limit-per-assignee)
- [Require description for high-priority tasks](#require-description-for-high-priority-tasks)
- [Auto-create next occurrence for recurring tasks](#auto-create-next-occurrence-for-recurring-tasks)
- [Return stale in-progress tasks to backlog](#return-stale-in-progress-tasks-to-backlog)
- [Unblock tasks when dependencies complete](#unblock-tasks-when-dependencies-complete)
- [Prevent re-opening completed tasks directly](#prevent-re-opening-completed-tasks-directly)
- [Auto-assign creator on start](#auto-assign-creator-on-start)
- [Escalate overdue tasks](#escalate-overdue-tasks)
- [Require points estimate before review](#require-points-estimate-before-review)
- [Auto-tag bugs as urgent when high priority](#auto-tag-bugs-as-urgent-when-high-priority)
- [Prevent epics from being assigned](#prevent-epics-from-being-assigned)
- [Auto-remove urgent tag when priority drops](#auto-remove-urgent-tag-when-priority-drops)
- [Notify on critical task creation via webhook](#notify-on-critical-task-creation-via-webhook)
- [Spike time-box](#spike-time-box)
- [Require task breakdown for large estimates](#require-task-breakdown-for-large-estimates)
- [Propagate priority from epic to children](#propagate-priority-from-epic-to-children)
- [Auto-set due date for in-progress tasks](#auto-set-due-date-for-in-progress-tasks)
- [Require a title on creation](#require-a-title-on-creation)
- [Prevent creating tasks as done](#prevent-creating-tasks-as-done)
- [Epics require a description at creation](#epics-require-a-description-at-creation)
- [Prevent deleting epics with active children](#prevent-deleting-epics-with-active-children)
- [Block deletion of high-priority tasks](#block-deletion-of-high-priority-tasks)

## WIP limit per assignee

Prevent overloading a single person with too many active tasks.

```yaml
- description: limit work in progress to 3 tasks per assignee
  ruki: >
    before update
      where new.status = "inProgress"
        and new.assignee is not empty
        and count(select where assignee = new.assignee and status = "inProgress" and id != new.id) >= 3
      deny "WIP limit reached: assignee already has 3 tasks in progress"
```

## Require description for high-priority tasks

Force authors to provide context before something becomes urgent.

```yaml
- description: high priority tasks must have a description
  ruki: >
    before update
      where new.priority <= 2 and new.description is empty
      deny "high priority tasks require a description"
```

## Auto-create next occurrence for recurring tasks

Recurring tasks automatically regenerate when completed.

```yaml
- description: spawn next occurrence when recurring task completes
  ruki: >
    after update
      where new.status = "done" and old.recurrence is not empty
      create title=old.title priority=old.priority tags=old.tags
             recurrence=old.recurrence due=next_date(old.recurrence) status="backlog"
```

## Return stale in-progress tasks to backlog

Tasks untouched for 7 days are likely blocked or abandoned.

```yaml
- description: return stale in-progress tasks to backlog after 7 days
  ruki: >
    every 1day
      update where status = "inProgress" and now() - updatedAt > 7day
      set status="backlog"
```

## Unblock tasks when dependencies complete

Automatically promote tasks to ready when all their dependencies are done.

```yaml
- description: unblock ready tasks when all dependencies complete
  ruki: >
    after update
      where new.status = "done" and old.status != "done"
      update where old.id in dependsOn and dependsOn all status = "done"
      set status="ready"
```

## Prevent re-opening completed tasks directly

Enforce a deliberate re-triage step through ready status.

```yaml
- description: prevent moving done tasks back to in-progress
  ruki: >
    before update
      where old.status = "done" and new.status = "inProgress"
      deny "cannot reopen completed tasks directly — move to ready first"
```

## Auto-assign creator on start

If you pull something into in-progress without assigning it, you implicitly own it.

```yaml
- description: auto-assign tasks to creator when moved to in-progress
  ruki: >
    after update
      where new.status = "inProgress" and new.assignee is empty
      update where id = new.id set assignee=user()
```

Note: this is the permissive alternative to the "require assignee before starting" trigger
that ships in the default workflow. Use one or the other, not both.

## Escalate overdue tasks

Overdue items that aren't already P1 get automatically escalated.

```yaml
- description: escalate overdue tasks by raising priority
  ruki: >
    every 1day
      update where due < now() and status not in ["done"] and priority > 1
      set priority=1
```

## Require points estimate before review

Ensures the team reflects on effort before review catches unestimated work.

```yaml
- description: tasks must be estimated before entering review
  ruki: >
    before update
      where new.status = "review" and new.points = 0
      deny "estimate points before moving to review"
```

## Auto-tag bugs as urgent when high priority

High-priority bugs should be visually flagged without relying on humans to remember.

```yaml
- description: auto-tag high priority bugs as urgent
  ruki: >
    after update
      where new.type = "bug" and new.priority <= 2 and "urgent" not in new.tags
      update where id = new.id set tags=tags + ["urgent"]
```

## Prevent epics from being assigned

Epics are containers, not actionable units — assign individual tasks instead.

```yaml
- description: epics track work groups and should not be assigned
  ruki: >
    before update
      where new.type = "epic" and new.assignee is not empty
      deny "epics represent work groups — assign individual tasks instead"
```

## Auto-remove urgent tag when priority drops

Paired with "auto-tag bugs as urgent" to keep tags in sync with priority.

```yaml
- description: remove urgent tag when priority is lowered
  ruki: >
    after update
      where old.priority <= 2 and new.priority > 2 and "urgent" in new.tags
      update where id = new.id set tags=tags - ["urgent"]
```

## Notify on critical task creation via webhook

P1s deserve immediate attention — shell integration bridges tiki to external alerting.

```yaml
- description: fire webhook when a P1 task is created
  ruki: >
    after create
      where new.priority = 1
      run("curl -s -X POST https://hooks.example.com/tiki -d 'P1 created: " + new.id + " - " + new.title + "'")
```

## Spike time-box

Spikes are investigation tasks — they should be time-boxed by definition.

```yaml
- description: time-box spikes to 3 days
  ruki: >
    every 1day
      update where type = "spike" and status = "inProgress" and now() - updatedAt > 3day
      set status="done"
```

## Require task breakdown for large estimates

Large estimates are a smell — nudge toward decomposition for better flow.

```yaml
- description: large tasks should be broken into smaller pieces
  ruki: >
    before update
      where new.points >= 8 and new.type != "epic"
      deny "tasks with 8+ points should be broken down — use an epic instead"
```

## Propagate priority from epic to children

When an epic gets escalated, its children should follow.

```yaml
- description: raise child task priority when epic priority increases
  ruki: >
    after update
      where new.type = "epic" and new.priority < old.priority
      update where new.id in dependsOn and priority > new.priority
      set priority=new.priority
```

## Auto-set due date for in-progress tasks

Tasks without deadlines drift forever — a default creates gentle pressure.

```yaml
- description: default 7-day due date when work starts
  ruki: >
    after update
      where new.status = "inProgress" and new.due is empty
      update where id = new.id set due=now() + 7day
```

## Require a title on creation

Every task needs at least a title — catch empty tasks at the gate.

```yaml
- description: tasks must have a title
  ruki: >
    before create
      where new.title is empty
      deny "tasks must have a title"
```

## Prevent creating tasks as done

New tasks should enter the workflow, not skip it entirely.

```yaml
- description: new tasks cannot start as done
  ruki: >
    before create
      where new.status = "done"
      deny "cannot create a task that is already done"
```

## Epics require a description at creation

Epics define scope — a description ensures the goal is documented upfront.

```yaml
- description: epics require a description at creation
  ruki: >
    before create
      where new.type = "epic" and new.description is empty
      deny "epics must have a description explaining the goal"
```

## Prevent deleting epics with active children

Deleting an epic should not orphan its child tasks.

```yaml
- description: cannot delete epics that still have linked tasks
  ruki: >
    before delete
      where old.type = "epic" and count(select where old.id in dependsOn and status != "done") > 0
      deny "cannot delete epic with active child tasks"
```

## Block deletion of high-priority tasks

P1 tasks require explicit demotion before they can be removed.

```yaml
- description: P1 tasks require explicit demotion before deletion
  ruki: >
    before delete
      where old.priority = 1
      deny "cannot delete a P1 task — lower priority first"
```
