package view

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
)

func TestBuildCompactTaskContent(t *testing.T) {
	colors := config.GetColors()

	tests := []struct {
		name           string
		task           *taskpkg.Task
		availableWidth int
		contains       []string
		notContains    []string
	}{
		{
			name:           "contains task ID",
			task:           &taskpkg.Task{ID: "TIKI-ABC123", Type: taskpkg.TypeStory, Priority: 3},
			availableWidth: 40,
			contains:       []string{"TIKI-ABC123"},
		},
		{
			name:           "contains title",
			task:           &taskpkg.Task{ID: "TIKI-TTL001", Title: "My Task", Type: taskpkg.TypeStory, Priority: 3},
			availableWidth: 40,
			contains:       []string{"My Task"},
		},
		{
			name:           "title truncated at width",
			task:           &taskpkg.Task{ID: "TIKI-TRC001", Title: "ABCDEFGHIJ", Type: taskpkg.TypeStory, Priority: 3},
			availableWidth: 7,
			contains:       []string{"ABCD"},
			notContains:    []string{"ABCDEFGHIJ"},
		},
		{
			name:           "emoji for story type",
			task:           &taskpkg.Task{ID: "TIKI-EMO001", Type: taskpkg.TypeStory, Priority: 3},
			availableWidth: 40,
			contains:       []string{taskpkg.TypeEmoji(taskpkg.TypeStory)},
		},
		{
			name:           "emoji for bug type",
			task:           &taskpkg.Task{ID: "TIKI-EMO002", Type: taskpkg.TypeBug, Priority: 3},
			availableWidth: 40,
			contains:       []string{taskpkg.TypeEmoji(taskpkg.TypeBug)},
		},
		{
			name:           "priority label",
			task:           &taskpkg.Task{ID: "TIKI-PRI001", Type: taskpkg.TypeStory, Priority: 1},
			availableWidth: 40,
			contains:       []string{taskpkg.PriorityLabel(1)},
		},
		{
			name:           "zero points does not panic",
			task:           &taskpkg.Task{ID: "TIKI-PT0001", Type: taskpkg.TypeStory, Priority: 3, Points: 0},
			availableWidth: 40,
		},
		{
			name:           "empty title does not panic",
			task:           &taskpkg.Task{ID: "TIKI-NT0001", Type: taskpkg.TypeStory, Priority: 3, Title: ""},
			availableWidth: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCompactTaskContent(tt.task, colors, tt.availableWidth)

			if result == "" {
				t.Error("expected non-empty output")
			}
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected output to contain %q\noutput: %q", want, result)
				}
			}
			for _, unwanted := range tt.notContains {
				if strings.Contains(result, unwanted) {
					t.Errorf("expected output NOT to contain %q\noutput: %q", unwanted, result)
				}
			}
		})
	}
}

func TestBuildExpandedTaskContent(t *testing.T) {
	colors := config.GetColors()

	tests := []struct {
		name           string
		task           *taskpkg.Task
		availableWidth int
		contains       []string
		notContains    []string
	}{
		{
			name:           "empty description no panic",
			task:           &taskpkg.Task{ID: "TIKI-EXP001", Type: taskpkg.TypeStory, Priority: 3, Description: ""},
			availableWidth: 40,
		},
		{
			name:           "single desc line included",
			task:           &taskpkg.Task{ID: "TIKI-EXP002", Type: taskpkg.TypeStory, Priority: 3, Description: "Line1"},
			availableWidth: 40,
			contains:       []string{"Line1"},
		},
		{
			name:           "three desc lines all included",
			task:           &taskpkg.Task{ID: "TIKI-EXP003", Type: taskpkg.TypeStory, Priority: 3, Description: "L1\nL2\nL3"},
			availableWidth: 40,
			contains:       []string{"L1", "L2", "L3"},
		},
		{
			name:           "fourth desc line not included",
			task:           &taskpkg.Task{ID: "TIKI-EXP004", Type: taskpkg.TypeStory, Priority: 3, Description: "L1\nL2\nL3\nL4"},
			availableWidth: 40,
			contains:       []string{"L1", "L2", "L3"},
			notContains:    []string{"L4"},
		},
		{
			name:           "empty tags omits tags label",
			task:           &taskpkg.Task{ID: "TIKI-EXP005", Type: taskpkg.TypeStory, Priority: 3, Tags: []string{}},
			availableWidth: 40,
			notContains:    []string{"Tags:"},
		},
		{
			name:           "non-empty tags included",
			task:           &taskpkg.Task{ID: "TIKI-EXP006", Type: taskpkg.TypeStory, Priority: 3, Tags: []string{"ui", "backend"}},
			availableWidth: 40,
			contains:       []string{"ui", "backend"},
		},
		{
			name:           "tag truncated at small width no panic",
			task:           &taskpkg.Task{ID: "TIKI-EXP007", Type: taskpkg.TypeStory, Priority: 3, Tags: []string{"abcdefghij"}},
			availableWidth: 8,
		},
		{
			name:           "desc line truncated at small width",
			task:           &taskpkg.Task{ID: "TIKI-EXP008", Type: taskpkg.TypeStory, Priority: 3, Description: "ABCDEFGHIJ"},
			availableWidth: 7,
			contains:       []string{"ABCD"},
			notContains:    []string{"ABCDEFGHIJ"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildExpandedTaskContent(tt.task, colors, tt.availableWidth)

			if result == "" {
				t.Error("expected non-empty output")
			}
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected output to contain %q\noutput: %q", want, result)
				}
			}
			for _, unwanted := range tt.notContains {
				if strings.Contains(result, unwanted) {
					t.Errorf("expected output NOT to contain %q\noutput: %q", unwanted, result)
				}
			}
		})
	}
}
