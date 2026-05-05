package model

import (
	"strings"
)

// ViewID identifies a view type
type ViewID string

// view identifiers
const (
	// TaskDetailViewID identifies the legacy built-in task detail view.
	// Phase 3 cleanup: this view is no longer the normal detail path —
	// `kind: detail` declared in workflow.yaml is. The constant is
	// retained only because the new-task draft creation flow still
	// pushes the built-in TaskEditView (which sits next to
	// TaskDetailView in the file tree) and a few `Edit source`/agent-chat
	// branches still target the built-in view. New code should push the
	// configurable detail view via DetailPluginViewID instead.
	TaskDetailViewID ViewID = "task_detail"

	// TaskEditViewID identifies the built-in task edit view. Retained
	// reason (Phase 3): the `n` (new-task) flow still uses this view to
	// host a draft tiki being created, since the configurable detail
	// view does not yet support drafts. Edit-of-existing flows have
	// migrated to in-place edit mode on the configurable detail view.
	TaskEditViewID ViewID = "task_edit"

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
// that previously pushed the legacy TaskDetailViewID and now route
// through the configurable detail view declared in workflow.yaml.
func DetailPluginViewID() ViewID {
	return MakePluginViewID(DetailPluginName)
}

// built-in view names and descriptions for the header info section
const (
	TaskDetailViewName = "Tiki Detail"
	TaskDetailViewDesc = "tiki overview. Quick edit, edit dependencies, tags or edit source file"

	TaskEditViewName = "Task Edit"
	TaskEditViewDesc = "Cycle through fields to edit title, status, priority and other"

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
