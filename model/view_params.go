package model

import tikipkg "github.com/boolean-maybe/tiki/tiki"

// typed view params live here to avoid stringly-typed param maps being spread across layers.

const (
	paramTaskID    = "taskID"
	paramDraftTask = "draftTask"
	paramFocus     = "focus"
	paramDescOnly  = "descOnly"
	paramTagsOnly  = "tagsOnly"
	paramReadOnly  = "readOnly"
)

// TaskDetailParams are params for TaskDetailViewID.
type TaskDetailParams struct {
	TaskID   string
	ReadOnly bool
}

// EncodeTaskDetailParams converts typed params into a navigation params map.
func EncodeTaskDetailParams(p TaskDetailParams) map[string]interface{} {
	if p.TaskID == "" {
		return nil
	}
	m := map[string]interface{}{
		paramTaskID: p.TaskID,
	}
	if p.ReadOnly {
		m[paramReadOnly] = true
	}
	return m
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
	if readOnly, ok := params[paramReadOnly].(bool); ok {
		p.ReadOnly = readOnly
	}
	return p
}

// TaskEditParams are params for TaskEditViewID.
type TaskEditParams struct {
	TaskID   string
	Draft    *tikipkg.Tiki
	Focus    EditField
	DescOnly bool
	TagsOnly bool
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
	if p.DescOnly {
		m[paramDescOnly] = true
	}
	if p.TagsOnly {
		m[paramTagsOnly] = true
	}
	return m
}

// PluginViewParams are params accepted by any plugin view. TaskID carries the
// selected document across `kind: view` navigation (6B.3) and drives
// `kind: detail` rendering (6B.2). Boards / lists / wikis ignore TaskID —
// only the detail kind reads it today.
type PluginViewParams struct {
	TaskID string
}

// EncodePluginViewParams serializes PluginViewParams to a navigation map.
// Returns nil when there is nothing to carry — callers that want "push a
// view with no selection context" can pass nil directly.
func EncodePluginViewParams(p PluginViewParams) map[string]interface{} {
	if p.TaskID == "" {
		return nil
	}
	return map[string]interface{}{paramTaskID: p.TaskID}
}

// DecodePluginViewParams extracts PluginViewParams from a navigation map.
// Unknown keys are ignored; an empty map yields a zero-valued struct.
func DecodePluginViewParams(params map[string]interface{}) PluginViewParams {
	var p PluginViewParams
	if params == nil {
		return p
	}
	if id, ok := params[paramTaskID].(string); ok {
		p.TaskID = id
	}
	return p
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
	if draft, ok := params[paramDraftTask].(*tikipkg.Tiki); ok {
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
	if descOnly, ok := params[paramDescOnly].(bool); ok {
		p.DescOnly = descOnly
	}
	if tagsOnly, ok := params[paramTagsOnly].(bool); ok {
		p.TagsOnly = tagsOnly
	}
	return p
}
