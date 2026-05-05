# Quick capture

Create tikis straight from the command line.

First line becomes the title. Everything after becomes the description.

## What fields the new tiki gets

The active workflow decides which fields the captured tiki carries:

- Workflows with a `default: true` status (kanban, todo, bug-tracker) capture input with status,
  type, priority, and points filled in from registry defaults. The new tiki shows up on every view
  whose lane filter matches those fields.
- Workflows with no `default: true` status capture input with only `id:` and `title:` in the
  frontmatter. The new tiki stays out of views that require schema-known fields. Useful for
  notes-only projects where piped input should be a note rather than a tracked task.

## Examples

### Quick capture an idea
```bash
echo "cool idea" | tiki
```

### Turn a GitHub issue into a tiki task
```bash
gh issue view 42 --json title,body -q '"\(.title)\n\n\(.body)"' | tiki
```

### Capture a bug report from an API
```bash
curl -s https://sentry.io/api/issues/latest/ | jq -r '.title' | tiki
```

### Scan a log file and create a task for every error
```bash
grep ERROR server.log | sort -u | while read -r line; do echo "$line" | tiki; done
```

### Create a task from a file
```bash
tiki < bug-report.md
```

### Bulk-import tasks from a file
```bash
while read -r line; do echo "$line" | tiki; done < backlog.txt
```

### Chain with other tools
```bash
id=$(echo "Deploy v2.3 to staging" | tiki) && echo "Tracked as $id"
```

## Input format

| Input | Title | Description |
|---|---|---|
| `echo "Fix the bug"` | Fix the bug | *(empty)* |
| `printf "Title\n\nDetails here"` | Title | Details here |