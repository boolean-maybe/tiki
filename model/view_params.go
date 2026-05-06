package model

import tikipkg "github.com/boolean-maybe/tiki/tiki"

// typed view params live here to avoid stringly-typed param maps being
// spread across layers. The configurable detail view uses PluginViewParams
// (see below); only TaskEditParams is unique to a built-in view now that
// the legacy TaskDetailViewID is gone.

const (
	paramTaskID    = "taskID"
	paramDraftTask = "draftTask"
	paramFocus     = "focus"
	paramDescOnly  = "descOnly"
	paramTagsOnly  = "tagsOnly"
	paramMetadata  = "metadata"
)

// TaskEditParams are params for TaskEditViewID. Metadata carries the
// metadata field list the source view (typically a configurable detail
// view) was using. When non-nil, the factory uses it directly; nil falls
// through to the workflow-driven Detail-plugin lookup, then to a
// hardcoded last-resort default.
type TaskEditParams struct {
	TaskID   string
	Draft    *tikipkg.Tiki
	Focus    EditField
	DescOnly bool
	TagsOnly bool
	Metadata []string
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
	if len(p.Metadata) > 0 {
		// store a defensive copy so later mutations to the source slice
		// don't leak into the encoded params.
		cp := make([]string, len(p.Metadata))
		copy(cp, p.Metadata)
		m[paramMetadata] = cp
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
	if md, ok := params[paramMetadata].([]string); ok {
		p.Metadata = md
	}
	return p
}
