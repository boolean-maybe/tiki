package integration

// testWorkflowPreamble contains the required statuses and types sections
// for integration test workflow.yaml files. All workflow sections come from
// the single highest-priority file, so test workflows must be self-contained.
const testWorkflowPreamble = `statuses:
  - key: backlog
    label: Backlog
    emoji: "📥"
    default: true
  - key: ready
    label: Ready
    emoji: "📋"
    active: true
  - key: inProgress
    label: "In Progress"
    emoji: "⚙️"
    active: true
  - key: review
    label: Review
    emoji: "👀"
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
  - key: spike
    label: Spike
    emoji: "🔍"
  - key: epic
    label: Epic
    emoji: "🗂️"
`
