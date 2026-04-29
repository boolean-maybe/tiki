package tikistore

// workflowKeys lists the frontmatter keys that carry workflow semantics.
// Kept here rather than re-exported from task so the load path does not take
// on a task-package dependency just for a string list.
var workflowKeys = []string{
	"status",
	"type",
	"tags",
	"dependsOn",
	"due",
	"recurrence",
	"assignee",
	"priority",
	"points",
}

// workflowSubsetFromFrontmatter returns the verbatim subset of src
// containing only workflow keys. Absent keys stay absent, so the returned
// map is a presence-preserving snapshot of the source YAML.
//
// This is the input to Task.WorkflowFrontmatter on the load path. Callers
// on the save/update path typically do NOT need to refresh this — the
// workflow-field serialization in saveTask emits every typed field anyway
// for workflow docs, and the preserved map is only read by ToDocument.
func workflowSubsetFromFrontmatter(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	var out map[string]interface{}
	for _, k := range workflowKeys {
		if v, ok := src[k]; ok {
			if out == nil {
				out = make(map[string]interface{})
			}
			out[k] = v
		}
	}
	return out
}
