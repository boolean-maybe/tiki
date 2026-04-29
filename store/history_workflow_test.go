package store

import (
	"testing"
)

// TestParseStatusFromContent_PlainDocIsNotWorkflow verifies the M2 fix:
// a doc with frontmatter but no workflow fields is NOT treated as
// DefaultStatus; the returned isWorkflow flag is false and burndown skips it.
func TestParseStatusFromContent_PlainDocIsNotWorkflow(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			name:    "no frontmatter at all",
			content: "# Just a heading\n\nno fm here\n",
		},
		{
			name:    "frontmatter with only id and title",
			content: "---\nid: PLAIN1\ntitle: A plain doc\n---\n\nbody text\n",
		},
		{
			name:    "frontmatter with unknown keys but no workflow keys",
			content: "---\nid: PLAIN2\ntitle: Custom fields\nauthor: alice\n---\nbody\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, isWorkflow, err := parseStatusFromContent(tc.content)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if isWorkflow {
				t.Errorf("expected isWorkflow=false, got true — plain doc would leak into burndown")
			}
		})
	}
}

// TestParseStatusFromContent_WorkflowDocIsCountedAsWorkflow verifies the
// happy path: a doc with any workflow field comes back with isWorkflow=true
// and the parsed status.
func TestParseStatusFromContent_WorkflowDocIsCountedAsWorkflow(t *testing.T) {
	content := "---\nid: WORK01\ntitle: work item\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n"
	status, isWorkflow, err := parseStatusFromContent(content)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !isWorkflow {
		t.Error("expected isWorkflow=true for doc with workflow fields")
	}
	if string(status) == "" {
		t.Error("expected non-empty status for workflow doc")
	}
}
