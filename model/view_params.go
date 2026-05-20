package model

import (
	"github.com/boolean-maybe/tiki/gridlayout"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// typed view params live here to avoid stringly-typed param maps being
// spread across layers. The configurable detail view uses PluginViewParams
// (see below); only TikiEditParams is unique to a built-in view now that
// the legacy TikiDetailViewID is gone.

const (
	paramTikiID    = "tikiID"
	paramDraftTiki = "draftTiki"
	paramFocus     = "focus"
	paramDescOnly  = "descOnly"
	paramTagsOnly  = "tagsOnly"
	paramLayout    = "layout"
	paramSpec      = "spec"
)

// TikiEditParams are params for TikiEditViewID. Layout carries the
// layout field list the source view (typically a configurable detail
// view) was using. When non-nil, the factory uses it directly; nil falls
// through to the workflow-driven Detail-plugin lookup, then to a
// hardcoded last-resort default.
type TikiEditParams struct {
	TikiID   string
	Draft    *tikipkg.Tiki
	Focus    EditField
	DescOnly bool
	TagsOnly bool
	Layout   []string
	Spec     gridlayout.GridSpec
}

// EncodeTikiEditParams converts typed params into a navigation params map.
func EncodeTikiEditParams(p TikiEditParams) map[string]interface{} {
	if p.TikiID == "" && p.Draft != nil {
		p.TikiID = p.Draft.ID
	}
	if p.TikiID == "" {
		return nil
	}

	m := map[string]interface{}{
		paramTikiID: p.TikiID,
	}
	if p.Draft != nil {
		m[paramDraftTiki] = p.Draft
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
	if len(p.Layout) > 0 {
		// store a defensive copy so later mutations to the source slice
		// don't leak into the encoded params.
		cp := make([]string, len(p.Layout))
		copy(cp, p.Layout)
		m[paramLayout] = cp
	}
	if len(p.Spec.Anchors) > 0 {
		m[paramSpec] = p.Spec
	}
	return m
}

// PluginViewParams are params accepted by any plugin view. TikiID carries the
// selected document across `kind: view` navigation (6B.3) and drives
// `kind: detail` rendering (6B.2). Boards / lists / wikis ignore TikiID —
// only the detail kind reads it today.
type PluginViewParams struct {
	TikiID string
}

// EncodePluginViewParams serializes PluginViewParams to a navigation map.
// Returns nil when there is nothing to carry — callers that want "push a
// view with no selection context" can pass nil directly.
func EncodePluginViewParams(p PluginViewParams) map[string]interface{} {
	if p.TikiID == "" {
		return nil
	}
	return map[string]interface{}{paramTikiID: p.TikiID}
}

// DecodePluginViewParams extracts PluginViewParams from a navigation map.
// Unknown keys are ignored; an empty map yields a zero-valued struct.
func DecodePluginViewParams(params map[string]interface{}) PluginViewParams {
	var p PluginViewParams
	if params == nil {
		return p
	}
	if id, ok := params[paramTikiID].(string); ok {
		p.TikiID = id
	}
	return p
}

// DecodeTikiEditParams converts a navigation params map into typed params.
func DecodeTikiEditParams(params map[string]interface{}) TikiEditParams {
	var p TikiEditParams
	if params == nil {
		return p
	}
	if id, ok := params[paramTikiID].(string); ok {
		p.TikiID = id
	}
	if draft, ok := params[paramDraftTiki].(*tikipkg.Tiki); ok {
		p.Draft = draft
		if p.TikiID == "" && draft != nil {
			p.TikiID = draft.ID
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
	if md, ok := params[paramLayout].([]string); ok {
		p.Layout = md
	}
	if s, ok := params[paramSpec].(gridlayout.GridSpec); ok {
		p.Spec = s
	}
	return p
}
