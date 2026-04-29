package tikistore

import (
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

func TestLegacyUpgrader_UpgradeTask(t *testing.T) {
	upgrader := &LegacyUpgrader{}

	tests := []struct {
		name       string
		status     taskpkg.Status
		wantStatus taskpkg.Status
	}{
		{"snake_case in_progress → inProgress", "in_progress", "inProgress"},
		{"already camelCase inProgress", "inProgress", "inProgress"},
		{"single word done", "done", "done"},
		{"single word backlog", "backlog", "backlog"},
		{"single word ready", "ready", "ready"},
		{"single word review", "review", "review"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &taskpkg.Task{
				ID:     "TEST01",
				Title:  "test task",
				Status: tt.status,
			}

			result := upgrader.UpgradeTask(task)

			if result.Status != tt.wantStatus {
				t.Errorf("UpgradeTask() status = %q, want %q", result.Status, tt.wantStatus)
			}
		})
	}
}
