package document

// workflowFrontmatterKeys lists frontmatter keys that signal a document is
// participating in the workflow. Presence of any one of these in the file's
// frontmatter promotes a plain markdown document to a workflow item; absence
// of all of them means the document is a plain doc and should not appear on
// boards, burndown, or any other workflow-only surface.
//
// Kept in the document package so the classifier has a single source of
// truth across the store boundary and repair tooling.
var workflowFrontmatterKeys = map[string]struct{}{
	"status":     {},
	"type":       {},
	"priority":   {},
	"points":     {},
	"tags":       {},
	"dependsOn":  {},
	"due":        {},
	"recurrence": {},
	"assignee":   {},
}

// IsWorkflowFrontmatter reports whether fm carries any explicit workflow
// field. The caller typically passes the raw decoded frontmatter map.
//
// This implements the presence-aware contract from the plan: a plain doc with
// only `id` and `title` is NOT workflow-capable, even if the surrounding
// type used to default its status/type/priority to non-zero values.
func IsWorkflowFrontmatter(fm map[string]interface{}) bool {
	if fm == nil {
		return false
	}
	for k := range fm {
		if _, ok := workflowFrontmatterKeys[k]; ok {
			return true
		}
	}
	return false
}
