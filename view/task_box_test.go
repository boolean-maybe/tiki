package view

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// makeTiki creates a tiki with the given ID, type string, and priority for tests.
func makeTiki(id string, taskType string, priority int) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = id
	tk.Set(tikipkg.FieldType, taskType)
	if priority > 0 {
		tk.Set(tikipkg.FieldPriority, priority)
	}
	return tk
}

func TestBuildCompactTaskContent(t *testing.T) {
	colors := config.GetColors()

	tests := []struct {
		name           string
		task           *tikipkg.Tiki
		availableWidth int
		contains       []string
		notContains    []string
	}{
		{
			name:           "contains task ID",
			task:           makeTiki("ABC123", string(taskpkg.TypeStory), 3),
			availableWidth: 40,
			contains:       []string{"ABC123"},
		},
		{
			name: "contains title",
			task: func() *tikipkg.Tiki {
				tk := makeTiki("TTL001", string(taskpkg.TypeStory), 3)
				tk.Title = "My Task"
				return tk
			}(),
			availableWidth: 40,
			contains:       []string{"My Task"},
		},
		{
			name: "title truncated at width",
			task: func() *tikipkg.Tiki {
				tk := makeTiki("TRC001", string(taskpkg.TypeStory), 3)
				tk.Title = "ABCDEFGHIJ"
				return tk
			}(),
			availableWidth: 7,
			contains:       []string{"ABCD"},
			notContains:    []string{"ABCDEFGHIJ"},
		},
		{
			name:           "emoji for story type",
			task:           makeTiki("EMO001", string(taskpkg.TypeStory), 3),
			availableWidth: 40,
			contains:       []string{taskpkg.TypeEmoji(taskpkg.TypeStory)},
		},
		{
			name:           "emoji for bug type",
			task:           makeTiki("EMO002", string(taskpkg.TypeBug), 3),
			availableWidth: 40,
			contains:       []string{taskpkg.TypeEmoji(taskpkg.TypeBug)},
		},
		{
			name:           "priority label",
			task:           makeTiki("PRI001", string(taskpkg.TypeStory), 1),
			availableWidth: 40,
			contains:       []string{taskpkg.PriorityLabel(1)},
		},
		{
			name:           "zero points does not panic",
			task:           makeTiki("PT0001", string(taskpkg.TypeStory), 3),
			availableWidth: 40,
		},
		{
			name:           "empty title does not panic",
			task:           makeTiki("NT0001", string(taskpkg.TypeStory), 3),
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
		task           *tikipkg.Tiki
		availableWidth int
		contains       []string
		notContains    []string
	}{
		{
			name:           "empty description no panic",
			task:           makeTiki("EXP001", string(taskpkg.TypeStory), 3),
			availableWidth: 40,
		},
		{
			name: "single desc line included",
			task: func() *tikipkg.Tiki {
				tk := makeTiki("EXP002", string(taskpkg.TypeStory), 3)
				tk.Body = "Line1"
				return tk
			}(),
			availableWidth: 40,
			contains:       []string{"Line1"},
		},
		{
			name: "three desc lines all included",
			task: func() *tikipkg.Tiki {
				tk := makeTiki("EXP003", string(taskpkg.TypeStory), 3)
				tk.Body = "L1\nL2\nL3"
				return tk
			}(),
			availableWidth: 40,
			contains:       []string{"L1", "L2", "L3"},
		},
		{
			name: "fourth desc line not included",
			task: func() *tikipkg.Tiki {
				tk := makeTiki("EXP004", string(taskpkg.TypeStory), 3)
				tk.Body = "L1\nL2\nL3\nL4"
				return tk
			}(),
			availableWidth: 40,
			contains:       []string{"L1", "L2", "L3"},
			notContains:    []string{"L4"},
		},
		{
			name: "empty tags omits tags label",
			task: func() *tikipkg.Tiki {
				tk := makeTiki("EXP005", string(taskpkg.TypeStory), 3)
				tk.Set(tikipkg.FieldTags, []string{})
				return tk
			}(),
			availableWidth: 40,
			notContains:    []string{"Tags:"},
		},
		{
			name: "non-empty tags included",
			task: func() *tikipkg.Tiki {
				tk := makeTiki("EXP006", string(taskpkg.TypeStory), 3)
				tk.Set(tikipkg.FieldTags, []string{"ui", "backend"})
				return tk
			}(),
			availableWidth: 40,
			contains:       []string{"ui", "backend"},
		},
		{
			name: "tag truncated at small width no panic",
			task: func() *tikipkg.Tiki {
				tk := makeTiki("EXP007", string(taskpkg.TypeStory), 3)
				tk.Set(tikipkg.FieldTags, []string{"abcdefghij"})
				return tk
			}(),
			availableWidth: 8,
		},
		{
			name: "desc line truncated at small width",
			task: func() *tikipkg.Tiki {
				tk := makeTiki("EXP008", string(taskpkg.TypeStory), 3)
				tk.Body = "ABCDEFGHIJ"
				return tk
			}(),
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
