package model

import (
	"strings"
)

// ViewID identifies a view type
type ViewID string

// view identifiers
const (
	// TikiEditViewID identifies the built-in task edit view. Retained
	// reason: the `n` (new-task) flow uses this view to host a draft
	// tiki being created, since the configurable detail view does not
	// support drafts. Edit-of-existing flows have migrated to in-place
	// edit mode on the configurable detail view.
	TikiEditViewID ViewID = "tiki_edit"

	PluginViewIDPrefix ViewID = "plugin:" // Prefix for plugin views

	// DetailPluginName is the conventional workflow `views[].name` for
	// the configurable detail view. The bundled kanban workflow uses
	// this name; internal navigations that need to open "the" detail
	// view target this name. If a workflow renames its detail view,
	// internal callers will still target this name and may fall through
	// to no-op — that's acceptable because workflows that rename the
	// view should also rebind the open keys themselves.
	DetailPluginName = "Detail"
)

// DetailPluginViewID returns the canonical configurable-detail view id
// (`plugin:Detail`). Used by internal navigations (deps editor, etc.)
// that previously pushed the legacy TikiDetailViewID and now route
// through the configurable detail view declared in workflow.yaml.
func DetailPluginViewID() ViewID {
	return MakePluginViewID(DetailPluginName)
}

// built-in view names and descriptions for the header info section
const (
	TikiEditViewName = "Tiki Edit"
	TikiEditViewDesc = "Cycle through fields to edit title, status, priority and other"

	DepsEditorViewName = "Dependencies"
	DepsEditorViewDesc = "Move a tiki to Blocks to make it block edited tiki. Move it to Depends to make edited tiki depend on it"
)

// IsPluginViewID checks if a ViewID is for a plugin view
func IsPluginViewID(id ViewID) bool {
	return strings.HasPrefix(string(id), string(PluginViewIDPrefix))
}

// GetPluginName extracts the plugin name from a plugin ViewID
func GetPluginName(id ViewID) string {
	return strings.TrimPrefix(string(id), string(PluginViewIDPrefix))
}

// MakePluginViewID creates a ViewID for a plugin with the given name
func MakePluginViewID(name string) ViewID {
	return ViewID(string(PluginViewIDPrefix) + name)
}
