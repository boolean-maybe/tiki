# Quick capture

Create tiki tasks straight from the command line
First line becomes the title. Everything after becomes the description.

## Examples

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