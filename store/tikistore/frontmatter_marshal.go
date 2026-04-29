package tikistore

import (
	"fmt"
	"sort"
	"strings"

	taskpkg "github.com/boolean-maybe/tiki/task"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"

	"gopkg.in/yaml.v3"
)

// plainFrontmatter is the minimal YAML frontmatter written for plain
// documents — markdown files that have explicit `id` and optional `title`
// but no workflow fields. Keeping workflow keys OUT of the serialized YAML
// is what makes plain-doc creation survive a reload: if we wrote
// `type: story\nstatus: backlog` even when the in-memory task was flagged
// plain, the next load would classify the file as workflow via
// document.IsWorkflowFrontmatter and re-promote it.
type plainFrontmatter struct {
	ID    string `yaml:"id,omitempty"`
	Title string `yaml:"title,omitempty"`
}

// marshalFrontmatter serializes the YAML frontmatter for a task.
//
// Three cases, picked in order:
//
//  1. Plain task (IsWorkflow=false) → plainFrontmatter: only `id` + `title`
//     go to disk so a reload keeps classifying the file as plain.
//  2. Workflow task with a non-nil WorkflowFrontmatter → SPARSE emission:
//     only the keys in the preserved presence set are written, with values
//     taken from the current typed fields on the task (so edits persist).
//     This is the key Phase 4 invariant the review caught: a loaded sparse
//     workflow doc whose source had only `status:` must not gain `type:`,
//     `priority:`, etc. from a body-only UpdateDocument.
//  3. Workflow task with nil WorkflowFrontmatter → full-schema synthesis
//     (buildWorkflowFrontmatter). Used for newly created tasks whose
//     in-memory origin has no preserved presence set to honor.
//
// Custom fields and unknown fields are appended by saveTask itself and are
// intentionally out of scope here — plain docs may carry custom fields
// (e.g. an author-provided `project: foo`) without becoming workflow items,
// because `project` is not in the workflow-classification key set.
func marshalFrontmatter(task *taskpkg.Task) ([]byte, error) {
	if task == nil {
		return yaml.Marshal(plainFrontmatter{})
	}
	if !task.IsWorkflow {
		return yaml.Marshal(plainFrontmatter{
			ID:    task.ID,
			Title: task.Title,
		})
	}
	if task.WorkflowFrontmatter != nil {
		return marshalSparseWorkflowFrontmatter(task)
	}
	return yaml.Marshal(buildWorkflowFrontmatter(task))
}

// marshalSparseWorkflowFrontmatter writes YAML containing only the keys
// present in task.WorkflowFrontmatter. Values come from the current typed
// fields on the task so subsequent edits through the task API persist,
// while keys never present in the source (or explicitly cleared via
// MergeTypedWorkflowDeltas) stay out of the file.
//
// The output is a hand-built YAML block rather than a struct marshal: the
// workflow struct's yaml tags enforce a fixed schema we explicitly want to
// bypass here. Keys are sorted to keep file contents deterministic across
// runs.
func marshalSparseWorkflowFrontmatter(task *taskpkg.Task) ([]byte, error) {
	header := plainFrontmatter{ID: task.ID, Title: task.Title}
	headerBytes, err := yaml.Marshal(header)
	if err != nil {
		return nil, err
	}

	// Build the body keys in the canonical workflow order. Absent keys stay
	// absent; present keys are marshaled individually so per-field YAML
	// shapes (e.g. TagsValue) still apply.
	var body strings.Builder
	body.Write(headerBytes)

	keys := make([]string, 0, len(task.WorkflowFrontmatter))
	for k := range task.WorkflowFrontmatter {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fieldYAML, err := marshalWorkflowField(task, k)
		if err != nil {
			return nil, fmt.Errorf("marshaling %s: %w", k, err)
		}
		body.Write(fieldYAML)
	}
	return []byte(body.String()), nil
}

// marshalWorkflowField emits a single YAML line for the named workflow
// field using the task's current typed value. Reusing the PriorityValue /
// DueValue / etc. wrappers keeps the disk format identical to the full-
// schema serializer.
func marshalWorkflowField(task *taskpkg.Task, key string) ([]byte, error) {
	switch key {
	case "status":
		return yaml.Marshal(map[string]interface{}{key: taskpkg.StatusToString(task.Status)})
	case "type":
		return yaml.Marshal(map[string]interface{}{key: string(task.Type)})
	case "tags":
		tags := collectionutil.NormalizeStringSet(task.Tags)
		sort.Strings(tags)
		return yaml.Marshal(map[string]interface{}{key: taskpkg.TagsValue(tags)})
	case "dependsOn":
		deps := collectionutil.NormalizeRefSet(task.DependsOn)
		sort.Strings(deps)
		return yaml.Marshal(map[string]interface{}{key: taskpkg.DependsOnValue(deps)})
	case "due":
		return yaml.Marshal(map[string]interface{}{key: taskpkg.DueValue{Time: task.Due}})
	case "recurrence":
		return yaml.Marshal(map[string]interface{}{key: taskpkg.RecurrenceValue{Value: task.Recurrence}})
	case "assignee":
		return yaml.Marshal(map[string]interface{}{key: task.Assignee})
	case "priority":
		return yaml.Marshal(map[string]interface{}{key: taskpkg.PriorityValue(task.Priority)})
	case "points":
		return yaml.Marshal(map[string]interface{}{key: task.Points})
	}
	return nil, fmt.Errorf("unknown workflow field %q", key)
}

// buildWorkflowFrontmatter constructs the full workflow YAML struct with
// normalized tag/ref slices and sorted output. Extracted from saveTask so
// the plain-doc path can bypass it entirely.
func buildWorkflowFrontmatter(task *taskpkg.Task) taskFrontmatter {
	fm := taskFrontmatter{
		ID:         task.ID,
		Title:      task.Title,
		Type:       string(task.Type),
		Status:     taskpkg.StatusToString(task.Status),
		Tags:       task.Tags,
		DependsOn:  task.DependsOn,
		Due:        taskpkg.DueValue{Time: task.Due},
		Recurrence: taskpkg.RecurrenceValue{Value: task.Recurrence},
		Assignee:   task.Assignee,
		Priority:   taskpkg.PriorityValue(task.Priority),
		Points:     task.Points,
	}

	fm.Tags = collectionutil.NormalizeStringSet(fm.Tags)
	fm.DependsOn = collectionutil.NormalizeRefSet(fm.DependsOn)
	if len(fm.Tags) > 0 {
		sort.Strings(fm.Tags)
	}
	if len(fm.DependsOn) > 0 {
		sort.Strings(fm.DependsOn)
	}
	return fm
}
