# Quick capture

Create tikis straight from the command line.

First line becomes the title. Everything after becomes the description.

## What fields the new tiki gets

The active workflow decides which fields the captured tiki carries. Every workflow field declared
in `workflow.yaml fields:` that carries a default contributes one frontmatter key on capture:

- Enum fields apply the value marked `default: true` (typically `status: backlog`, `type: story`).
- Non-enum fields apply their declared `default:` value (e.g. `priority: 3`, `points: 1`,
  `tags: ["idea"]`).
- Fields with no declared default are absent from the captured frontmatter — the tiki only carries
  what the workflow asked for.

If the workflow declares no defaults at all, capture produces a tiki with only `id:` and `title:`
in its frontmatter — useful for notes-only projects where piped input should be a plain document
rather than a tracked task.

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