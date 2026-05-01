# Quick capture

Create documents straight from the command line.

First line becomes the title. Everything after becomes the description.

## Workflow task vs plain document

Whether the new document is a **workflow task** or a **plain document** is decided by the active
workflow:

- Workflows with a `default: true` status (kanban, todo, bug-tracker) capture input as **workflow
  tasks** — status, type, priority, and points are filled in from registry defaults, and the new
  item shows up on board/list views.
- Workflows with no `default: true` status capture input as **plain documents** — only `id:` and
  `title:` go in the frontmatter, and the document stays out of workflow views. Useful for
  notes-only projects where piped input should be a note, not a task.

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