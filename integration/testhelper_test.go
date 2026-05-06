package integration

// testWorkflowPreamble contains the required status and type registry fields
// for integration test workflow.yaml files. All workflow sections come from
// the single highest-priority file, so test workflows must be self-contained.
const testWorkflowPreamble = `fields:
  - name: status
    type: enum
    values:
      - value: backlog
        label: Backlog
        emoji: "📥"
        default: true
      - value: ready
        label: Ready
        emoji: "📋"
        active: true
      - value: inProgress
        label: "In Progress"
        emoji: "⚙️"
        active: true
      - value: review
        label: Review
        emoji: "👀"
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
      - value: spike
        label: Spike
        emoji: "🔍"
      - value: epic
        label: Epic
        emoji: "🗂️"
`
