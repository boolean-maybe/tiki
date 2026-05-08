package integration

// testWorkflowPreamble contains the required workflow field declarations for
// integration test workflow.yaml files. All workflow sections come from the
// single highest-priority file, so test workflows must be self-contained.
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
      - value: inProgress
        label: "In Progress"
        emoji: "⚙️"
      - value: review
        label: Review
        emoji: "👀"
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
      - value: spike
        label: Spike
        emoji: "🔍"
      - value: epic
        label: Epic
        emoji: "🗂️"
  - name: priority
    type: enum
    values:
      - {value: high, label: High, emoji: "🔴"}
      - {value: medium-high, label: "Medium High", emoji: "🟠"}
      - {value: medium, label: Medium, emoji: "🟡", default: true}
      - {value: medium-low, label: "Medium Low", emoji: "🟢"}
      - {value: low, label: Low, emoji: "🔵"}
  - name: points
    type: integer
    default: 1
  - name: tags
    type: stringList
  - name: dependsOn
    type: taskIdList
  - name: due
    type: date
  - name: recurrence
    type: recurrence
  - name: assignee
    type: text
`
