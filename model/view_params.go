package model

import taskpkg "github.com/boolean-maybe/tiki/task"

// typed view params live here to avoid stringly-typed param maps being spread across layers.

const (
	paramTaskID    = "taskID"
	paramDraftTask = "draftTask"
	paramFocus     = "focus"
)

// TaskDetailParams are params for TaskDetailViewID.
type TaskDetailParams struct {
	TaskID string
}

// EncodeTaskDetailParams converts typed params into a navigation params map.
func EncodeTaskDetailParams(p TaskDetailParams) map[string]interface{} {
	if p.TaskID == "" {
		return nil
	}
	return map[string]interface{}{
		paramTaskID: p.TaskID,
	}
}

// DecodeTaskDetailParams converts a navigation params map into typed params.
func DecodeTaskDetailParams(params map[string]interface{}) TaskDetailParams {
	var p TaskDetailParams
	if params == nil {
		return p
	}
	if id, ok := params[paramTaskID].(string); ok {
		p.TaskID = id
	}
	return p
}

// TaskEditParams are params for TaskEditViewID.
type TaskEditParams struct {
	TaskID string
	Draft  *taskpkg.Task
	Focus  EditField
}

// EncodeTaskEditParams converts typed params into a navigation params map.
func EncodeTaskEditParams(p TaskEditParams) map[string]interface{} {
	if p.TaskID == "" && p.Draft != nil {
		p.TaskID = p.Draft.ID
	}
	if p.TaskID == "" {
		return nil
	}

	m := map[string]interface{}{
		paramTaskID: p.TaskID,
	}
	if p.Draft != nil {
		m[paramDraftTask] = p.Draft
	}
	if p.Focus != "" {
		// Store focus as a plain string for interop and stable params fingerprinting.
		m[paramFocus] = string(p.Focus)
	}
	return m
}

// DecodeTaskEditParams converts a navigation params map into typed params.
func DecodeTaskEditParams(params map[string]interface{}) TaskEditParams {
	var p TaskEditParams
	if params == nil {
		return p
	}
	if id, ok := params[paramTaskID].(string); ok {
		p.TaskID = id
	}
	if draft, ok := params[paramDraftTask].(*taskpkg.Task); ok {
		p.Draft = draft
		if p.TaskID == "" && draft != nil {
			p.TaskID = draft.ID
		}
	}
	switch f := params[paramFocus].(type) {
	case string:
		p.Focus = EditField(f)
	case EditField:
		p.Focus = f
	}
	return p
}
