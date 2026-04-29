package document

import "testing"

func TestIsWorkflowFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		fm   map[string]interface{}
		want bool
	}{
		{"nil is plain", nil, false},
		{"empty is plain", map[string]interface{}{}, false},
		{"id+title only is plain", map[string]interface{}{"id": "X", "title": "hi"}, false},
		{"status present is workflow", map[string]interface{}{"id": "X", "status": "ready"}, true},
		{"type present is workflow", map[string]interface{}{"type": "story"}, true},
		{"priority present is workflow", map[string]interface{}{"priority": 3}, true},
		{"points present is workflow", map[string]interface{}{"points": 5}, true},
		{"tags present is workflow", map[string]interface{}{"tags": []string{"x"}}, true},
		{"dependsOn present is workflow", map[string]interface{}{"dependsOn": []string{"A"}}, true},
		{"due present is workflow", map[string]interface{}{"due": "2026-01-01"}, true},
		{"assignee present is workflow", map[string]interface{}{"assignee": "alice"}, true},
		{"recurrence present is workflow", map[string]interface{}{"recurrence": "weekly"}, true},
		{"unknown custom keys don't count", map[string]interface{}{"id": "X", "title": "t", "notes": "foo"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsWorkflowFrontmatter(tt.fm); got != tt.want {
				t.Errorf("IsWorkflowFrontmatter() = %v, want %v", got, tt.want)
			}
		})
	}
}
