package model

import (
	"github.com/boolean-maybe/tiki/plugin"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// typed view params live here to avoid stringly-typed param maps being
// spread across layers. PluginViewParams is the only typed wrapper now —
// all detail-edit modes flow through it.

const (
	paramTikiID    = "tikiID"
	paramDraftTiki = "draftTiki"
	paramFocus     = "focus"
	paramMode      = "mode"
)

// PluginViewParams are params accepted by any plugin view. TikiID carries the
// selected document across `kind: view` navigation (6B.3) and drives
// `kind: detail` rendering (6B.2). Boards / lists / wikis ignore TikiID —
// only the detail kind reads it today.
//
// Mode/Focus/Draft are consumed by `kind: detail` views via
// DetailController.ApplyDetailMode (called from view/factory.go after
// BindEditView). Mode is the closed vocabulary declared on the source
// PluginAction; Focus is the metadata field to focus on entry; Draft is
// only set for mode: new and carries a freshly synthesized tiki that the
// edit session adopts as its in-flight draft.
type PluginViewParams struct {
	TikiID string
	Mode   plugin.DetailMode
	Focus  EditField
	Draft  *tikipkg.Tiki
}

// EncodePluginViewParams serializes PluginViewParams to a navigation map.
// Returns nil when there is nothing to carry — no TikiID, no Draft, no
// Mode. For mode: new the dispatcher passes Draft != nil with TikiID
// empty; the encoder lifts the draft's ID into TikiID so the wire keys
// stay consistent.
func EncodePluginViewParams(p PluginViewParams) map[string]interface{} {
	if p.TikiID == "" && p.Draft != nil {
		p.TikiID = p.Draft.ID
	}
	if p.TikiID == "" && p.Draft == nil && p.Mode == "" {
		return nil
	}
	m := map[string]interface{}{}
	if p.TikiID != "" {
		m[paramTikiID] = p.TikiID
	}
	if p.Draft != nil {
		m[paramDraftTiki] = p.Draft
	}
	if p.Focus != "" {
		m[paramFocus] = string(p.Focus)
	}
	if p.Mode != "" {
		m[paramMode] = string(p.Mode)
	}
	return m
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
	switch m := params[paramMode].(type) {
	case string:
		p.Mode = plugin.DetailMode(m)
	case plugin.DetailMode:
		p.Mode = m
	}
	return p
}
