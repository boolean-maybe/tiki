package model

import (
	"strings"
)

// ViewID identifies a view type
type ViewID string

// view identifiers
const (
	TaskDetailViewID   ViewID = "task_detail"
	TaskEditViewID     ViewID = "task_edit"
	PluginViewIDPrefix ViewID = "plugin:" // Prefix for plugin views
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
