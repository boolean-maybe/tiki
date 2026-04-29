package store

import (
	"testing"
)

// TestParseIdentityFromContent_NoExplicitStatusIsSkipped verifies the
// presence-aware rule: burndown only counts versions that carry an
// explicit `status` field. Docs with no frontmatter, docs with only
// non-status metadata, and docs with workflow-adjacent fields like
// `priority` or `type` but no `status` must all be skipped.
func TestParseIdentityFromContent_NoExplicitStatusIsSkipped(t *testing.T) {
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
		{
			name:    "priority set but no status — formerly counted, now skipped",
			content: "---\nid: PARTL1\ntitle: partial\npriority: 1\n---\nbody\n",
		},
		{
			name:    "type set but no status — formerly counted, now skipped",
			content: "---\nid: PARTL2\ntitle: partial\ntype: story\n---\nbody\n",
		},
		{
			name:    "empty string status — treated as absent",
			content: "---\nid: PARTL3\ntitle: partial\nstatus: \"\"\n---\nbody\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ident, err := parseIdentityFromContent(tc.content)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if ident.hasExplicitStat {
				t.Errorf("expected hasExplicitStat=false; version would leak into burndown")
			}
		})
	}
}

// TestParseIdentityFromContent_ExplicitStatusIsCounted verifies the happy
// path: a version with an explicit non-empty `status` is counted and its
// id is taken from the frontmatter (not the filename).
func TestParseIdentityFromContent_ExplicitStatusIsCounted(t *testing.T) {
	content := "---\nid: WORK01\ntitle: work item\ntype: story\nstatus: ready\npriority: 1\n---\nbody\n"
	ident, err := parseIdentityFromContent(content)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ident.hasExplicitStat {
		t.Error("expected hasExplicitStat=true for doc with explicit status")
	}
	if string(ident.status) == "" {
		t.Error("expected non-empty status for workflow doc")
	}
	if ident.id != "WORK01" {
		t.Errorf("expected id=WORK01 from frontmatter, got %q", ident.id)
	}
}
