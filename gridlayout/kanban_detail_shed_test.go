package gridlayout

import (
	"strings"
	"testing"
)

// TestKanbanDetail_CoreFieldsSurviveNarrowBox is the end-to-end regression for
// the smoke-test defect: at an ~80-col box the bundled kanban Detail grid shed
// every core field (status/type/priority/points/assignee/…/due) and kept only
// the optional rightmost Tags column. Under position-priority shedding the core
// columns (left) must survive and the optional Tags/Deps columns (right) shed.
func TestKanbanDetail_CoreFieldsSurviveNarrowBox(t *testing.T) {
	raw := []string{
		`<highlight>title             | -- | -- | -- | -- | -- | -- | --`,
		`_ | _ | _ | _ | _ | _ | _ | _`,
		`<text.label>status.caption   | (status.label + " " + status.visual):16.. | <text.label>assignee.caption  | assignee:18.. | <text.label>due.caption        | due:12..    | <text.label>tags.caption | <text.label>dependsOn.caption`,
		`<text.label>type.caption     | type.label + " " + type.visual            | <text.muted>createdBy.caption | createdBy     | <text.label>recurrence.caption | recurrence? | tags?:18..               | dependsOn?:16..fr`,
		`<text.label>priority.caption | priority                                  | <text.muted>createdAt.caption | createdAt     | _                              | _           | ^                        | ^`,
		`<text.label>points.caption   | points                                    | <text.muted>updatedAt.caption | updatedAt     | _                              | _           | ^                        | ^`,
	}
	rows := make([][]string, len(raw))
	for i, line := range raw {
		parts := strings.Split(line, "|")
		for j := range parts {
			parts[j] = strings.TrimSpace(parts[j])
		}
		rows[i] = parts
	}
	spec, err := ParseGrid(rows)
	if err != nil {
		t.Fatalf("ParseGrid: %v", err)
	}

	// realistic content widths for a rendered tiki.
	measure := func(a Anchor) int {
		if a.Display == DisplayCaption {
			return len([]rune(a.Name)) // caption text ≈ field name length
		}
		switch a.Name {
		case "status", "type", "priority", "points", "assignee", "createdBy", "createdAt", "updatedAt":
			return 12
		case "due":
			return 10
		case "recurrence":
			return 8
		case "tags":
			return 16
		case "dependsOn":
			return 12
		case "title":
			return 20
		}
		return len([]rune(a.Text))
	}

	plan := SolveLayout(spec, 80, 1, measure, nil)

	// the core status field must survive: its caption (col0) and value (col1).
	coreCols := map[string]int{"status.caption(col0)": 0, "status.value(col1)": 1}
	for label, c := range coreCols {
		if plan.Dropped[c] {
			t.Errorf("%s wrongly shed at 80 cols; dropped=%v", label, plan.Dropped)
		}
	}
	// the optional Tags column (col6) must shed before the core fields.
	if !plan.Dropped[6] {
		t.Errorf("optional Tags column (col6) should shed at 80 cols; dropped=%v", plan.Dropped)
	}
}
