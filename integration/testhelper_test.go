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
        visual: "📥"
        default: true
      - value: ready
        label: Ready
        visual: "📋"
      - value: inProgress
        label: "In Progress"
        visual: "⚙️"
      - value: review
        label: Review
        visual: "👀"
      - value: done
        label: Done
        visual: "✅"
  - name: type
    type: enum
    values:
      - value: story
        label: Story
        visual: "🌀"
        default: true
      - value: bug
        label: Bug
        visual: "💥"
      - value: spike
        label: Spike
        visual: "🔍"
      - value: epic
        label: Epic
        visual: "🗂️"
  - name: priority
    type: enum
    values:
      - {value: high, label: High, visual: "🔴"}
      - {value: medium-high, label: "Medium High", visual: "🟠"}
      - {value: medium, label: Medium, visual: "🟡", default: true}
      - {value: medium-low, label: "Medium Low", visual: "🟢"}
      - {value: low, label: Low, visual: "🔵"}
  - name: points
    type: enum
    values:
      - {value: "11", label: "11", visual: "<accent>❚❚❚❚❚❚❚❚❚❚❚"}
      - {value: "7",  label: "7",  visual: "<accent>❚❚❚❚❚❚❚<muted>❘❘❘❘"}
      - {value: "3",  label: "3",  visual: "<accent>❚❚❚<muted>❘❘❘❘❘❘❘❘", default: true}
      - {value: "1",  label: "1",  visual: "<accent>❚<muted>❘❘❘❘❘❘❘❘❘❘"}
  - name: escalations
    type: integer
    default: 0
  - name: tags
    type: stringList
  - name: dependsOn
    type: tikiIdList
  - name: due
    type: date
  - name: recurrence
    type: recurrence
  - name: assignee
    type: text
`
