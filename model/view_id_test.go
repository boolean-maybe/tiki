package model

import (
	"testing"
)

func TestIsPluginViewID(t *testing.T) {
	tests := []struct {
		name     string
		viewID   ViewID
		expected bool
	}{
		{
			name:     "plugin view with name",
			viewID:   "plugin:burndown",
			expected: true,
		},
		{
			name:     "plugin view with hyphenated name",
			viewID:   "plugin:my-plugin",
			expected: true,
		},
		{
			name:     "plugin view with underscore",
			viewID:   "plugin:my_plugin",
			expected: true,
		},
		{
			name:     "task detail view",
			viewID:   TaskDetailViewID,
			expected: false,
		},
		{
			name:     "task edit view",
			viewID:   TaskEditViewID,
			expected: false,
		},
		{
			name:     "empty string",
			viewID:   "",
			expected: false,
		},
		{
			name:     "plugin prefix only",
			viewID:   PluginViewIDPrefix,
			expected: true,
		},
		{
			name:     "plugin with colon in name",
			viewID:   "plugin:name:with:colons",
			expected: true,
		},
		{
			name:     "starts with plugin but no colon",
			viewID:   "pluginview",
			expected: false,
		},
		{
			name:     "contains plugin but not at start",
			viewID:   "myplugin:view",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPluginViewID(tt.viewID)
			if got != tt.expected {
				t.Errorf("IsPluginViewID(%q) = %v, want %v", tt.viewID, got, tt.expected)
			}
		})
	}
}

func TestGetPluginName(t *testing.T) {
	tests := []struct {
		name         string
		viewID       ViewID
		expectedName string
	}{
		{
			name:         "simple plugin name",
			viewID:       "plugin:burndown",
			expectedName: "burndown",
		},
		{
			name:         "hyphenated plugin name",
			viewID:       "plugin:my-plugin",
			expectedName: "my-plugin",
		},
		{
			name:         "underscored plugin name",
			viewID:       "plugin:my_plugin",
			expectedName: "my_plugin",
		},
		{
			name:         "plugin prefix only",
			viewID:       PluginViewIDPrefix,
			expectedName: "",
		},
		{
			name:         "plugin with colon in name",
			viewID:       "plugin:name:with:colons",
			expectedName: "name:with:colons",
		},
		{
			name:         "non-plugin view returns full ID",
			viewID:       TaskDetailViewID,
			expectedName: "task_detail",
		},
		{
			name:         "empty string",
			viewID:       "",
			expectedName: "",
		},
		{
			name:         "plugin with numbers",
			viewID:       "plugin:plugin123",
			expectedName: "plugin123",
		},
		{
			name:         "plugin with special chars",
			viewID:       "plugin:my.plugin-v2_test",
			expectedName: "my.plugin-v2_test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPluginName(tt.viewID)
			if got != tt.expectedName {
				t.Errorf("GetPluginName(%q) = %q, want %q", tt.viewID, got, tt.expectedName)
			}
		})
	}
}

func TestMakePluginViewID(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
		expected   ViewID
	}{
		{
			name:       "simple name",
			pluginName: "burndown",
			expected:   "plugin:burndown",
		},
		{
			name:       "hyphenated name",
			pluginName: "my-plugin",
			expected:   "plugin:my-plugin",
		},
		{
			name:       "underscored name",
			pluginName: "my_plugin",
			expected:   "plugin:my_plugin",
		},
		{
			name:       "empty name",
			pluginName: "",
			expected:   PluginViewIDPrefix,
		},
		{
			name:       "name with colon",
			pluginName: "name:with:colons",
			expected:   "plugin:name:with:colons",
		},
		{
			name:       "name with numbers",
			pluginName: "plugin123",
			expected:   "plugin:plugin123",
		},
		{
			name:       "name with special chars",
			pluginName: "my.plugin-v2_test",
			expected:   "plugin:my.plugin-v2_test",
		},
		{
			name:       "name with spaces",
			pluginName: "my plugin",
			expected:   "plugin:my plugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakePluginViewID(tt.pluginName)
			if got != tt.expected {
				t.Errorf("MakePluginViewID(%q) = %q, want %q", tt.pluginName, got, tt.expected)
			}
		})
	}
}

func TestViewID_RoundTrip(t *testing.T) {
	// Test that MakePluginViewID and GetPluginName are inverses
	testNames := []string{
		"burndown",
		"my-plugin",
		"my_plugin",
		"plugin123",
		"my.plugin-v2_test",
		"",
	}

	for _, name := range testNames {
		t.Run("roundtrip_"+name, func(t *testing.T) {
			viewID := MakePluginViewID(name)
			gotName := GetPluginName(viewID)
			if gotName != name {
				t.Errorf("Round trip failed: MakePluginViewID(%q) -> GetPluginName() = %q, want %q",
					name, gotName, name)
			}

			// Verify it's identified as a plugin view
			if !IsPluginViewID(viewID) {
				t.Errorf("MakePluginViewID(%q) = %q not identified as plugin view",
					name, viewID)
			}
		})
	}
}

func TestViewID_BuiltInViews(t *testing.T) {
	// Verify built-in views are not plugin views
	builtInViews := []struct {
		name   string
		viewID ViewID
	}{
		{"task_detail", TaskDetailViewID},
		{"task_edit", TaskEditViewID},
	}

	for _, v := range builtInViews {
		t.Run(v.name, func(t *testing.T) {
			if IsPluginViewID(v.viewID) {
				t.Errorf("Built-in view %q identified as plugin view", v.viewID)
			}
		})
	}

	// Verify plugin prefix constant is identified as plugin view
	if !IsPluginViewID(PluginViewIDPrefix) {
		t.Error("PluginViewIDPrefix not identified as plugin view")
	}
}

func TestViewID_EdgeCases(t *testing.T) {
	t.Run("double plugin prefix", func(t *testing.T) {
		// What if someone accidentally passes "plugin:foo" to MakePluginViewID?
		viewID := MakePluginViewID("plugin:foo")
		if viewID != "plugin:plugin:foo" {
			t.Errorf("MakePluginViewID(\"plugin:foo\") = %q, want %q",
				viewID, "plugin:plugin:foo")
		}

		// It should still be identified as a plugin view
		if !IsPluginViewID(viewID) {
			t.Error("Double-prefixed ID not identified as plugin view")
		}

		// GetPluginName should strip only the first prefix
		name := GetPluginName(viewID)
		if name != "plugin:foo" {
			t.Errorf("GetPluginName(%q) = %q, want %q", viewID, name, "plugin:foo")
		}
	})

	t.Run("case sensitivity", func(t *testing.T) {
		// Plugin prefix is lowercase, verify case matters
		upperID := ViewID("PLUGIN:foo")
		if IsPluginViewID(upperID) {
			t.Error("Uppercase PLUGIN: incorrectly identified as plugin view")
		}

		mixedID := ViewID("Plugin:foo")
		if IsPluginViewID(mixedID) {
			t.Error("Mixed-case Plugin: incorrectly identified as plugin view")
		}
	})
}
